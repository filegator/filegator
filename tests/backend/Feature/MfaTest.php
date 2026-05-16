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
        $this->signIn('admin@example.com', 'admin123');

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
}
