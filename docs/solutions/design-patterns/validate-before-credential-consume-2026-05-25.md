---
title: Validate cheaply before consuming credentials in step-up auth flows
date: 2026-05-25
category: design-patterns
module: auth
problem_type: design_pattern
component: authentication
severity: high
applies_when:
  - Adding step-up auth (TOTP / backup code / OTP / magic link) to endpoints that can fail pre-validation
  - Wiring any single-use credential consumption (audit emit, replay marker write, rate-limit increment) into a request path
  - Reviewing admin CRUD endpoints behind a second factor
  - Auditing 2FA / MFA reset, password reset, account deletion, OTP-gated mutations
tags: [mfa, step-up, auth, validation-order, single-use-credentials, side-effects, totp, backup-codes]
related_components: [rails_controller]
---

# Validate cheaply before consuming credentials in step-up auth flows

## Context

During FileGator's MFA hardening pass (commit `7a77424` → autofix `760ac2c` → fix `bdd892a` on `master`), a multi-agent code review surfaced the same bug pattern across `AdminController::storeUser`, `updateUser`, `deleteUser`, and `resetMfa`. All four endpoints ran the step-up auth gate FIRST and pre-validation SECOND.

Four independent reviewer personas (correctness, api-contract, security, adversarial) each flagged it from a different angle, promoting the merged finding to anchor 100 confidence:

- **correctness:** "step-up consumes a code before the validation rejects the action."
- **api-contract:** "test had to be modified to supply valid step-up creds in order to keep reaching the self-reset guard — the precedence change is observable."
- **security:** "burns a TOTP / backup code on a guaranteed-422."
- **adversarial:** "self-DoS amplifier — admin can grind through their own backup codes on typos."

The literal cost per fat-fingered request:

1. A valid TOTP code was **consumed** (the 90-second replay marker prevented reuse).
2. If the admin used a backup code, it was **permanently burned** — 1 of 10 single-use codes gone, with no operator-visible signal that anything was wrong.
3. The `mfaBackupCodeConsumed` audit email fired, polluting the operator's audit stream with spurious "code used" events for requests that did nothing.
4. The request then returned 422.

The admin would notice nothing went wrong, but their next legitimate admin action 30 seconds later needed a fresh TOTP, and they'd be closer to running out of backup codes than they realized.

## Guidance

Order request handling strictly:

```
(1) Cheap validation that touches no credentials or rate-limit state
       │
       ▼
(2) Step-up auth / credential consumption / audit emit / rate-limit increment
       │
       ▼
(3) Actual mutation
```

Anything that consumes single-use or rate-limited state belongs in phase 2, never phase 1. Anything that returns a guaranteed-422 (target-not-found, self-reference guard, dup-key check, required-fields validation, role-not-permitted) belongs in phase 1.

This applies broadly:

- **TOTP codes** — 90-second replay marker means a consumed code is unusable for 90s.
- **Backup codes** — permanently single-use; one burn is one of N codes gone for good.
- **OTP tokens / magic-link tokens** — single-use by design.
- **Audit events** — emitting a "code used" / "permission granted" / "sensitive action attempted" event on a no-op request poisons the audit stream and makes real anomalies harder to spot.
- **Replay markers** — once written, the marker keeps the slot occupied until TTL expiry.
- **Rate-limit counters** — incrementing on a malformed-input failure (vs an actual auth failure) lets attackers DoS users with bogus requests.

## Why This Matters

**Credential waste.** Each burned code is a unit of recovery capacity the admin no longer has. Across many sessions, this silently degrades the admin's MFA resilience: an admin who's burned 3 backup codes on typos has 7 left, not 10, and may not know.

**Audit fidelity.** Operators monitor the backup-code-used alert as a signal of compromise. Every spurious emit on a no-op request adds noise that real attacks can hide behind. The signal-to-noise ratio of the security audit stream IS the security audit stream's value.

**Rate-limiter weaponization.** If the failed-attempt counter increments on input-validation failures (not just credential failures), an attacker who can probe the endpoint with malformed inputs can lock out legitimate users indefinitely — DoS-by-malformed-request. Per-username lockouts amplify this (attacker who knows the username can lock that user out across rotating IPs).

**Test integrity.** The smoking gun in this case was `testAdminCannotResetOwnMfa` — the original test had to be modified to supply valid step-up credentials *just to reach the self-reset guard*. That test modification was the visible artifact of the precedence inversion. A test that has to bypass real protection to reach the property it's testing is a structural signal that the property under test is no longer where it appears to be.

**Cross-reviewer corroboration is signal.** When 4 reviewers find the same bug from 4 angles, it's not opinion — it's the same load-bearing rule that each reviewer's mental model expects to hold. Trust the convergence and fix the root cause; don't argue with any single reviewer.

## When to Apply

Apply this pattern whenever all three conditions hold:

- The endpoint requires step-up authentication, OTP consumption, or any other credential burn.
- The request can fail pre-validation in ways unrelated to auth (target doesn't exist, self-reference detected, required fields missing, invalid state transition, duplicate key).
- Those pre-validation failures should not cost the requester a credential, an audit event, or a rate-limit increment.

Common scenarios: admin CRUD operations behind MFA, password reset flows, 2FA enrollment, backup-code regeneration, high-privilege permission changes, account deletion, payment confirmation.

When NOT to apply (judgment edge cases):

- If the pre-validation itself could be a probe vector (e.g., username enumeration via target-exists timing), consider whether step-up should fire first to make the endpoint indistinguishable for unauthenticated probes. In most cases the admin-only route guard already protects the endpoint, so this concern doesn't apply — but think about it.
- If the credential consumption is itself the rate limit (e.g., "you get 3 magic link sends per hour"), the consumption IS the intended cost of the attempt, even on a 422.

## Examples

### Before: step-up fires before validation

```php
// AdminController::resetMfa — commit 7a77424 (the bug)
public function resetMfa($username, ..., MfaService $mfa, MfaLockout $lockout)
{
    if (! $this->auth instanceof MfaCapableInterface) {
        return $response->json('Not supported', 501);
    }

    // ⚠️ Step-up FIRST — burns TOTP / backup code on every call, including
    // calls that are about to return 422 for self-reset or missing target.
    $check = $this->stepUpForAdmin($request, $response, $mfa, $lockout);
    if (! $check['ok']) return;
    $this->auditAdminBackupCodeIfUsed($check, $audit, $request->getClientIp());

    // ... and only THEN do we discover the request is a no-op:
    $current = $this->auth->user();
    if ($current && $current->getUsername() === $username) {
        return $response->json('Cannot reset your own MFA from the admin panel', 422);
    }
    $target = $this->auth->find($username);
    if (! $target) {
        return $response->json('User not found', 422);
    }

    $this->auth->disableMfa($username);
}
```

### After: pre-validation first, step-up only when the request is valid

```php
// AdminController::resetMfa — commit bdd892a (the fix)
public function resetMfa($username, ..., MfaService $mfa, MfaLockout $lockout)
{
    if (! $this->auth instanceof MfaCapableInterface) {
        return $response->json('Not supported', 501);
    }

    // ✅ Pre-validation FIRST — no state change, no credential cost.
    $current = $this->auth->user();
    if ($current && $current->getUsername() === $username) {
        return $response->json('Cannot reset your own MFA from the admin panel', 422);
    }
    $target = $this->auth->find($username);
    if (! $target) {
        return $response->json('User not found', 422);
    }

    // Step-up ONLY now that we know the request will land.
    $check = $this->stepUpForAdmin($request, $response, $mfa, $lockout);
    if (! $check['ok']) return;
    $this->auditBackupCodeIfUsed($check, $audit, $this->logger, $request->getClientIp());

    $this->auth->disableMfa($username);
}
```

### The regression test that pins the property

The most important test isn't "the endpoint returns 422" — it's "the credential is still usable after the 422." This is the structural test that catches future regressions:

```php
// tests/backend/Feature/AdminStepUpTest.php
public function testResetMfaSelfRejectDoesNotConsumeTotpCode()
{
    $info = $this->signInMfaAdmin();
    $code = $this->totpFor($info['secret']);

    // 1st call: self-reset with valid step-up creds.
    // MUST 422 on the self-reset guard WITHOUT consuming the TOTP code.
    $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa', [
        'stepup_password' => 'admin123',
        'stepup_code' => $code,
    ]);
    $this->assertUnprocessable();

    // 2nd call: SAME code on a legitimate operation.
    // If the self-reset attempt had consumed the code (replay marker), this
    // would 422 with "Invalid code". The fix guarantees it succeeds.
    $this->sendRequest('POST', '/deleteuser/john@example.com', [
        'stepup_password' => 'admin123',
        'stepup_code' => $code,
    ]);
    $this->assertOk();
}
```

Same shape for the backup-code path — assert that the `backup_codes_remaining` count is unchanged across a rejected call:

```php
public function testResetMfaSelfRejectDoesNotConsumeBackupCode()
{
    $info = $this->signInMfaAdmin();
    $before = $this->getMfaState('admin@example.com')['backup_codes_remaining'];

    $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa', [
        'stepup_password' => 'admin123',
        'stepup_code' => $info['backup_codes'][0],
        'stepup_use_backup' => true,
    ]);
    $this->assertUnprocessable();

    $after = $this->getMfaState('admin@example.com')['backup_codes_remaining'];
    $this->assertSame($before, $after);  // Count unchanged — code not consumed.
}
```

## Related

- [docs/solutions/best-practices/wide-surface-refactors-need-characterization-tests-and-multi-reviewer-review-2026-05-24.md](../best-practices/wide-surface-refactors-need-characterization-tests-and-multi-reviewer-review-2026-05-24.md) — the multi-reviewer-review discipline this learning was caught by. That doc covers the *process* that surfaces bugs like this one; this doc covers the *principle* that the bug violated. Reading both together: process discipline catches single-pattern bugs across an entire diff; principle docs prevent the bugs from being written in the first place.
- [backend/Services/Auth/RequiresStepUpAuth.php](../../../backend/Services/Auth/RequiresStepUpAuth.php) — the trait that performs the step-up gate. Note its docblock: it documents what it does, not when callers should call it. This doc fills the "when" gap.
- [docs/configuration/security.md](../../../docs/configuration/security.md) — operator-facing documentation of the step-up flow and its credential lifecycle.
- FileGator commit `bdd892a` — the fix that prompted this doc.
- FileGator commit `7a77424` — the original implementation that contained the bug.
