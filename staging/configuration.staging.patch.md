# Staging `configuration.php` overrides

Base: **copy your production `configuration.php` to the VM verbatim** (this preserves user/permission setup, auth adapter choice, repository adapter, all of it). Then apply the deltas below — these are the only lines that should differ from prod.

> Line numbers reference [configuration_sample.php](../configuration_sample.php) so they map cleanly even if prod's file has minor drift.

---

## 1. CSRF key — line ~82

Regenerate. Never share with prod.

```php
'csrf_key' => "<paste 32+ random chars here>",
```

Generate one on the VM with:

```
openssl rand -hex 32
```

---

## 2. Mailer block — lines ~122–139

Replace the prod mailer config with the Gmail Workspace DSN. Use your own Workspace mailbox both as the SMTP user and in `from_email` (you chose "send as me").

```php
'Filegator\Services\Mailer\MailerInterface' => [
    'handler' => '\Filegator\Services\Mailer\Adapters\SymfonyMailer',
    'config' => [
        // Postmark via HTTPS API (port 443). Requires symfony/postmark-mailer
        // composer package (bundled in this repo). Send is one HTTP POST —
        // no SMTP handshake, no port-block risk, structured error responses.
        'dsn' => 'postmark+api://POSTMARK_SERVER_API_TOKEN@default',
        'from_email' => 'staff@elliffcpa.com',
        'from_name'  => 'FileGator Staging',
        'timeout'    => 5,
    ],
],
```

**Why these exact values:**
- `postmark+api://` uses HTTPS (port 443) — works on any cloud provider regardless of outbound SMTP policy.
- `from_email` must match a verified Sender Signature or verified Domain in your Postmark account.
- `@default` is literal — Symfony's syntax for "use the provider's default endpoint" (api.postmarkapp.com).
- Replace `POSTMARK_SERVER_API_TOKEN` with the Server API Token from Postmark → Servers → API Tokens.
- `timeout: 5` clamps PHP's default socket timeout so a Postmark hiccup never stalls a PHP-FPM worker for 60s.

**Alternative if you'd rather use SMTP (port 2525):**
```
'dsn' => 'smtp://TOKEN:TOKEN@smtp.postmarkapp.com:2525',
```
Same token in both spots. Works without the postmark-mailer composer package, but loses structured error reporting and is one rebuild behind the API path if you ever change transport.

---

## 3. Password reset block — lines ~146–168

Two changes: subject prefix and `reset_url_base`. Branding stays whatever prod has so testers see the real theme.

```php
'Filegator\Services\PasswordReset\PasswordResetService' => [
    'handler' => '\Filegator\Services\PasswordReset\PasswordResetService',
    'config' => [
        'token_file' => __DIR__.'/private/password_resets.json',
        'reset_subject' => '[STAGING] Reset your FileGator password',
        // sslip.io form, e.g. https://203-0-113-45.sslip.io/  — note trailing slash.
        'reset_url_base' => 'https://<DASHED-IP>.sslip.io/',
        'branding' => [
            // copy from prod verbatim
        ],
    ],
],
```

---

## 4. Audit alerts block — new lines, drop in near the other service entries

Operational alert emails for admin user mutations (create / update / delete / MFA reset) and user-self-disabled MFA. Drop the block in just before the `Router` entry — it must register before `Router` is initialised so controllers can resolve `AuditMailer` via type-hint:

```php
'Filegator\Services\Audit\AuditMailer' => [
    'handler' => '\Filegator\Services\Audit\AuditMailer',
    'config' => [
        // Inbox that monitors audit alerts. Receives one email per admin
        // user-mutation plus one per user-self-disabled MFA event.
        'recipient'  => 'staff@elliffcpa.com',
        // Visible From: on audit emails. Deliberately distinct from the
        // transactional no-reply sender configured in the Mailer block
        // above, so audit alerts are recognisable in the recipient inbox.
        'from_email' => 'staff@elliffcpa.com',
        'from_name'  => 'Elliff CPA Audit',
        // Shown in the body header line, e.g.
        // "A user was updated in the Elliff CPA Portal."
        'app_label'  => 'Elliff CPA Portal',
        'enabled'    => true,
    ],
],
```

**What fires what:**

| Trigger | Subject line |
| --- | --- |
| Admin creates a user | `New user created: <username>` |
| Admin deletes a user | `User deleted: <username>` |
| Admin updates a user (role/folder/permissions/password/email/username) | Subject leads with the highest-priority change; body lists every diff |
| Admin updates only the user's display name | (silent — cosmetic) |
| Admin saves the edit dialog without changing anything | (silent — true no-op) |
| Admin resets another user's MFA | `MFA reset by admin for <username>` |
| Any user disables their own MFA | `MFA disabled by user: <username>` |

**Failure mode:** if `recipient` is blank or `enabled => false`, the service short-circuits silently and writes a single "Audit alert skipped: AuditMailer not configured" line to `private/logs/app.log`. Admin actions themselves succeed either way — the audit email is fire-and-forget around the existing flow.

**Sender deliverability:** the `from_email` must be a verified Postmark Sender Signature (or covered by a verified Domain) on the same Postmark server the transactional Mailer block uses. If `staff@elliffcpa.com` isn't already verified there, add it under Postmark → Sender Signatures before going live or audit emails will bounce.

---

## 5. Session cookie — line ~60

If you fronted with Caddy (TLS), keep `cookie_secure` true (matches prod). If you skipped TLS and went plain HTTP on `:8080`, flip it to false for staging or testers can't log in.

---

## 6. Lines that should NOT change from prod

- `mfa_required_for_admins` (line 12) — match prod exactly so UAT exercises the forced-enroll path
- `password_reset_token_ttl`, `_max_per_*` rate limits — match prod
- Auth adapter, repository adapter — match prod
- `public_path`, `public_dir` — leave whatever prod uses

---

## Verifying the DSN before going live

On the VM, after writing `configuration.php`:

```
docker compose -f staging/docker-compose.staging.yml up -d
docker compose -f staging/docker-compose.staging.yml exec filegator \
  php -r "require '/var/www/filegator/dist/index.php';" 2>&1 | head -20
```

Then trigger a forgot-password from the UI and watch:

```
docker compose -f staging/docker-compose.staging.yml exec filegator \
  tail -f private/logs/app.log
```

A successful send writes nothing dramatic; a failure prints a Symfony Mailer transport exception with the SMTP server's actual reply (auth failure / blocked sender / etc.) — usually self-explanatory.
