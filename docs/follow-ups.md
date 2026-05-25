# Follow-up work

Residual code-review findings from PR #1 (MFA + self-service password reset) that were intentionally deferred. All P0/P1 security, reliability, and concurrency findings plus the test-coverage gaps were closed in that PR; what remains is enumerated here so the work survives the closed PR thread.

Original review artifact: `/tmp/compound-engineering/ce-code-review/20260516-130000-0bc0dee1/findings.json` (ephemeral — re-runnable via `/compound-engineering:ce-code-review`).

## Adversarial hardening

- [x] **#16 — Two-tab MFA pending pollution.** Closed by the MFA hardening pass (see `docs/solutions/`): `/login` now returns an `mfa_nonce` that must be echoed on `/login/mfa[/setup]`; mismatched nonces are rejected with the same generic 422 as missing pending.
- [x] **#25 — MFA step-2 brute-force across rotating IPs.** Closed by `MfaLockout`: per-username + per-IP counters share `lockout_attempts` / `lockout_timeout`. Lockout travels with the account, not the source.
- [x] **#26 — MFA pending state not bound to IP/UA.** Closed: pending payload now stores a binding hash. Default is User-Agent only (NAT-friendly); operators can opt in to IP-prefix binding via `mfa_pending_bind_ip_prefix` (`/24`, `/48`, or `exact`).

## API contract & UX

- [ ] **#20 — `/login` returns three distinct shapes; old API clients silently break.** Add a stable `{status, data}` discriminator and document in CHANGELOG. (manual, breaking)
- [ ] **#22 — `/password/forgot` returns 429 on IP throttle but generic 200 on email throttle.** Inconsistent + leaks distinguishing signal. Pick one (generic 200 recommended for anti-enumeration). (manual)
- [ ] **#33 — `/login/mfa` vs `/login/mfa/setup` return inconsistent wrappers.** `/login/mfa` returns the user directly; `/login/mfa/setup` returns `{user, backup_codes}`. Wrap both consistently. (manual)
- [ ] **#34 — `/listusers` shape change needs CHANGELOG note.** Changed from `UsersCollection` serialization to raw array with conditional fields. (manual)
- [ ] **#39 — Login.vue stuck after one wrong TOTP entry.** Single-use pending means a mistype returns 422 'MFA challenge expired' with no UX path. Frontend should restart at password step on that response. (manual)

## Maintainability cleanups

- [ ] **#23 — `email` field validated on storeUser/updateUser but silently discarded by non-MfaCapable adapters.** Either skip validation entirely or return 422 'Email not supported by this adapter'. (manual)
- [ ] **#32 — Single-admin MFA loss has no UI recovery path.** Document a CLI/file-edit recovery procedure in README, or add a setup-time recovery key. (manual)
- [ ] **#35 — `MfaService::isRequiredForUser` accepts but ignores `$username`; config is role-scoped.** Rename to `isRequiredForRole(string $role)` until per-user MFA requirements arrive. (manual)
- [ ] **#36 — `Security::init` hardcodes password-reset routes in default CSRF exempt list.** Default to `[]`; let `configuration_sample.php` own the list. (manual)
- [ ] **#37 — `/admin/users/{u}/reset_mfa` breaks the flat-route convention** used by `/deleteuser/{username}` etc. Rename to `/resetmfa/{username}`. (gated_auto)
- [ ] **#38 — `completeMfaLogin` non-MfaCapable fallback omits SESSION_HASH.** Currently dead (unreachable for the JsonFile adapter), but a latent P0 for any future adapter that implements `AuthInterface` but not `MfaCapableInterface`. Either hoist `establishSessionFor` to `AuthInterface`, or delete the dead branch and document the requirement. (manual)

## Minor advisories

- [ ] **#40 — `PasswordResetService::confirmReset` find+markUsed race.** Mitigated by #14's lock on `markUsed` (re-checks `used`/`expires` inside the lock), but `find` is still unlocked. Re-evaluate if observed in production. (gated_auto)
- [ ] **#41 — Session-hash pipe-separator collision risk** if user fields contain `|`. Switch `buildSessionHash` to `serialize()` or `json_encode()` before hashing. (gated_auto)

## Documentation

- [ ] **F-004 — New config blocks undocumented in `docs/configuration/security.md`**: `csrf_exempt_paths`, Mailer service, MfaService, PasswordResetService, MfaSecretCrypto (private/mfa_encryption.key), MfaLockout, `mfa_pending_bind_ua` + `mfa_pending_bind_ip_prefix`. (advisory)

## From the MFA hardening pass

- [ ] **Frontend step-up dialog** for admin CRUD + reset_mfa. Backend gate landed (AdminController requires `stepup_password` + `stepup_code` for admins with MFA enrolled); the admin panel still needs to prompt for these before submitting `/storeuser`, `/updateuser`, `/deleteuser`, and `/admin/users/{u}/reset_mfa`. (manual, frontend-only)
- [ ] **Step-up token** to amortise one TOTP across a 5-minute admin-write window. Today every sensitive admin action burns one TOTP (90s replay marker), so an admin doing 5 ops needs 5 codes. Operators are likely to push back once this lands; design a short-lived cookie-bound token issued on successful step-up that authorises further admin writes within a small TTL. (manual)
- [ ] **MFA encryption-key rotation procedure.** `private/mfa_encryption.key` has no rotation tooling. Document (and later automate) a procedure that loads every encrypted secret with the old key, re-encrypts with the new key, then atomically swaps the keyfile. (advisory)
- [ ] **Admin self-MFA-loss recovery (cross-ref #32).** If the only admin loses both their device AND their backup codes AND the keyfile, recovery requires editing `private/users.json` directly. Document the procedure in `docs/configuration/security.md` (`"mfa_enabled": false`, `"mfa_secret": null` on the admin row → log in → re-enroll). (advisory)
