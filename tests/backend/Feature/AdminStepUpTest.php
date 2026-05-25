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
 * Step-up auth on AdminController CRUD + resetMfa. The trait degrades to a
 * no-op when the admin has no MFA enrolled, so the existing AdminTest
 * suite (which signs in as an admin with no MFA) continues to pass
 * without changes. This file covers the MFA-enrolled-admin path that
 * triggers the password+TOTP gate. Step-up fields are `stepup_password`
 * and `stepup_code` so they do not collide with the existing `password`
 * field on storeUser/updateUser (which carries the target user's
 * password, not the admin's).
 *
 * @internal
 */
class AdminStepUpTest extends TestCase
{
    protected function enrollMfaFor(string $username, ?array $backupCodesPlain = null): array
    {
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $secret = TOTP::create()->getSecret();
        $auth->setMfaSecret($username, $secret);
        $plain = $backupCodesPlain ?: BackupCodeGenerator::generate(5, 10);
        $auth->enableMfa($username, BackupCodeGenerator::hashAll($plain));
        return ['secret' => $secret, 'backup_codes' => $plain];
    }

    protected function totpFor(string $secret): string
    {
        return TOTP::createFromSecret($secret)->now();
    }

    /**
     * Sign the admin in by manually establishing the session (signIn()
     * can't drive the two-step MFA flow). The route guard still applies.
     */
    protected function signInMfaAdmin(): array
    {
        $info = $this->enrollMfaFor('admin@example.com');
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $auth->establishSessionFor('admin@example.com');
        $this->captureSession();
        return $info;
    }

    // ---------------------------------------------------------------------
    // Missing-credentials path: every sensitive endpoint refuses without P+C.
    // ---------------------------------------------------------------------

    public function testStoreUserRejectedWithoutStepUp()
    {
        $this->signInMfaAdmin();
        $this->sendRequest('POST', '/storeuser', [
            'name' => 'New User',
            'username' => 'new@example.com',
            'password' => 'newpw',
            'homedirs' => ['/'],
        ]);
        $this->assertUnprocessable();
    }

    public function testUpdateUserRejectedWithoutStepUp()
    {
        $this->signInMfaAdmin();
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'Renamed',
            'username' => 'john@example.com',
            'homedirs' => ['/'],
        ]);
        $this->assertUnprocessable();
    }

    public function testDeleteUserRejectedWithoutStepUp()
    {
        $this->signInMfaAdmin();
        $this->sendRequest('POST', '/deleteuser/john@example.com');
        $this->assertUnprocessable();
    }

    public function testResetMfaRejectedWithoutStepUp()
    {
        $this->signInMfaAdmin();
        $this->sendRequest('POST', '/admin/users/john@example.com/reset_mfa');
        $this->assertUnprocessable();
    }

    // ---------------------------------------------------------------------
    // Happy path: valid admin password + TOTP unlocks the action.
    // ---------------------------------------------------------------------

    public function testStoreUserStepUpAcceptsCorrectAdminCredentials()
    {
        $info = $this->signInMfaAdmin();

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Brand New',
            'username' => 'brandnew@example.com',
            'password' => 'newuserpw',
            'homedirs' => ['/'],
            'role' => 'user',
            'permissions' => [],
            'stepup_password' => 'admin123',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();
    }

    public function testUpdateUserStepUpAcceptsCorrectAdminCredentials()
    {
        $info = $this->signInMfaAdmin();

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'Renamed John',
            'username' => 'john@example.com',
            'homedirs' => ['/'],
            'role' => 'user',
            'permissions' => [],
            'stepup_password' => 'admin123',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();
    }

    public function testDeleteUserStepUpAcceptsCorrectAdminCredentials()
    {
        $info = $this->signInMfaAdmin();

        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'admin123',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();
    }

    public function testResetMfaStepUpAcceptsCorrectAdminCredentials()
    {
        // Target user has MFA enrolled so reset has something to clear.
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $auth->setMfaSecret('john@example.com', TOTP::create()->getSecret());
        $auth->enableMfa('john@example.com', BackupCodeGenerator::hashAll(['ABCDE-12345']));

        $info = $this->signInMfaAdmin();

        $this->sendRequest('POST', '/admin/users/john@example.com/reset_mfa', [
            'stepup_password' => 'admin123',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertOk();

        $app = $this->sendRequest('GET', '/getuser');
        $state = $app->resolve(AuthInterface::class)->getMfaState('john@example.com');
        $this->assertFalse($state['enabled']);
    }

    // ---------------------------------------------------------------------
    // Backup-code path decrements remaining count.
    // ---------------------------------------------------------------------

    public function testStepUpAcceptsBackupCodeAndDecrementsCount()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $stateBefore = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com');
        $remainingBefore = $stateBefore['backup_codes_remaining'];

        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertOk();

        $app = $this->sendRequest('GET', '/getuser');
        $stateAfter = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com');
        $this->assertSame($remainingBefore - 1, $stateAfter['backup_codes_remaining']);
    }

    // ---------------------------------------------------------------------
    // Wrong-password / wrong-TOTP paths.
    // ---------------------------------------------------------------------

    public function testStepUpRejectsWrongPassword()
    {
        $info = $this->signInMfaAdmin();
        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'wrong-admin-pw',
            'stepup_code' => $this->totpFor($info['secret']),
        ]);
        $this->assertUnprocessable();

        // The user is still present — delete did not execute.
        $app = $this->sendRequest('GET', '/getuser');
        $this->assertNotNull($app->resolve(AuthInterface::class)->find('john@example.com'));
    }

    public function testStepUpRejectsWrongTotp()
    {
        $this->signInMfaAdmin();
        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'admin123',
            'stepup_code' => '000000',
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $this->assertNotNull($app->resolve(AuthInterface::class)->find('john@example.com'));
    }

    // ---------------------------------------------------------------------
    // Replay-marker: a single TOTP code cannot authorise two sensitive ops.
    // ---------------------------------------------------------------------

    public function testStepUpTotpCannotBeReplayedAcrossActions()
    {
        $info = $this->signInMfaAdmin();
        $code = $this->totpFor($info['secret']);

        // First action: succeeds.
        $this->sendRequest('POST', '/storeuser', [
            'name' => 'First',
            'username' => 'first@example.com',
            'password' => 'newpw',
            'homedirs' => ['/'],
            'role' => 'user',
            'permissions' => [],
            'stepup_password' => 'admin123',
            'stepup_code' => $code,
        ]);
        $this->assertOk();

        // Second action with same code: the replay marker rejects it.
        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Second',
            'username' => 'second@example.com',
            'password' => 'newpw',
            'homedirs' => ['/'],
            'role' => 'user',
            'permissions' => [],
            'stepup_password' => 'admin123',
            'stepup_code' => $code,
        ]);
        $this->assertUnprocessable();

        // Second user was NOT created.
        $app = $this->sendRequest('GET', '/getuser');
        $this->assertNull($app->resolve(AuthInterface::class)->find('second@example.com'));
    }

    // ---------------------------------------------------------------------
    // Lockout: workstream 2 honours workstream 4's per-username lockout.
    // ---------------------------------------------------------------------

    public function testStepUpLocksOutAfterTooManyFailedTotps()
    {
        $this->overrideConfig(['lockout_attempts' => 3, 'lockout_timeout' => 60]);
        $this->signInMfaAdmin();

        // 3 failed TOTPs at the step-up endpoint.
        for ($i = 0; $i < 3; $i++) {
            $this->sendRequest('POST', '/deleteuser/john@example.com', [
                'stepup_password' => 'admin123',
                'stepup_code' => '000000',
            ]);
            $this->assertUnprocessable();
        }

        // 4th attempt — even with a different code — is blocked by the lockout.
        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'admin123',
            'stepup_code' => '111111',
        ]);
        $this->assertStatus(429);
    }

    // ---------------------------------------------------------------------
    // Pre-validation runs BEFORE step-up (R-1 regression coverage).
    // Goal: prove that a no-op admin request (self-reset, missing target,
    // dup username, etc.) does NOT burn the admin's TOTP / backup code.
    // ---------------------------------------------------------------------

    public function testResetMfaSelfRejectDoesNotConsumeTotpCode()
    {
        $info = $this->signInMfaAdmin();
        $code = $this->totpFor($info['secret']);

        // First call: self-reset, valid step-up creds. Expected to 422 on
        // the self-reset guard WITHOUT consuming the TOTP code.
        $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa', [
            'stepup_password' => 'admin123',
            'stepup_code' => $code,
        ]);
        $this->assertUnprocessable();

        // Second call: use the SAME code on a legitimate operation. If the
        // self-reset attempt had consumed the code (replay marker), this
        // would 422 with "Invalid code". The fix guarantees it succeeds.
        $this->sendRequest('POST', '/deleteuser/john@example.com', [
            'stepup_password' => 'admin123',
            'stepup_code' => $code,
        ]);
        $this->assertOk();
    }

    public function testResetMfaSelfRejectDoesNotConsumeBackupCode()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $before = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        // Self-reset attempt with a backup code. Must 422 BEFORE the trait
        // consumes the code.
        $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa', [
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $after = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        // Count unchanged — the code was not consumed.
        $this->assertSame($before, $after);
    }

    public function testResetMfaUnknownTargetDoesNotConsumeBackupCode()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $before = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->sendRequest('POST', '/admin/users/nobody@nowhere.test/reset_mfa', [
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $after = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->assertSame($before, $after);
    }

    public function testStoreUserDupUsernameDoesNotConsumeBackupCode()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $before = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        // john@example.com already exists in the test fixture.
        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Imposter',
            'username' => 'john@example.com',
            'password' => 'newpw',
            'homedirs' => ['/'],
            'role' => 'user',
            'permissions' => [],
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $after = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->assertSame($before, $after);
    }

    public function testDeleteUserUnknownTargetDoesNotConsumeBackupCode()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $before = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->sendRequest('POST', '/deleteuser/nobody@nowhere.test', [
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $after = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->assertSame($before, $after);
    }

    public function testDeleteUserGuestTargetDoesNotConsumeBackupCode()
    {
        $info = $this->signInMfaAdmin();

        $app = $this->sendRequest('GET', '/getuser');
        $before = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->sendRequest('POST', '/deleteuser/guest', [
            'stepup_password' => 'admin123',
            'stepup_code' => $info['backup_codes'][0],
            'stepup_use_backup' => true,
        ]);
        $this->assertUnprocessable();

        $app = $this->sendRequest('GET', '/getuser');
        $after = $app->resolve(AuthInterface::class)->getMfaState('admin@example.com')['backup_codes_remaining'];

        $this->assertSame($before, $after);
    }
}
