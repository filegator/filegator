# Upgrade guide

These notes cover upgrades that cross the **MFA + self-service password reset** release. If your existing version predates that, follow this whole document. If you're upgrading between versions that already include MFA, you can skip Step 2 — only the standard "replace files" flow applies.

## Standard upgrade flow

This release continues to be compatible with the upstream recipe:

1. **Back up everything**, including `private/` and `repository/`.
2. **Download the latest release** and unpack it into a staging directory.
3. **Replace all files and folders except `repository/`, `private/`, and `configuration.php`** in your live install with the staging copies.

Your `repository/` (user-uploaded files), `private/users.json`, `private/sessions/`, `private/logs/`, and `private/tmp/` are all preserved unmodified.

`configuration.php` is preserved because it's not in the distribution archive — only `configuration_sample.php` ships. Step 2 below covers what you need to add to your existing `configuration.php`.

## Step 2: Add the new service blocks to `configuration.php`

Open the updated `configuration_sample.php` and copy three new entries into the `services` array of your live `configuration.php`. **Order matters** — these three blocks must appear *before* the existing `Filegator\Services\Router\Router` block. The bootstrap iterates services in declaration order and dispatches the route inside `Router::init`, so the mailer / MFA / password-reset bindings have to exist before any controller is invoked.

Paste this above the existing `Router` entry, replacing the `dsn` and `reset_url_base` values for your environment:

```php
'Filegator\Services\Mailer\MailerInterface' => [
    'handler' => '\Filegator\Services\Mailer\Adapters\SymfonyMailer',
    'config' => [
        // 'null://null' disables sending and hides the Forgot-Password UI link.
        // Examples:
        //   'smtp://user:pass@smtp.example.com:587?encryption=tls'
        //   'sendmail://default'
        'dsn' => 'null://null',
        'from_email' => 'no-reply@example.com',
        'from_name' => 'FileGator',
        // Hard cap (seconds) we force on every SMTP socket so a slow / unreachable
        // mail server cannot hang a PHP-FPM worker for PHP's 60-second
        // default_socket_timeout. Appended to the DSN automatically if not already
        // present, and also enforced via a per-request default_socket_timeout
        // clamp. Tune up for slow servers; do not set to 0.
        'timeout' => 5,
    ],
],
'Filegator\Services\Mfa\MfaService' => [
    'handler' => '\Filegator\Services\Mfa\MfaService',
    'config' => [
        'issuer' => 'FileGator',
    ],
],
'Filegator\Services\PasswordReset\PasswordResetService' => [
    'handler' => '\Filegator\Services\PasswordReset\PasswordResetService',
    'config' => [
        'token_file' => __DIR__.'/private/password_resets.json',
        'reset_subject' => 'Reset your FileGator password',
        // REQUIRED for password reset to work. The full public URL operators
        // want reset links to point to (scheme + host + base path). We
        // deliberately do NOT derive this from the request Host header; doing
        // so would allow an attacker to send victims reset links pointing at
        // an attacker-controlled host.
        // Leave null to keep the feature disabled.
        'reset_url_base' => null, // e.g. 'https://files.example.com/'
    ],
],
```

## Step 3: Add the new top-level keys

In the same `configuration.php`, add these top-level keys (alongside `lockout_attempts`, `timezone`, etc.):

```php
'mfa_required_for_admins' => true,           // admins must enroll TOTP on first login
'mfa_pending_bind_ua' => true,               // reject /login/mfa if User-Agent differs from /login
'mfa_pending_bind_ip_prefix' => null,        // 'exact', '/24', '/48', or null to disable IP binding
'password_reset_token_ttl' => 3600,          // seconds the reset link stays valid
'password_reset_max_per_hour_per_ip' => 3,
'password_reset_max_per_day_per_email' => 3,
```

You also need two new service blocks alongside the existing `Filegator\Services\Mfa\MfaService` block (still before the Router entry):

```php
'Filegator\Services\Auth\MfaLockout' => [
    'handler' => '\Filegator\Services\Auth\MfaLockout',
    'config' => [],
],
'Filegator\Services\Mfa\MfaSecretCrypto' => [
    'handler' => '\Filegator\Services\Mfa\MfaSecretCrypto',
    'config' => [
        // 32-byte sodium secretbox key. Auto-generated on first use,
        // mode 0600. Back up alongside users.json — losing one without
        // the other makes every enrolled TOTP secret unrecoverable.
        'key_path' => __DIR__.'/private/mfa_encryption.key',
    ],
],
```

The keyfile is created on first MFA enrollment or first TOTP verify against a previously-enrolled user. Existing plaintext secrets are lazy-migrated to encrypted form on the next successful TOTP verify — no manual migration step required.

## Step 4: Add the CSRF exemption list (recommended)

The password-reset flow exposes three public endpoints that need to be CSRF-exempt (clients arriving from an email link have no prior session token to send). Add this key to your existing `Filegator\Services\Security\Security` config block:

```php
'csrf_exempt_paths' => ['/password/forgot', '/password/reset/validate', '/password/reset'],
```

If you omit it, the code falls back to the same default list, so this step is optional — but explicit is better.

## Step 5: Install new PHP dependencies (Composer installs only)

If you installed FileGator via `composer install` rather than from a prebuilt zip:

```bash
composer install
```

The lock file ships with two new direct dependencies (`spomky-labs/otphp ^11.2` and `symfony/mailer ^5.4`) plus their transitives.

## Step 6: What happens on the first request after upgrade

Two one-time side effects you should expect and communicate to users:

1. **All currently-logged-in users are signed out.** The session-hash format changed (we now include `mfa_enabled` and `email` so out-of-band MFA toggles invalidate live sessions). Users just log in again; nothing else is affected.
2. **Admin users are forced into MFA enrollment on next login** if `mfa_required_for_admins` is `true` (the default). Each admin will be shown a QR code and asked to enter a code from an authenticator app, then shown 10 one-time backup codes to save. If you cannot coordinate this across all admins, set `mfa_required_for_admins => false` for the rollout window and tighten later.

## Step 7: Verify

After the upgrade, hit these in order:

1. Log in as a regular user — the experience is identical to before (no MFA unless they opt in).
2. Log in as an admin — should be forced through the MFA setup screen on first login.
3. (If you configured the mailer) Log out, click "Forgot password?", enter a known email, and verify the reset link arrives and works.

## Rollback

If something goes wrong:

1. Restore your `configuration.php` backup.
2. Revert the file replacement using your pre-upgrade backup.
3. `private/users.json` is unchanged by this release at rest, so users continue to authenticate normally on the old code. The `private/password_resets.json` file may exist (if reset was exercised) but is ignored by the old code.

## Requirements

- **PHP 8.1+** (unchanged from upstream master).
- **`zip` PHP extension** (unchanged; needed by `league/flysystem-ziparchive`).
- If you build the frontend from source instead of using the prebuilt `dist/`, the `qrcode ^1.5` npm package is a new runtime dependency.
