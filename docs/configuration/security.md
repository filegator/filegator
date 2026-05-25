---
currentMenu: security
---

## Configuring Security service

Simple security service is included in the script by default. This service provides:

- Basic session-based [CSRF](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection
- IP allow list
- IP deny list

```
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                'csrf_protection' => true,
                'csrf_key' => "123456", // randomize this
                'ip_allowlist' => [],
                'ip_denylist' => [
                    '172.16.1.2',
                    '172.16.3.4',
                ],
            ],
        ],
```

If you set `ip_allowlist` then only users coming from listed IP addresses will be able to use the script.

## MFA encryption key

TOTP secrets are encrypted at rest with libsodium secretbox. The key is auto-generated at `private/mfa_encryption.key` (mode `0600`) on first use.

**Back up the keyfile alongside `users.json`.** Losing one without the other makes every enrolled user's TOTP secret unrecoverable. Backup codes are hashed independently and unaffected, so users with backup codes can still log in and re-enroll.

### Keyfile-loss recovery

If the sole admin loses the keyfile AND their backup codes AND their device:

1. Stop the app.
2. Edit `private/users.json` and on the admin row set `"mfa_enabled": false` and `"mfa_secret": null`.
3. Restart the app.
4. Log in with password (no MFA).
5. Re-enroll MFA — a fresh secret is generated under whichever key exists.

This is the documented escape hatch for the single-admin worst case. Routine MFA resets should use the admin panel's reset_mfa endpoint instead.

## MFA pending-state binding

When a user passes step 1 (password) of MFA login, the pending state stores a binding hash that step 2 (`/login/mfa`) must match. Default binding is User-Agent only (stable across NAT and mobile carriers, still defeats the cookie-theft scenario).

```php
'mfa_pending_bind_ua' => true,             // default
'mfa_pending_bind_ip_prefix' => null,      // 'exact', '/24', '/48', or null
```

Set `mfa_pending_bind_ip_prefix` to `'/24'` (IPv4) or `'/48'` (IPv6) to also require the request IP to be in the same prefix between steps. `'exact'` requires identical IPs (high-security; will reject legitimate users on NAT'd networks). `null` disables IP binding.

## MFA brute-force lockout

Failed `/login/mfa` attempts increment both per-IP and per-username counters. Either counter at `lockout_attempts` triggers a 429 for `lockout_timeout` seconds. The per-username counter is the rotating-IP defence — an attacker proxying through a botnet still trips the lock because it travels with the account.

**Tradeoff:** anyone who knows a username can lock that user out by failing TOTP `lockout_attempts` times. The outage is bounded by `lockout_timeout` (default 15s). If you see user reports of "I can't log in" with no other context, check `private/tmp/mfa_user_*.lock` and the recent login log for a brute-force attempt against that account.

## Admin step-up auth

When the acting admin has MFA enrolled, the following endpoints require a fresh `stepup_password` + `stepup_code` in the request body (in addition to the session cookie):

- `POST /storeuser`
- `POST /updateuser/{username}`
- `POST /deleteuser/{username}`
- `POST /admin/users/{username}/reset_mfa`

`stepup_password` is the acting admin's password; `stepup_code` is a current TOTP or a backup code (set `stepup_use_backup: true` for the latter). The 90-second TOTP replay marker applies — five admin writes in one minute need five distinct codes. See `docs/follow-ups.md` for the planned step-up-token follow-up that amortises one TOTP across a short admin-write window.

When the acting admin has no MFA enrolled, step-up is a no-op (no behaviour change for deploys that haven't enabled `mfa_required_for_admins`).
