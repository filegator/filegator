---
title: "Single sign-on from other apps"
weight: 45
---

Other applications — a CMS, an admin panel, a customer portal — can sign a
visitor into FileGator without a password. The application builds a short,
signed URL; the browser follows it; FileGator opens a normal session.

No password crosses the boundary and no API keys are handed out. Each
application has its own secret, so one can be revoked without touching the
others.

## How it works

```
panel                       browser                    FileGator
  |  build signed URL  ->      |                            |
  |                            |  GET /auth/sso?...    ->   |  verify signature
  |                            |                            |  spend the nonce
  |                            |                            |  create session cookie
  |                            |  <- 303 to /               |
  |                            |  normal browsing       ->  |
```

The signed URL is a **ticket**, not a session:

| | Lifetime |
|---|---|
| Ticket (`/auth/sso?...`) | 60 seconds, **single use** |
| Session it creates | `security.session_ttl`, **8 hours** by default |

Once exchanged, the ticket is spent. Replaying it returns `403` and issues
nothing, so a URL leaked through browser history or a proxy log is worthless.

## Configure an application

```json
{
  "sso": {
    "applications": [
      {
        "name": "panel",
        "secret": "0123456789abcdef0123456789abcdef",
        "mode": "create",
        "home": "/clients/%s",
        "permissions": ["read", "download", "upload"],
        "admin_usernames": [],
        "expiry": 60
      }
    ]
  }
}
```

Generate a secret with `openssl rand -hex 32`. It must be at least 32
characters.

| Key | Meaning |
|-----|---------|
| `name` | Identifier the handoff must carry (`app=panel`). Unique, and required |
| `secret` | Shared with the application. At least 32 characters, required. **Never returned by any API** |
| `mode` | `existing`, `policy` or `create` — see below. Required |
| `home` | Home template; `%s` becomes the username. **Required** for `policy` and `create`; ignored by `existing`, which uses the account's own home |
| `permissions` | Granted in `policy`, and to accounts made by `create`. Ignored by `existing` |
| `admin_usernames` | Administrators **inside this application's sessions only**. Empty is normal |
| `expiry` | Ticket lifetime in seconds, `0`–`3600`. **`0` means the default of 60**, not "no expiry" |

A configuration that fails any of these checks stops startup with the
application's name and the reason, rather than failing at the first handoff.

## The three modes

| `mode` | Username FileGator already knows | Username it does not know |
|--------|----------------------------------|---------------------------|
| `existing` | signs in as that user, with **their own** permissions and home | `403` |
| `policy` | signs in with **the application's** permissions and home template | same — the account need to exist |
| `create` | signs in with **their own** permissions | account created from the policy, then signs in |

**`existing`** is the safest: the remote application can only open sessions for
people you created, and can never widen what they can do. It cannot grant
administrator unless you list the name in `admin_usernames`.

**`policy`** is for handing someone into *a folder* rather than an account. For a
customer portal, `"home": "/clients/%s"` puts `alice` in `/clients/alice` with
exactly the listed permissions. Nothing is written to the users file, so there
is nothing to clean up — and the visitor cannot reach another customer's folder
even if the portal is misconfigured.

**`create`** creates a real account on first sign-in, which you can then edit in
**Admin → Users**. Use it when the remote app owns the user list and you want
them visible here. The account has a password nobody knows, so it can only be
entered through this application, never from the login form.

## Building the URL

The signature is HMAC-SHA256 over `user|app|exp|nonce`, hex encoded.

```
GET /auth/sso?user=alice&app=panel&exp=1792933275&nonce=<32 hex chars>&sig=<hex>
sig = HMAC-SHA256(secret, "alice|panel|1792933275|cf9683ba…")
```

The fields are joined with `|` in that order; nothing is URL-encoded before
signing.

### PHP helper

Drop this in your application and call it at the moment of redirect.

```php
<?php
/**
 * Builds a signed FileGator single sign-on URL.
 *
 * The URL is single-use and expires in 60 seconds, so build it when you
 * redirect and never cache or log it.
 *
 * @param string $baseUrl  e.g. "https://files.example.com" or ".../browser"
 * @param string $app      application name configured in FileGator
 * @param string $secret   that application's secret
 * @param string $username the user to sign in as
 * @param int    $ttl      seconds the link stays valid
 * @return string
 */
function filegator_sso_url($baseUrl, $app, $secret, $username, $ttl = 60)
{
    $expiry = time() + $ttl;
    $nonce  = bin2hex(random_bytes(16));

    // Must match internal/sso.Signature: HMAC-SHA256 over
    // "user|app|exp|nonce", hex encoded.
    $payload   = $username . '|' . $app . '|' . $expiry . '|' . $nonce;
    $signature = hash_hmac('sha256', $payload, $secret);

    $query = http_build_query([
        'user'  => $username,
        'app'   => $app,
        'exp'   => $expiry,
        'nonce' => $nonce,
        'sig'   => $signature,
    ]);

    return rtrim($baseUrl, '/') . '/auth/sso?' . $query;
}
```

Use it:

```php
require 'filegator-sso.php';

header('Location: ' . filegator_sso_url(
    'https://files.example.com',    // or '.../browser' when mounted under a path
    'panel',
    getenv('FILEGATOR_SSO_SECRET'),
    $user->login
));
exit;
```

The signature this produces is pinned by a test, so it cannot drift from the
Go implementation.

Any language works — it is one HMAC:

```python
import hmac, hashlib, time, secrets

exp, nonce = int(time.time()) + 60, secrets.token_hex(16)
payload = f"alice|panel|{exp}|{nonce}"
sig = hmac.new(secret.encode(), payload.encode(), hashlib.sha256).hexdigest()
url = f"https://files.example.com/auth/sso?user=alice&app=panel&exp={exp}&nonce={nonce}&sig={sig}"
```

## Rules the implementation enforces

- **Single use.** A nonce is remembered until the ticket expires; a second use
  is refused.
- **Short-lived.** Past `exp` the ticket is refused. Extending `exp` breaks the
  signature, so a link cannot be made to last longer.
- **Constant-time comparison**, so a signature cannot be guessed byte by byte.
- **Indistinguishable failures.** An unknown application, a bad signature and an
  expired ticket all return the same `403`; the reason is only in the event log.
- **The username becomes a folder name safely.** A name containing `/` or `..`
  is hex-encoded before it fills `%s`, so it cannot climb out of the template.

## Administrator access

`admin_usernames` grants administrator only within sessions created by *that*
application. A portal listing `jane` cannot make anyone an administrator in
FileGator itself, and removing the name takes effect the next time she signs in
through that portal.

Leave the list empty unless you mean it: an application that can grant
administrator can also change storage settings.

## What a session created this way can do

The same as any session: the permissions in the policy, confined to the
template home. It cannot reach the admin area unless it is an administrator.
`policy` and `create` sessions are ordinary sessions afterwards — FileGator has
no idea how they started, and logging out works normally.

**The session does not follow the remote application's session.** Logging out
of the panel does not log the user out of FileGator; that happens when the
session expires or the user logs out here. Lower `security.session_ttl` if you
need tighter coupling.

## Troubleshooting

Every refusal is logged as `auth.sso_failed` with the reason:

```
{"event":"auth.sso_failed","msg":"single sign-on refused","app":"panel","reason":"expired"}
```

| Reason | Cause |
|--------|-------|
| `expired` | The link was built more than `expiry` seconds ago — clock skew between servers is the usual culprit |
| `replayed` | The link was already used. Build a new one per redirect |
| `unknown user` | `mode: existing` and the account does not exist |
| `invalid` | Wrong secret, wrong `app` name, or a field edited after signing |

A successful handoff logs `auth.sso_login`, naming the application and whether
an account was created.
