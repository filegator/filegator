<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Mfa\BackupCodeGenerator;
use OTPHP\TOTP;
use Tests\TestCase;

/**
 * @internal
 */
class MfaTest extends TestCase
{
    protected function enrollMfa(string $username, ?string $secret = null, ?array $backupCodesPlain = null): array
    {
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);

        if ($secret === null) {
            $secret = TOTP::create()->getSecret();
        }
        $auth->setMfaSecret($username, $secret);

        $plain = $backupCodesPlain ?: BackupCodeGenerator::generate(3, 10);
        $auth->enableMfa($username, BackupCodeGenerator::hashAll($plain));

        return ['secret' => $secret, 'backup_codes' => $plain];
    }

    protected function totpFor(string $secret): string
    {
        return TOTP::createFromSecret($secret)->now();
    }

    /**
     * Pull the mfa_nonce from the most recent response body. Tests that
     * post to /login/mfa or /login/mfa/setup must include this in the
     * request body — the server checks it to defeat the two-tab
     * pending-state-pollution attack (follow-up #16).
     */
    protected function lastMfaNonce(): string
    {
        return (string) ($this->decodeResponseJson()['data']['mfa_nonce'] ?? '');
    }

    protected function establishSessionFor(string $username): void
    {
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $auth->establishSessionFor($username);
        $this->captureSession();
    }

    public function testLoginWithoutMfaUnchanged()
    {
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->assertOk();
        $data = $this->decodeResponseJson();
        $this->assertSame('john@example.com', $data['data']['username']);
        $this->assertArrayNotHasKey('mfa_required', $data['data']);
    }

    public function testLoginWithMfaRequiresSecondStep()
    {
        $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);

        $this->assertOk();
        $data = $this->decodeResponseJson();
        $this->assertTrue($data['data']['mfa_required'] ?? false);
    }

    public function testLoginMfaSuccess()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ]);

        $this->assertOk();
        $data = $this->decodeResponseJson();
        $this->assertSame('john@example.com', $data['data']['username']);
    }

    public function testLoginMfaWrongCodeRejected()
    {
        $this->enrollMfa('john@example.com');
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce]);

        $this->assertUnprocessable();
    }

    public function testLoginMfaPendingSingleUse()
    {
        $info = $this->enrollMfa('john@example.com');
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // First attempt: wrong code, consumes the pending.
        $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce]);
        $this->assertUnprocessable();

        // Second attempt with the right code: pending is gone → 422.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ]);
        $this->assertUnprocessable();
    }

    public function testBackupCodeWorksAndIsConsumed()
    {
        $codes = ['ABCDE-12345', 'ZZZZZ-99999'];
        $this->enrollMfa('john@example.com', null, $codes);

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => 'ABCDE-12345',
            'use_backup' => true,
            'mfa_nonce' => $nonce,
        ]);
        $this->assertOk();

        // Log out and try the same backup code again — should be consumed.
        $this->sendRequest('POST', '/logout');
        $this->captureSession();

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => 'ABCDE-12345',
            'use_backup' => true,
            'mfa_nonce' => $nonce,
        ]);
        $this->assertUnprocessable();
    }

    public function testAdminWithoutMfaIsForcedIntoSetupWhenRequired()
    {
        $this->overrideConfig(['mfa_required_for_admins' => true]);

        $this->sendRequest('POST', '/login', [
            'username' => 'admin@example.com',
            'password' => 'admin123',
        ]);

        $this->assertOk();
        $data = $this->decodeResponseJson();
        $this->assertTrue($data['data']['mfa_setup_required'] ?? false);
        $this->assertNotEmpty($data['data']['enrollment']['secret']);
        $this->assertNotEmpty($data['data']['enrollment']['otpauth_uri']);
    }

    public function testAdminCanCompleteForcedMfaSetup()
    {
        $this->overrideConfig(['mfa_required_for_admins' => true]);

        $this->sendRequest('POST', '/login', [
            'username' => 'admin@example.com',
            'password' => 'admin123',
        ]);
        $this->assertOk();
        $data = $this->decodeResponseJson()['data'];
        $secret = $data['enrollment']['secret'];
        $nonce = $data['mfa_nonce'];
        $this->captureSession();

        $this->sendRequest('POST', '/login/mfa/setup', [
            'code' => $this->totpFor($secret),
            'mfa_nonce' => $nonce,
        ]);

        $this->assertOk();
        $data = $this->decodeResponseJson();
        $this->assertNotEmpty($data['data']['user']);
        $this->assertNotEmpty($data['data']['backup_codes']);
        $this->assertCount(10, $data['data']['backup_codes']);
    }

    public function testEnrollmentFlowViaApi()
    {
        $this->signIn('john@example.com', 'john123');

        $this->sendRequest('POST', '/mfa/enroll/begin');
        $this->assertOk();
        $enroll = $this->decodeResponseJson()['data'];
        $this->assertNotEmpty($enroll['secret']);
        $this->assertStringContainsString('otpauth://', $enroll['otpauth_uri']);

        $this->sendRequest('POST', '/mfa/enroll/confirm', ['code' => '000000']);
        $this->assertUnprocessable();

        $this->sendRequest('POST', '/mfa/enroll/confirm', [
            'code' => $this->totpFor($enroll['secret']),
        ]);
        $this->assertOk();
        $confirm = $this->decodeResponseJson()['data'];
        $this->assertCount(10, $confirm['backup_codes']);

        // Regression: enrolling MFA flips mfa_enabled, which is part of the
        // session-hash tamper check. confirmEnroll must re-establish the
        // session or the user is silently logged out on the next request.
        $this->sendRequest('GET', '/getuser');
        $this->assertOk();
        $this->assertResponseJsonHas(['data' => ['username' => 'john@example.com']]);
    }

    public function testMfaStateEndpoint()
    {
        $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('GET', '/mfa/state');
        $this->assertOk();
        $state = $this->decodeResponseJson()['data'];
        $this->assertTrue($state['enabled']);
        $this->assertGreaterThan(0, $state['backup_codes_remaining']);
    }

    public function testBeginEnrollRefusedWhenAlreadyEnrolled()
    {
        // Existing enrollment with a known secret.
        $info = $this->enrollMfa('john@example.com');
        $originalSecret = $info['secret'];
        $this->establishSessionFor('john@example.com');

        // Simulate a hijacked session attempting to overwrite the secret.
        $this->sendRequest('POST', '/mfa/enroll/begin');
        $this->assertUnprocessable();

        // Stored secret must be unchanged — attacker's QR was never generated and
        // the victim's authenticator continues to work.
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $state = $auth->getMfaState('john@example.com');
        $this->assertSame($originalSecret, $state['secret']);
        $this->assertTrue($state['enabled']);
    }

    public function testDisableMfaRequiresCurrentCode()
    {
        $info = $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'wrong',
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();

        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'john123',
            'code' => '000000',
        ]);
        $this->assertUnprocessable();

        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'john123',
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();

        // Regression: disable flips mfa_enabled — must re-establish the
        // session or john gets silently logged out on the next request.
        $this->sendRequest('GET', '/getuser');
        $this->assertResponseJsonHas(['data' => ['username' => 'john@example.com']]);
    }

    public function testAdminCannotDisableMfaWhenForcedByConfig()
    {
        $this->overrideConfig(['mfa_required_for_admins' => true]);
        $info = $this->enrollMfa('admin@example.com');
        $this->establishSessionFor('admin@example.com');

        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'admin123',
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();
    }

    public function testAdminResetMfaClearsTargetUser()
    {
        $this->enrollMfa('john@example.com');
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/admin/users/john@example.com/reset_mfa');
        $this->assertOk();

        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $state = $auth->getMfaState('john@example.com');
        $this->assertFalse($state['enabled']);
        $this->assertNull($state['secret']);
    }

    public function testAdminCannotResetOwnMfa()
    {
        $info = $this->enrollMfa('admin@example.com');
        // signIn() can't drive the two-step MFA flow, so establish the admin
        // session directly. The route guard (admin only) still applies.
        $this->establishSessionFor('admin@example.com');

        // Pass valid step-up creds so the self-reset guard (not the step-up
        // gate) is what produces the 422.
        $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa', [
            'stepup_password' => 'admin123',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();
    }

    public function testTotpReplayWithinSameWindowRejected()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce1 = $this->lastMfaNonce();
        $code = $this->totpFor($info['secret']);

        $this->sendRequest('POST', '/login/mfa', ['code' => $code, 'mfa_nonce' => $nonce1]);
        $this->assertOk();
        $this->sendRequest('POST', '/logout');
        $this->captureSession();

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonce2 = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', ['code' => $code, 'mfa_nonce' => $nonce2]);
        $this->assertUnprocessable();
    }

    public function testSessionInvalidatesWhenMfaToggledRemotely()
    {
        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('GET', '/getuser');
        $this->assertResponseJsonHas(['data' => ['username' => 'john@example.com']]);

        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $auth->setMfaSecret('john@example.com', TOTP::create()->getSecret());
        $auth->enableMfa('john@example.com', BackupCodeGenerator::hashAll(['AAAAA-11111']));

        $this->sendRequest('GET', '/getuser');
        $data = $this->decodeResponseJson()['data'];
        $this->assertSame('guest', $data['role']);
    }

    public function testListUsersDoesNotLeakSecrets()
    {
        $this->enrollMfa('john@example.com');
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->assertOk();
        $body = json_encode($this->decodeResponseJson());

        $this->assertStringNotContainsString('mfa_secret', $body);
        $this->assertStringNotContainsString('mfa_backup_codes', $body);
        $this->assertStringNotContainsString('"password"', $body);
        // mfa_enabled flag is fine to expose
        $this->assertStringContainsString('mfa_enabled', $body);
    }

    // ---------------------------------------------------------------------
    // Test gap closure: #19 (listusers MFA-fields shape)
    // ---------------------------------------------------------------------

    public function testListUsersAssertsInitialMfaFieldsForEveryUser()
    {
        $this->signIn('admin@example.com', 'admin123');
        $this->sendRequest('GET', '/listusers');
        $this->assertOk();
        $rows = $this->decodeResponseJson()['data'];

        // MockUsers seeds five users: guest, admin, john, jane, plus the
        // multi-folder fixture added in Phase 2 of the homedirs refactor.
        $this->assertCount(5, $rows, 'Test isolation drifted — expected exactly the 5 seeded users');
        foreach ($rows as $row) {
            $this->assertArrayHasKey('mfa_enabled', $row, "User {$row['username']} missing mfa_enabled");
            $this->assertArrayHasKey('backup_codes_remaining', $row, "User {$row['username']} missing backup_codes_remaining");
            $this->assertArrayHasKey('email', $row, "User {$row['username']} missing email");
            $this->assertFalse($row['mfa_enabled'], "User {$row['username']} unexpectedly enrolled — state leak from prior test");
            $this->assertSame(0, $row['backup_codes_remaining'], "User {$row['username']} unexpectedly has backup codes");
            $this->assertNull($row['email'], "User {$row['username']} unexpectedly has email set");
        }
    }

    // ---------------------------------------------------------------------
    // Test gap closure: #27 (MFA step-2 lockout)
    // ---------------------------------------------------------------------

    public function testMfaStep2LocksOutAfterTooManyBadCodes()
    {
        $this->overrideConfig(['lockout_attempts' => 3, 'lockout_timeout' => 60]);
        $this->enrollMfa('john@example.com');

        // Step-1: enter pending state on a single IP. Each /login/mfa call
        // is single-use, so we must re-enter the password step between
        // failed code attempts.
        $ip = ['REMOTE_ADDR' => '9.9.9.9'];

        for ($i = 0; $i < 3; $i++) {
            $this->signOut();
            $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], $ip);
            $this->captureSession();
            $nonce = $this->lastMfaNonce();
            $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce], [], $ip);
            $this->assertUnprocessable();
        }

        // The 4th attempt from this IP should be blocked by the per-IP MFA lock.
        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], $ip);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => '111111', 'mfa_nonce' => $nonce], [], $ip);
        $this->assertStatus(429);
    }

    // ---------------------------------------------------------------------
    // Per-username MFA lockout (workstream 4 — follow-up #25)
    // ---------------------------------------------------------------------

    public function testMfaStep2LocksOutByUsernameAcrossRotatingIps()
    {
        $this->overrideConfig(['lockout_attempts' => 3, 'lockout_timeout' => 60]);
        $this->enrollMfa('john@example.com');

        // Three failed MFA attempts, each from a different IP — the per-IP
        // counter never crosses the threshold, but the per-username counter
        // does. Without per-username lockout the attacker can keep going.
        $ips = ['1.1.1.1', '2.2.2.2', '3.3.3.3'];
        foreach ($ips as $ip) {
            $this->signOut();
            $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => $ip]);
            $this->captureSession();
            $nonce = $this->lastMfaNonce();
            $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => $ip]);
            $this->assertUnprocessable();
        }

        // The 4th attempt from a totally fresh IP must still be locked — the
        // ban travels with the account, not the source.
        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => '4.4.4.4']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => '111111', 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => '4.4.4.4']);
        $this->assertStatus(429);
    }

    // TTL-expiry coverage lives in tests/backend/Unit/MfaLockoutTest.php
    // (a unit test that doesn't have to wait through HTTP round-trips for
    // each fixture step — at the feature level, HTTP setup time exceeds
    // the lockout_timeout we'd want to use, making the test flaky).

    public function testSuccessfulMfaClearsPerUsernameCounter()
    {
        $this->overrideConfig(['lockout_attempts' => 3, 'lockout_timeout' => 60]);
        $info = $this->enrollMfa('john@example.com');

        // Two failures (one short of the lockout threshold).
        foreach (['10.0.0.1', '10.0.0.2'] as $ip) {
            $this->signOut();
            $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => $ip]);
            $this->captureSession();
            $nonce = $this->lastMfaNonce();
            $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => $ip]);
            $this->assertUnprocessable();
        }

        // A legitimate success resets the username counter — so the user does
        // not stay one failure away from lockout after a fat-fingered code.
        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => '10.0.0.3']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => $this->totpFor($info['secret']), 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => '10.0.0.3']);
        $this->assertOk();

        // Two more failures should NOT trip the lockout — the counter is back to zero.
        // (Per-IP TTL only resets on timeout, so use new IPs.)
        $this->sendRequest('POST', '/logout');
        $this->captureSession();
        foreach (['10.0.0.4', '10.0.0.5'] as $ip) {
            $this->signOut();
            $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => $ip]);
            $this->captureSession();
            $nonce = $this->lastMfaNonce();
            $this->sendRequest('POST', '/login/mfa', ['code' => '000000', 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => $ip]);
            $this->assertUnprocessable();
        }

        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], ['REMOTE_ADDR' => '10.0.0.6']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => '111111', 'mfa_nonce' => $nonce], [], ['REMOTE_ADDR' => '10.0.0.6']);
        // Still 422 (not 429) — username counter was cleared after the win.
        $this->assertUnprocessable();
    }

    // ---------------------------------------------------------------------
    // Test gap closure: #28 (/me/email happy + invalid + duplicate)
    // ---------------------------------------------------------------------

    public function testUpdateOwnEmailHappyPath()
    {
        $this->signIn('john@example.com', 'john123');

        $this->sendRequest('POST', '/me/email', ['email' => 'New.John@Reset.Test']);
        $this->assertOk();
        $this->assertResponseJsonHas(['data' => ['email' => 'new.john@reset.test']]);

        // /getuser must still return john — email is part of buildSessionHash
        // so updateEmail must re-establish the session.
        $this->sendRequest('GET', '/getuser');
        $this->assertOk();
        $this->assertResponseJsonHas(['data' => ['username' => 'john@example.com']]);

        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $this->assertSame('new.john@reset.test', $auth->getEmail('john@example.com'));
    }

    public function testUpdateOwnEmailRejectsInvalidFormat()
    {
        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/me/email', ['email' => 'not-an-email']);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $this->assertNull($auth->getEmail('john@example.com'));
    }

    public function testUpdateOwnEmailRejectsDuplicate()
    {
        // Pre-seed jane's email.
        $app = $this->sendRequest('GET', '/getuser');
        $app->resolve(AuthInterface::class)->setEmail('jane@example.com', 'shared@example.test');

        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/me/email', ['email' => 'shared@example.test']);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $this->assertNull($app->resolve(AuthInterface::class)->getEmail('john@example.com'));
    }

    public function testUpdateOwnEmailAllowsClearing()
    {
        $app = $this->sendRequest('GET', '/getuser');
        $app->resolve(AuthInterface::class)->setEmail('john@example.com', 'existing@example.test');

        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/me/email', ['email' => '']);
        $this->assertOk();

        $app = $this->sendRequest('GET', '/getuser');
        $this->assertNull($app->resolve(AuthInterface::class)->getEmail('john@example.com'));
    }

    // ---------------------------------------------------------------------
    // Test gap closure: #29 (/mfa/backup_codes/regenerate)
    // ---------------------------------------------------------------------

    public function testRegenerateBackupCodesReturnsTenAndInvalidatesOld()
    {
        $oldCodes = ['OLDAA-11111', 'OLDBB-22222'];
        $info = $this->enrollMfa('john@example.com', null, $oldCodes);
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('POST', '/mfa/backup_codes/regenerate', [
            'password' => 'john123',
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();
        $newCodes = $this->decodeResponseJson()['data']['backup_codes'];
        $this->assertCount(10, $newCodes);
        // Brand-new codes should not include the originals.
        $this->assertEmpty(array_intersect($oldCodes, $newCodes));

        // Old codes must no longer log the user in via the backup-code path.
        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => 'OLDAA-11111', 'use_backup' => true, 'mfa_nonce' => $nonce]);
        $this->assertUnprocessable();
    }

    public function testRegenerateBackupCodesRejectsWrongPassword()
    {
        $info = $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('POST', '/mfa/backup_codes/regenerate', [
            'password' => 'wrong',
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();
    }

    public function testRegenerateBackupCodesRejectsWrongTotp()
    {
        $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('POST', '/mfa/backup_codes/regenerate', [
            'password' => 'john123',
            'code' => '000000',
        ]);
        $this->assertUnprocessable();
    }

    // ---------------------------------------------------------------------
    // TOTP encryption at rest (workstream 1)
    // ---------------------------------------------------------------------

    public function testLegacyPlaintextSecretIsLazyMigratedOnSuccessfulVerify()
    {
        // enrollMfa() writes the secret as plaintext (test convenience —
        // bypasses MfaService::beginEnrollment which now encrypts).
        $info = $this->enrollMfa('john@example.com');

        // Snapshot the raw stored value before login — should be plaintext.
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $stateBefore = $auth->getMfaState('john@example.com');
        $this->assertSame($info['secret'], $stateBefore['secret']);
        $this->assertStringStartsNotWith('v1$', $stateBefore['secret']);

        // Complete a real MFA login — this triggers the lazy re-encrypt.
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => $this->totpFor($info['secret']), 'mfa_nonce' => $nonce]);
        $this->assertOk();

        // The stored value should now be the encrypted form.
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $stateAfter = $auth->getMfaState('john@example.com');
        $this->assertStringStartsWith('v1$', $stateAfter['secret']);
        $this->assertNotSame($info['secret'], $stateAfter['secret']);
    }

    public function testCorruptedEncryptedSecretStillAllowsBackupCodeLogin()
    {
        // Enroll real backup codes, then overwrite the stored secret with a
        // corrupted ciphertext — simulates a key rotation or disk corruption.
        $backupCodes = ['ABCDE-12345', 'WXYZH-98765'];
        $this->enrollMfa('john@example.com', null, $backupCodes);

        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        // Valid v1$ prefix but the payload won't decrypt against the real key.
        $auth->setMfaSecret('john@example.com', 'v1$'.base64_encode(str_repeat("\x00", 64)));

        // TOTP verify is impossible — secret is unrecoverable.
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => '123456', 'mfa_nonce' => $nonce]);
        $this->assertUnprocessable();

        // But backup codes are hashed separately and unaffected.
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();
        $this->sendRequest('POST', '/login/mfa', ['code' => 'ABCDE-12345', 'use_backup' => true, 'mfa_nonce' => $nonce]);
        $this->assertOk();
    }

    // ---------------------------------------------------------------------
    // Pending-state binding + nonce (workstreams 5 & 6 — follow-ups #16, #26)
    // ---------------------------------------------------------------------

    public function testPendingBindingRejectsMismatchedUserAgent()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Original']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Same cookie + nonce, but different UA — cookie-theft scenario.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Attacker']);
        $this->assertUnprocessable();
    }

    public function testPendingBindingAcceptsMatchingUserAgent()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Same']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Same']);
        $this->assertOk();
    }

    public function testIpPrefixBindingMatchesWithinSameSubnet()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => '/24']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Different last octet, same /24 — accepted.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '192.168.1.99']);
        $this->assertOk();
    }

    public function testIpPrefixBindingRejectsDifferentSubnet()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => '/24']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Different /24 — rejected.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '192.168.2.10']);
        $this->assertUnprocessable();
    }

    public function testIpExactBindingRejectsAnyDifferentIp()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => 'exact']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Even a single-octet change in the same /24 must reject under exact.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '192.168.1.11']);
        $this->assertUnprocessable();
    }

    public function testIpExactBindingAcceptsExactSameIp()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => 'exact']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->assertOk();
    }

    public function testIpPrefixBindingIpv6Slash48Matches()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => '/48']);
        $info = $this->enrollMfa('john@example.com');

        // Same /48: differ only in the 4th hextet onward.
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '2001:db8:abcd:1234::1']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '2001:db8:abcd:5678::99']);
        $this->assertOk();
    }

    public function testIpPrefixBindingIpv6Slash48RejectsDifferentPrefix()
    {
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => '/48']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '2001:db8:abcd:1234::1']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Third hextet differs → different /48 → rejected.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '2001:db8:beef:1234::1']);
        $this->assertUnprocessable();
    }

    public function testIpPrefixBindingUnknownModeFallsBackToExact()
    {
        // 'foo' is not a known mode — normalizeIpForBinding falls back to
        // exact-match. Documented behaviour: unknown modes are safe-by-default
        // (strict), not permissive. A typo locks down rather than opens up.
        $this->overrideConfig(['mfa_pending_bind_ip_prefix' => 'foo']);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['REMOTE_ADDR' => '192.168.1.10']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        // Any IP change should reject under fallback-to-exact.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['REMOTE_ADDR' => '192.168.1.11']);
        $this->assertUnprocessable();
    }

    public function testUaBindingDisabledToleratesUserAgentChange()
    {
        // With both UA-binding OFF and no IP-binding configured, the pending
        // binding collapses to a constant hash — any client can complete
        // step 2 of the matched session. Operators see a WARNING log;
        // tested separately. This case confirms the documented behaviour.
        $this->overrideConfig(['mfa_pending_bind_ua' => false]);
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Browser-A']);
        $this->captureSession();
        $nonce = $this->lastMfaNonce();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonce,
        ], [], ['HTTP_USER_AGENT' => 'Mozilla/Browser-B']);
        $this->assertOk();
    }

    public function testNonceAbsentRejected()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();

        // No mfa_nonce in body — must reject.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();
    }

    public function testNonceTamperedRejected()
    {
        $info = $this->enrollMfa('john@example.com');

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => 'deadbeefdeadbeef',
        ]);
        $this->assertUnprocessable();
    }

    public function testTwoTabRaceOlderNonceLoses()
    {
        $info = $this->enrollMfa('john@example.com');

        // Tab 1 opens MFA — captures nonce A and the session.
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonceA = $this->lastMfaNonce();

        // Tab 2 starts MFA on the same cookie — captures nonce B, overwrites pending.
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();
        $nonceB = $this->lastMfaNonce();
        $this->assertNotSame($nonceA, $nonceB);

        // Tab 1 (older nonce) submits — rejected.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
            'mfa_nonce' => $nonceA,
        ]);
        $this->assertUnprocessable();
    }

    // ---------------------------------------------------------------------
    // Test gap closure: #31 (session-validity true negative)
    // ---------------------------------------------------------------------

    public function testSessionRemainsValidWhenNonHashedFieldChanges()
    {
        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('GET', '/getuser');
        $this->assertResponseJsonHas(['data' => ['username' => 'john@example.com']]);

        // Mutate a field that is NOT part of buildSessionHash (display name only).
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $user = $auth->find('john@example.com');
        $user->setName('Johnathan Doe');
        $auth->update('john@example.com', $user, '');

        $this->sendRequest('GET', '/getuser');
        $data = $this->decodeResponseJson()['data'];
        // Still authenticated as john, not logged out.
        $this->assertSame('john@example.com', $data['username']);
        $this->assertSame('user', $data['role']);
    }
}
