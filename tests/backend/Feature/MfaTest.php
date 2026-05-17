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

        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
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

        $this->sendRequest('POST', '/login/mfa', ['code' => '000000']);

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

        // First attempt: wrong code, consumes the pending.
        $this->sendRequest('POST', '/login/mfa', ['code' => '000000']);
        $this->assertUnprocessable();

        // Second attempt with the right code: pending is gone → 422.
        $this->sendRequest('POST', '/login/mfa', [
            'code' => $this->totpFor($info['secret']),
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

        $this->sendRequest('POST', '/login/mfa', [
            'code' => 'ABCDE-12345',
            'use_backup' => true,
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

        $this->sendRequest('POST', '/login/mfa', [
            'code' => 'ABCDE-12345',
            'use_backup' => true,
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
        $secret = $this->decodeResponseJson()['data']['enrollment']['secret'];
        $this->captureSession();

        $this->sendRequest('POST', '/login/mfa/setup', [
            'code' => $this->totpFor($secret),
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
        $this->enrollMfa('admin@example.com');
        // signIn() can't drive the two-step MFA flow, so establish the admin
        // session directly. The route guard (admin only) still applies.
        $this->establishSessionFor('admin@example.com');

        $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa');
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
        $code = $this->totpFor($info['secret']);

        $this->sendRequest('POST', '/login/mfa', ['code' => $code]);
        $this->assertOk();
        $this->sendRequest('POST', '/logout');
        $this->captureSession();

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->captureSession();

        $this->sendRequest('POST', '/login/mfa', ['code' => $code]);
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

        // MockUsers seeds exactly four (guest, admin, john, jane).
        $this->assertCount(4, $rows, 'Test isolation drifted — expected exactly the 4 seeded users');
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
            $this->sendRequest('POST', '/login/mfa', ['code' => '000000'], [], $ip);
            $this->assertUnprocessable();
        }

        // The 4th attempt from this IP should be blocked by the per-IP MFA lock.
        $this->signOut();
        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123'], [], $ip);
        $this->captureSession();
        $this->sendRequest('POST', '/login/mfa', ['code' => '111111'], [], $ip);
        $this->assertStatus(429);
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
        $this->sendRequest('POST', '/login/mfa', ['code' => 'OLDAA-11111', 'use_backup' => true]);
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
