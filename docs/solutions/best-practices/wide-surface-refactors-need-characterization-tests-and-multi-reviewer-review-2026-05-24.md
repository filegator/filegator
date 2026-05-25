---
module: refactoring
date: 2026-05-24
problem_type: best_practice
component: development_workflow
severity: high
related_components:
  - testing_framework
  - documentation
  - frontend_stimulus
  - rails_controller
tags:
  - refactoring
  - characterization-tests
  - code-review
  - multi-reviewer
  - migration
  - api-contracts
  - frontend-backend-coupling
applies_when:
  - changing a data model field across many layers (model, persistence, serialization, request handling, frontend store, frontend forms, frontend API client)
  - renaming or replacing a value used in multiple files (string → array, scalar → struct, single → list)
  - adding a new field to an existing payload that already flows through passthrough/destructuring layers
  - rolling out a feature that ships in one PR but the change touches 20+ files
  - PR diffs over ~500 LOC where mechanical pattern-matching alone won't catch contract drift
---

# Wide-surface refactors need characterization tests AND multi-reviewer review

## Context

A "single-folder → multi-folder per user" refactor touched 30 files, ~1700 LOC across the PHP backend (auth adapters, controllers, audit services) and Vue 2 frontend (Vuex store, form components, API client, router). The owner of the work explicitly directed: *"build lots of tests... add any tests to existing code that we need too before we make changes."*

Phase 1 of the work landed **16 characterization tests with zero production change** — most importantly 10 cross-user folder-isolation tests that didn't exist in the suite before. The refactor commits followed, each keeping the suite green at every step. 306/306 tests passed after the last refactor commit.

Despite this discipline, the feature **shipped two P0 bugs** that the test suite did not catch:

1. **`frontend/api/api.js`** silently dropped the new `homedirs` array. The Vue form populated `params.homedirs`, but the service method destructured only the legacy `homedir` scalar into the actual HTTP body. Admin save returned 200 OK but only the first folder persisted. Multi-folder editing was effectively broken in production.
2. **`AuthController` response endpoints** (`/getuser`, `/login`, `/login/mfa`, `/login/mfa/setup`) returned `$auth->user()->jsonSerialize()` directly. The session-stored `active_homedir` was never merged in. Frontend `router.beforeEach` then bounced multi-folder users to the picker on every page refresh, deep link, or new tab — even when their server-side active selection was still valid.

Both were caught by running `/compound-engineering:ce-code-review` on the branch after all phases landed. The api-contract reviewer surfaced both at 90–95% confidence. A subsequent `/compound-engineering:ce-simplify-code` pass then found a **third silent-drift bug**: `AuditMailer::extractHomedirs` was missing the `trim()` that the four sibling normalisers in `User::setHomedirs`, `JsonFile::extractHomedirsFromRow`, `Database::extractHomedirsFromRow`, and `AdminController::normaliseHomedirsInput` all had. A snapshot row carrying `" /a "` would have rendered in audit subjects with the whitespace while the User object stripped it. The drift sat unflagged across five near-identical implementations.

Without those two review passes, the multi-folder feature would have shipped silently broken in two ways and carrying a latent normalisation inconsistency.

## Guidance

**For wide-surface refactors (~20+ files, ~500+ LOC, or any change that crosses the frontend/backend boundary), do both of these — not just one:**

### 1. Land characterization tests FIRST

Before changing any production code, write tests that pin the existing behaviour you must preserve. These tests will fail in revealing ways during the refactor if you break a contract you didn't realize you depended on.

Specifically:
- **Pin cross-cutting invariants** that are easy to break by accident. The multi-folder refactor's biggest existing gap was *cross-user folder isolation* — there was no test verifying that user A couldn't reach user B's homedir via a crafted path. Ten new tests filled that gap before any production code changed.
- **Pin response payload shapes** for any endpoint the frontend reads from. The refactor was going to evolve the User model; a test pinning the exact key set returned by `/getuser` for a known user would have caught the missing-active_homedir bug immediately.
- **Pin model contract behaviour** — exact key set returned by `jsonSerialize`, exact behaviour of getters/setters at the boundaries of the change.

If a characterization test surfaces a bug in pre-refactor code (e.g. a missing security check the project never tested for), fix that bug in its own commit before continuing.

### 2. Run a multi-reviewer code review on the completed branch

A green test suite proves the contract you *thought* to write. A multi-persona review surfaces contracts you didn't think to write. Specifically:

- **api-contract reviewer**: catches frontend/backend payload drift. This is the one that found both P0 bugs in the multi-folder case — neither was a test gap exactly, both were *implementation-completeness* gaps that the new tests didn't think to assert against.
- **adversarial reviewer**: actively tries to break the implementation. Found 12 distinct failure scenarios, several of which corroborated the correctness and security findings (4-way agreement on the active_homedir bug, for example).
- **simplify-code reviewers** (reuse + quality + efficiency): catches silent drift across near-duplicates. The five-way trim/filter duplication had drifted in exactly one site, hidden by all five being plausible-looking.

Each reviewer found things the others missed.

### 3. Treat reviewer corroboration as actionable signal

When 2+ reviewers independently flag the same thing, even at moderate confidence, treat it as actionable. In this refactor:
- Missing `active_homedir`: api-contract (90) + correctness (85) + adversarial (100) + maintainability (75) — fix immediately
- Stale `SESSION_CWD` after admin shrink: correctness + adversarial — fix immediately
- Database empty JSON `[]` granting root: correctness + adversarial — fix before deploy

Cross-reviewer agreement is the strongest signal that a finding is real and not persona-specific noise.

## Why this matters

Test-driven discipline ("build lots of tests... before we make changes") is necessary but not sufficient for wide-surface refactors. The tests verify contracts you wrote down; reviewers find contracts you didn't.

Three classes of bug ride this gap consistently:

| Bug class | Why tests miss it | Why reviewers catch it |
|---|---|---|
| **Passthrough layers drop new fields** | The test exercises end-to-end via the *backend*, never re-validating that the frontend service method actually carries the new field in its HTTP body | api-contract reviewer reads the request shape on both sides and notices the mismatch |
| **Response payload omits derived state** | The test asserts on `jsonSerialize` output, but the bug is in the controller wrapping that combines `jsonSerialize` with session state | api-contract + adversarial reviewers walk the full response path |
| **Silent drift across near-duplicates** | All five copies are individually correct against their own tests; the divergence is invisible without comparing them | reuse reviewer surfaces the duplication and the diff between copies |

The multi-folder refactor case was unusually well-tested *and* still shipped these. Skipping either discipline would have meant either:

- Tests-only: ship two P0 bugs, plus the latent normalisation drift
- Review-only: catch the bugs but ship without the safety net for the next refactor; lose the cross-user isolation contract pins for free

Doing both is the cheap insurance.

## When to apply

Apply this practice when **any** of the following hold:

- The change touches 20+ files in one branch
- The change crosses the frontend/backend boundary (form fields → API → controller → model)
- A data shape evolves (string → array, scalar → struct, single → list, sync → async)
- A public response payload changes shape, gains new fields, or removes legacy ones
- A core abstraction is refactored across many call sites
- The change involves a back-compat window where two payload shapes must coexist

Skip the formal multi-reviewer pass for narrowly-scoped changes — bug fixes touching one file, simple copy edits, dependency bumps. The overhead isn't worth it.

## Examples

### Characterization tests that earned their keep

From the multi-folder refactor (`tests/backend/Feature/FilesTest.php`):

```php
public function testJohnCannotListJanesHomedirViaChangedirTraversal()
{
    $this->signIn('john@example.com', 'john123');
    $this->seedJohnAndJaneWithSecret();

    foreach (['../jane', '/../jane', '../../jane', '/../../jane'] as $path) {
        $this->sendRequest('POST', '/changedir', ['to' => $path]);
        $body = (string) $this->response->getContent();
        $this->assertStringNotContainsString(
            'secret.txt',
            $body,
            "Traversal path {$path} leaked jane's secret.txt"
        );
    }
}
```

Ten such tests landed in Phase 1 (cross-user isolation), four more pinning admin-input boundary behaviour and login-payload shape. Production code changes followed in Phases 2–9. Whenever a refactor commit produced an unexpected test failure, it was the test surfacing an actual contract break — never noise.

### Bug a test alone wouldn't have caught

`frontend/api/api.js` before the fix:

```javascript
storeUser(params) {
  return new Promise((resolve, reject) => {
    axios.post('storeuser', {
      role: params.role,
      name: params.name,
      username: params.username,
      email: params.email,
      homedir: params.homedir,
      password: params.password,
      permissions: params.permissions,
    })
      // ...
  })
},
```

`UserEdit.vue::save()` was already passing `homedirs: filter(Boolean)` in its params object, and the backend was already accepting either key. **Backend tests passed**, **frontend Jest tests would have passed** if they existed for this method. But the actual production POST body carried only `homedir`. Backend's `normaliseHomedirsInput` fell through to the legacy scalar branch and saved the first folder only.

A characterization test that *would* have caught it: a mechanical fixture asserting the network request body shape itself:

```javascript
// Pseudocode — pin the request payload contract
it('storeUser POST body forwards homedirs array', async () => {
  const spy = mockAxios()
  await api.storeUser({ username: 'x', homedirs: ['/a', '/b'], ... })
  expect(spy.lastBody).toHaveProperty('homedirs', ['/a', '/b'])
})
```

Lesson: **for any passthrough layer that destructures params into a different shape, pin the output shape** — don't trust that the layer was updated when the input layer was.

### Silent drift caught by simplify-code

Five sites had this loop:

```php
foreach ($list as $h) {
    if (! is_string($h)) continue;
    $t = trim($h);
    if ($t === '') continue;
    $clean[] = $t;
}
```

`AuditMailer::extractHomedirs` had drifted to a version *without* the `trim()`. None of the audit-email tests exercised whitespace input, so the drift sat. Consolidating to a single utility (`Filegator\Utils\Homedirs::clean`) removed the drift surface entirely:

```php
public static function clean(array $list, array $default = []): array
{
    $out = [];
    foreach ($list as $h) {
        if (! is_string($h)) continue;
        $t = trim($h);
        if ($t === '') continue;
        $out[] = $t;
    }
    return $out ?: $default;
}
```

Lesson: **near-duplicates with the same intent are a drift trap.** Even when each individual copy is correct against its own callers' tests, the moment one is updated and the others aren't (or one is written slightly wrong), the bug becomes invisible until something exercises the difference. Centralise normalisation helpers eagerly, not as a follow-up.

## References

- Multi-folder PR: `https://github.com/zpaulsgrove/filegator/pull/6`
- Code-review autofix commit (caught the two P0s): `a2a1e32`
- Simplify-code commit (caught the silent drift): `d6ca9c8`
- The new shared utility: `backend/Utils/Homedirs.php`
- Characterization-tests-first commit: `43e512f` ("Pin cross-user folder isolation invariants")
