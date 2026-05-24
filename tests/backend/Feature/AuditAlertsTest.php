<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Filegator\Services\Audit\AuditMailer;
use Filegator\Services\Audit\WeeklyDigest;
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
        $this->assertStringContainsString('- Folder: /john → /relocated', $msg['text']);
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
        $this->assertStringContainsString('- Folder: /john → /somewhere', $msg['text']);
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

    public function testWeeklyDigestRendersTableForGivenRows()
    {
        $app = $this->bootFreshApp();
        $audit = $app->resolve(AuditMailer::class);

        $rows = [
            ['username' => 'admin@example.com', 'name' => 'Admin', 'role' => 'admin', 'homedir' => '/', 'permissions' => ['read', 'write'], 'mfa_enabled' => true, 'email' => 'admin@x.test'],
            ['username' => 'john@example.com', 'name' => 'John Doe', 'role' => 'user', 'homedir' => '/john', 'permissions' => ['read'], 'mfa_enabled' => false, 'email' => null],
        ];

        $sent = $audit->sendWeeklyDigest($rows);
        $this->assertTrue($sent);

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertSame('audit@example.com', $msg['to']);
        $this->assertSame('audit-from@example.com', $msg['from_email']);
        $this->assertStringContainsString('Weekly audit digest — 2 users (1 with MFA)', $msg['subject']);
        // Text body: row block per user with all attributes
        $this->assertStringContainsString('— admin@example.com (Admin) — admin', $msg['text']);
        $this->assertStringContainsString('  Folder: /', $msg['text']);
        $this->assertStringContainsString('  MFA: on', $msg['text']);
        $this->assertStringContainsString('— john@example.com (John Doe) — user', $msg['text']);
        $this->assertStringContainsString('  MFA: off', $msg['text']);
        $this->assertStringContainsString('  Email: (none)', $msg['text']);
        // HTML body: table with both usernames and HTML-escaped folder
        $this->assertNotNull($msg['html']);
        $this->assertStringContainsString('<table', $msg['html']);
        $this->assertStringContainsString('admin@example.com', $msg['html']);
        $this->assertStringContainsString('john@example.com', $msg['html']);
    }

    public function testWeeklyDigestWithNoRowsSendsNothing()
    {
        $app = $this->bootFreshApp();
        $audit = $app->resolve(AuditMailer::class);

        $sent = $audit->sendWeeklyDigest([]);
        $this->assertFalse($sent);
        $this->assertSame([], $this->audits());
    }

    public function testWeeklyDigestEscapesHtmlInUserFields()
    {
        $app = $this->bootFreshApp();
        $audit = $app->resolve(AuditMailer::class);

        // Defence-in-depth: usernames / names are operator-controlled, but a
        // single forgotten escape would be enough for an admin who creates a
        // user with HTML in the name field to inject markup into the audit
        // inbox. Verify the escape stays applied.
        $audit->sendWeeklyDigest([[
            'username' => 'x@example.com',
            'name' => '<script>alert(1)</script>',
            'role' => 'user',
            'homedir' => '/x',
            'permissions' => [],
            'mfa_enabled' => false,
            'email' => null,
        ]]);

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringNotContainsString('<script>', $msg['html']);
        $this->assertStringContainsString('&lt;script&gt;', $msg['html']);
    }

    public function testWeeklyDigestFiresOnFirstCallWhenStateMissing()
    {
        $app = $this->bootFreshApp();
        $digest = $app->resolve(WeeklyDigest::class);
        $auth = $app->resolve(AuthInterface::class);

        $fired = $digest->maybeFire($auth);
        $this->assertTrue($fired);

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Weekly audit digest', $msg['subject']);
        // Guest is filtered out, the three seeded users land in the table.
        $this->assertStringContainsString('admin@example.com', $msg['text']);
        $this->assertStringContainsString('john@example.com', $msg['text']);
        $this->assertStringContainsString('jane@example.com', $msg['text']);
        $this->assertStringNotContainsString('Username: guest', $msg['text']);

        $this->assertFileExists(TEST_TMP_PATH.'audit_state.json');
    }

    public function testWeeklyDigestSkipsWhenWithinInterval()
    {
        $app = $this->bootFreshApp();
        $digest = $app->resolve(WeeklyDigest::class);
        $auth = $app->resolve(AuthInterface::class);

        $first = $digest->maybeFire($auth);
        $this->assertTrue($first);
        InMemoryMailer::reset();

        // Second call right after — well inside the 7-day window.
        $second = $digest->maybeFire($auth);
        $this->assertFalse($second);
        $this->assertSame([], $this->audits());
    }

    public function testWeeklyDigestFiresAgainWhenIntervalHasElapsed()
    {
        // Use a 1-second interval so the next call is "due" after sleeping.
        $this->overrideConfig([
            'services' => [
                'Filegator\\Services\\Audit\\WeeklyDigest' => [
                    'config' => ['interval_seconds' => 1],
                ],
            ],
        ]);

        $app = $this->bootFreshApp();
        $digest = $app->resolve(WeeklyDigest::class);
        $auth = $app->resolve(AuthInterface::class);

        $this->assertTrue($digest->maybeFire($auth));
        InMemoryMailer::reset();

        // Push the recorded last-sent timestamp back by 2 seconds to simulate
        // the interval elapsing without actually sleeping in the test.
        $statePath = TEST_TMP_PATH.'audit_state.json';
        $state = json_decode((string) file_get_contents($statePath), true);
        $state['last_weekly_digest_at'] = time() - 2;
        file_put_contents($statePath, json_encode($state));

        $this->assertTrue($digest->maybeFire($auth));
        $this->assertNotNull($this->lastAudit());
    }

    public function testWeeklyDigestSortsRowsAlphabeticallyByUsername()
    {
        $app = $this->bootFreshApp();
        $digest = $app->resolve(WeeklyDigest::class);
        $auth = $app->resolve(AuthInterface::class);
        $digest->maybeFire($auth);

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $adminPos = strpos($msg['text'], '— admin@example.com');
        $janePos = strpos($msg['text'], '— jane@example.com');
        $johnPos = strpos($msg['text'], '— john@example.com');
        $this->assertNotFalse($adminPos);
        $this->assertNotFalse($janePos);
        $this->assertNotFalse($johnPos);
        $this->assertLessThan($janePos, $adminPos);
        $this->assertLessThan($johnPos, $janePos);
    }

    public function testListUsersFiresDigestOnFirstAdminLoad()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->assertOk();

        // First admin-panel load on a fresh install has no prior state file,
        // so the digest is "due" immediately. Subject is the digest one,
        // not an immediate-alert subject.
        $digestAudits = array_values(array_filter($this->audits(), function ($m) {
            return strpos($m['subject'] ?? '', 'Weekly audit digest') === 0;
        }));
        $this->assertCount(1, $digestAudits);
        $this->assertStringContainsString('john@example.com', $digestAudits[0]['text']);
    }

    public function testListUsersDoesNotResendDigestWithinInterval()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->sendRequest('GET', '/listusers');
        $this->assertOk();

        $digestAudits = array_values(array_filter($this->audits(), function ($m) {
            return strpos($m['subject'] ?? '', 'Weekly audit digest') === 0;
        }));
        $this->assertCount(1, $digestAudits);
    }

    public function testUpdatingHomedirsFromSingleToMultiFiresFoldersAlert()
    {
        $this->signIn('admin@example.com', 'admin123');

        // john starts single-folder at /john; expand to two folders.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedirs'    => ['/john', '/john-extra'],
        ]);
        $this->assertOk();

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Folders changed for john@example.com', $msg['subject']);
        $this->assertStringContainsString('- Folders: /john → /john, /john-extra', $msg['text']);
    }

    public function testUpdatingHomedirsOrderIsMeaningfulInDiff()
    {
        // Order matters: [/a,/b] is meaningfully different from [/b,/a]
        // because element 0 is the user's default landing folder.
        $this->signIn('admin@example.com', 'admin123');

        // First update — establish multi-folder shape.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedirs'    => ['/a', '/b'],
        ]);
        InMemoryMailer::reset();

        // Second update — flip the order, same set.
        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedirs'    => ['/b', '/a'],
        ]);
        $this->assertOk();

        // Reordering MUST fire an alert — element 0 changed.
        $msg = $this->lastAudit();
        $this->assertNotNull($msg);
        $this->assertStringContainsString('Folders changed for john@example.com', $msg['subject']);
        $this->assertStringContainsString('/a,/b → /b,/a', $msg['subject']);
    }

    public function testWeeklyDigestRendersMultipleFoldersForMultiFolderUser()
    {
        $app = $this->bootFreshApp();
        $digest = $app->resolve(WeeklyDigest::class);
        $auth = $app->resolve(AuthInterface::class);
        $digest->maybeFire($auth);

        $msg = $this->lastAudit();
        $this->assertNotNull($msg);

        // Text body shows the multi-folder user with both folders.
        $this->assertStringContainsString('multi@example.com', $msg['text']);
        $this->assertStringContainsString('Folders: /multiA, /multiB', $msg['text']);

        // HTML body shows the folder list as a multi-line cell.
        $this->assertStringContainsString('/multiA', $msg['html']);
        $this->assertStringContainsString('/multiB', $msg['html']);

        // Single-folder users keep the singular "Folder:" label in text.
        $this->assertStringContainsString('Folder: /john', $msg['text']);
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
