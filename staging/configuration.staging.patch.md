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
        // Authenticated SMTP submission via Google Workspace, STARTTLS on 587.
        // URL-encode the @ in the username as %40.
        // App password is 16 chars with no spaces (Google displays spaces — strip them).
        'dsn' => 'smtp://YOUR_WORKSPACE_USER%40YOUR_DOMAIN.com:APP_PASSWORD_NO_SPACES@smtp.gmail.com:587',
        'from_email' => 'YOUR_WORKSPACE_USER@YOUR_DOMAIN.com',
        'from_name'  => 'FileGator Staging',
        'timeout'    => 5,
    ],
],
```

**Why these exact values:**
- `smtp.gmail.com:587` with implicit STARTTLS is the documented Workspace path for app-password auth.
- App passwords bypass 2FA and don't require any tenant-side toggles.
- `from_email` must match the authenticated user (Gmail rewrites the `From:` header otherwise) — that's why you picked "send as me."
- `timeout: 5` is the existing safeguard against a stuck PHP-FPM worker if Gmail is slow; leave it.

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

## 4. Session cookie — line ~60

If you fronted with Caddy (TLS), keep `cookie_secure` true (matches prod). If you skipped TLS and went plain HTTP on `:8080`, flip it to false for staging or testers can't log in.

---

## 5. Lines that should NOT change from prod

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
