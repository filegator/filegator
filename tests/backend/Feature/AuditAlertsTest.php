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
use Tests\Fakes\InMemoryMailer;
use Tests\TestCase;

/**
 * @internal
 */
class AuditAlertsTest extends TestCase
{
    protected function audits(): array
    {
        // Filter to messages addressed to the configured audit recipient,
        // since other suites (password reset etc.) share the same mailer.
        return array_values(array_filter(
            InMemoryMailer::$messages,
            function ($m) { return ($m['to'] ?? null) === 'audit@example.com'; }
        ));
    }

    protected function lastAudit(): ?array
    {
        $a = $this->audits();
        return $a ? end($a) : null;
    }

    protected function enrollMfa(string $username): string
    {
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $secret = TOTP::create()->getSecret();
        $auth->setMfaSecret($username, $secret);
        $auth->enableMfa($username, BackupCodeGenerator::hashAll(BackupCodeGenerator::generate(3, 10)));
        return $secret;
    }

    protected function establishSessionFor(string $username): void
    {
        $app = $this->sendRequest('GET', '/getuser');
        $app->resolve(AuthInterface::class)->establishSessionFor($username);
        $this->captureSession();
    }

    public function testCreatingUserFiresCreateAlert()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'mike@example.com',
            'role' => 'user',
            'permissions' => ['read', 'write'],
            'password' => 'pass123',
            'homedir' => '/mike',
            'email' => 'mike@external.test',
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertSame('audit@example.com', $msg['to']);
        $this->assertSame('audit-from@example.com', $msg['from_email']);
        $this->assertStringContainsString('New user created: mike@example.com', $msg['subject']);
        $this->assertStringContainsString('Admin: admin@example.com', $msg['text']);
        $this->assertStringContainsString('Folder: /mike', $msg['text']);
        $this->assertStringContainsString('Role: user', $msg['text']);
        $this->assertStringContainsString('Permissions: read, write', $msg['text']);
        $this->assertStringContainsString('Email: mike@external.test', $msg['text']);
    }

    public function testCreatingUserWithoutEmailReportsNone()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Nora No-Email',
            'username' => 'nora@example.com',
            'role' => 'user',
            'permissions' => [],
            'password' => 'pass123',
            'homedir' => '/nora',
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Email: (none)', $msg['text']);
        $this->assertStringContainsString('Permissions: (none)', $msg['text']);
    }

    public function testDeletingUserFiresDeleteAlert()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/john@example.com');
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('User deleted: john@example.com', $msg['subject']);
        $this->assertStringContainsString('Admin: admin@example.com', $msg['text']);
        $this->assertStringContainsString('Folder at time of deletion: /john', $msg['text']);
    }

    public function testFailedDeleteSendsNoAlert()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/nonexisting@example.com');
        $this->assertStatus(422);

        $this->assertSame([], $this->audits());
    }

    public function testUpdatingHomedirFiresFolderSubject()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/relocated',
            'role' => 'user',
            'permissions' => [],
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Folder changed for john@example.com: /john → /relocated', $msg['subject']);
        $this->assertStringContainsString('- Homedir: /john → /relocated', $msg['text']);
    }

    public function testUpdatingPermissionsFiresPermissionsSubject()
    {
        $this->signIn('admin@example.com', 'admin123');

        // John starts with the full default set; reduce to a smaller list so
        // the only meaningful change is permissions.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/john',
            'role' => 'user',
            'permissions' => ['read', 'write'],
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Permissions changed for john@example.com', $msg['subject']);
        $this->assertStringContainsString('- Permissions: read, write, upload, download, batchdownload → read, write', $msg['text']);
    }

    public function testUpdatingRoleWinsOverOtherChanges()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Role + homedir + permissions all change; subject should lead with role
        // because role sits at the top of the update priority order.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/somewhere',
            'role' => 'admin',
            'permissions' => ['read'],
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('role changed: user → admin', $msg['subject']);
        // Body lists every diff, not just the headline
        $this->assertStringContainsString('- Role: user → admin', $msg['text']);
        $this->assertStringContainsString('- Homedir: /john → /somewhere', $msg['text']);
        $this->assertStringContainsString('- Permissions:', $msg['text']);
    }

    public function testAdminResettingPasswordFiresPasswordSubject()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Keep every other field identical to john's seed values so the only
        // diff is the password reset itself.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/john',
            'role' => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'password' => 'newpassword',
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Password reset by admin for john@example.com', $msg['subject']);
        $this->assertStringContainsString('Password: (reset by admin)', $msg['text']);
    }

    public function testNameOnlyChangeIsSilent()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'Johnny Doe', // cosmetic
            'username' => 'john@example.com',
            'homedir' => '/john',
            'role' => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
        ]);
        $this->assertOk();

        $this->assertSame([], $this->audits());
    }

    public function testNoOpUpdateIsSilent()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Every field matches john's seed values — true no-op.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/john',
            'role' => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
        ]);
        $this->assertOk();

        $this->assertSame([], $this->audits());
    }

    public function testAdminResettingMfaFiresMfaResetAlert()
    {
        $this->enrollMfa('john@example.com');
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/admin/users/john@example.com/reset_mfa');
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('MFA reset by admin for john@example.com', $msg['subject']);
        $this->assertStringContainsString('Admin: admin@example.com', $msg['text']);
        $this->assertStringContainsString('Target user: john@example.com', $msg['text']);
    }

    public function testAdminCannotResetOwnMfaSendsNoAlert()
    {
        $this->enrollMfa('admin@example.com');
        // signIn() can't drive the two-step MFA flow, so establish the admin
        // session directly (route guard still gates on the admin role).
        $this->establishSessionFor('admin@example.com');

        $this->sendRequest('POST', '/admin/users/admin@example.com/reset_mfa');
        $this->assertStatus(422);

        $this->assertSame([], $this->audits());
    }

    public function testUserSelfDisablingMfaFiresAlert()
    {
        $secret = $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'john123',
            'code' => TOTP::createFromSecret($secret)->now(),
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('MFA disabled by user: john@example.com', $msg['subject']);
        $this->assertStringContainsString('Username: john@example.com', $msg['text']);
        $this->assertStringContainsString('Role: user', $msg['text']);
    }

    public function testFailedMfaDisableSendsNoAlert()
    {
        $this->enrollMfa('john@example.com');
        $this->establishSessionFor('john@example.com');

        // Wrong password — reauth gate rejects, MFA stays enabled, no audit.
        $this->sendRequest('POST', '/mfa/disable', [
            'password' => 'wrongpassword',
            'code' => '000000',
        ]);
        $this->assertUnprocessable();

        $this->assertSame([], $this->audits());
    }

    public function testEmailChangeFiresEmailSubject()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Seed an initial email so we can verify the before-value renders.
        $auth = $this->bootFreshApp()->resolve(AuthInterface::class);
        $auth->setEmail('john@example.com', 'old@example.com');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'John Doe',
            'username' => 'john@example.com',
            'homedir' => '/john',
            'role' => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'email' => 'new@example.com',
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Email changed for john@example.com', $msg['subject']);
        $this->assertStringContainsString('- Email: old@example.com → new@example.com', $msg['text']);
    }
}
