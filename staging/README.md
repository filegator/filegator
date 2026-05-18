# staging/

Operator-side artifacts for standing up a short-lived public-facing staging environment for UAT of the unreleased MFA + password-reset changes. Not part of the application source; not loaded by the running app. Safe to keep in the repo, safe to delete after UAT.

| File | Purpose |
|---|---|
| [RUNBOOK.md](RUNBOOK.md) | Step-by-step commands to run on the VM (start here) |
| [configuration.staging.patch.md](configuration.staging.patch.md) | Exact deltas to apply to prod's `configuration.php` on the VM |
| [docker-compose.staging.yml](docker-compose.staging.yml) | Adds a Caddy reverse-proxy in front of the filegator container |
| [Caddyfile](Caddyfile) | TLS termination via Let's Encrypt + sslip.io |
| [UAT-checklist.md](UAT-checklist.md) | Printable checklist for business-user testing sessions |
