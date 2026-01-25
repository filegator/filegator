<?php

namespace Tests\Unit;

use Filegator\Services\Hooks\Hooks;
use Filegator\Services\Hooks\HooksInterface;
use PHPUnit\Framework\TestCase;

/**
 * Integration tests for Hooks service with FileGator controllers
 *
 * These tests verify that hooks are properly triggered for all supported events:
 * - onUpload: File upload complete
 * - onDownload: File download
 * - onDelete: File/directory deletion
 * - onCreate: File/directory creation
 * - onRename: File/directory rename
 * - onMove: File/directory move
 * - onCopy: File/directory copy
 * - onLogin: User login
 * - onLogout: User logout
 */
class HooksIntegrationTest extends TestCase
{
    protected $hooks;
    protected $tempDir;
    protected $triggeredHooks = [];

    protected function setUp(): void
    {
        parent::setUp();

        $this->tempDir = sys_get_temp_dir() . '/filegator_hooks_integration_' . uniqid();
        mkdir($this->tempDir, 0755, true);

        $this->hooks = new Hooks();
        $this->hooks->init([
            'enabled' => true,
            'hooks_path' => $this->tempDir,
            'timeout' => 10,
        ]);

        $this->triggeredHooks = [];
    }

    protected function tearDown(): void
    {
        parent::tearDown();
        $this->removeDirectory($this->tempDir);
    }

    protected function removeDirectory(string $dir): void
    {
        if (!is_dir($dir)) {
            return;
        }
        $files = array_diff(scandir($dir), ['.', '..']);
        foreach ($files as $file) {
            $path = $dir . '/' . $file;
            is_dir($path) ? $this->removeDirectory($path) : unlink($path);
        }
        rmdir($dir);
    }

    /**
     * Helper to register a hook callback that tracks triggering
     */
    protected function registerTrackingCallback(string $hookName): void
    {
        $this->hooks->register($hookName, function ($data) use ($hookName) {
            $this->triggeredHooks[$hookName][] = $data;
            return true;
        });
    }

    // ==================== onCreate Hook Tests ====================

    public function testOnCreateHookTriggersForFiles(): void
    {
        $this->registerTrackingCallback('onCreate');

        $hookData = [
            'file_path' => '/test/document.txt',
            'file_name' => 'document.txt',
            'type' => 'file',
            'user' => 'testuser',
        ];

        $this->hooks->trigger('onCreate', $hookData);

        $this->assertArrayHasKey('onCreate', $this->triggeredHooks);
        $this->assertCount(1, $this->triggeredHooks['onCreate']);
        $this->assertEquals('/test/document.txt', $this->triggeredHooks['onCreate'][0]['file_path']);
        $this->assertEquals('file', $this->triggeredHooks['onCreate'][0]['type']);
    }

    public function testOnCreateHookTriggersForDirectories(): void
    {
        $this->registerTrackingCallback('onCreate');

        $hookData = [
            'file_path' => '/test/newfolder',
            'file_name' => 'newfolder',
            'type' => 'dir',
            'user' => 'testuser',
        ];

        $this->hooks->trigger('onCreate', $hookData);

        $this->assertCount(1, $this->triggeredHooks['onCreate']);
        $this->assertEquals('dir', $this->triggeredHooks['onCreate'][0]['type']);
    }

    // ==================== onDelete Hook Tests ====================

    public function testOnDeleteHookTriggersForFiles(): void
    {
        $this->registerTrackingCallback('onDelete');

        $hookData = [
            'file_path' => '/documents/report.pdf',
            'file_name' => 'report.pdf',
            'type' => 'file',
            'user' => 'admin',
        ];

        $this->hooks->trigger('onDelete', $hookData);

        $this->assertArrayHasKey('onDelete', $this->triggeredHooks);
        $this->assertEquals('report.pdf', $this->triggeredHooks['onDelete'][0]['file_name']);
        $this->assertEquals('admin', $this->triggeredHooks['onDelete'][0]['user']);
    }

    public function testOnDeleteHookTriggersForDirectories(): void
    {
        $this->registerTrackingCallback('onDelete');

        $hookData = [
            'file_path' => '/archive/old_backups',
            'file_name' => 'old_backups',
            'type' => 'dir',
            'user' => 'admin',
        ];

        $this->hooks->trigger('onDelete', $hookData);

        $this->assertEquals('dir', $this->triggeredHooks['onDelete'][0]['type']);
    }

    // ==================== onCopy Hook Tests ====================

    public function testOnCopyHookTriggersWithSourceAndDestination(): void
    {
        $this->registerTrackingCallback('onCopy');

        $hookData = [
            'source_path' => '/documents/original.txt',
            'destination' => '/backup',
            'file_name' => 'original.txt',
            'type' => 'file',
            'user' => 'editor',
        ];

        $this->hooks->trigger('onCopy', $hookData);

        $this->assertArrayHasKey('onCopy', $this->triggeredHooks);
        $this->assertEquals('/documents/original.txt', $this->triggeredHooks['onCopy'][0]['source_path']);
        $this->assertEquals('/backup', $this->triggeredHooks['onCopy'][0]['destination']);
    }

    public function testOnCopyHookForDirectoryCopy(): void
    {
        $this->registerTrackingCallback('onCopy');

        $hookData = [
            'source_path' => '/projects/website',
            'destination' => '/archive',
            'file_name' => 'website',
            'type' => 'dir',
            'user' => 'developer',
        ];

        $this->hooks->trigger('onCopy', $hookData);

        $this->assertEquals('dir', $this->triggeredHooks['onCopy'][0]['type']);
    }

    // ==================== onMove Hook Tests ====================

    public function testOnMoveHookTriggersWithPaths(): void
    {
        $this->registerTrackingCallback('onMove');

        $hookData = [
            'source_path' => '/inbox/message.txt',
            'destination_path' => '/archive/message.txt',
            'file_name' => 'message.txt',
            'type' => 'file',
            'user' => 'mailuser',
        ];

        $this->hooks->trigger('onMove', $hookData);

        $this->assertArrayHasKey('onMove', $this->triggeredHooks);
        $this->assertEquals('/inbox/message.txt', $this->triggeredHooks['onMove'][0]['source_path']);
        $this->assertEquals('/archive/message.txt', $this->triggeredHooks['onMove'][0]['destination_path']);
    }

    // ==================== onRename Hook Tests ====================

    public function testOnRenameHookTriggersWithOldAndNewNames(): void
    {
        $this->registerTrackingCallback('onRename');

        $hookData = [
            'old_path' => '/documents/draft.txt',
            'new_path' => '/documents/final.txt',
            'old_name' => 'draft.txt',
            'new_name' => 'final.txt',
            'directory' => '/documents',
            'user' => 'writer',
        ];

        $this->hooks->trigger('onRename', $hookData);

        $this->assertArrayHasKey('onRename', $this->triggeredHooks);
        $this->assertEquals('draft.txt', $this->triggeredHooks['onRename'][0]['old_name']);
        $this->assertEquals('final.txt', $this->triggeredHooks['onRename'][0]['new_name']);
    }

    // ==================== onDownload Hook Tests ====================

    public function testOnDownloadHookTriggersForSingleFile(): void
    {
        $this->registerTrackingCallback('onDownload');

        $hookData = [
            'file_path' => '/reports/annual_report.pdf',
            'file_name' => 'annual_report.pdf',
            'file_size' => 1024000,
            'user' => 'manager',
        ];

        $this->hooks->trigger('onDownload', $hookData);

        $this->assertArrayHasKey('onDownload', $this->triggeredHooks);
        $this->assertEquals(1024000, $this->triggeredHooks['onDownload'][0]['file_size']);
    }

    public function testOnDownloadHookTriggersForBatchDownload(): void
    {
        $this->registerTrackingCallback('onDownload');

        $hookData = [
            'file_path' => '/images/photo1.jpg',
            'file_name' => 'photo1.jpg',
            'type' => 'file',
            'batch_download' => true,
            'user' => 'photographer',
        ];

        $this->hooks->trigger('onDownload', $hookData);

        $this->assertTrue($this->triggeredHooks['onDownload'][0]['batch_download']);
    }

    // ==================== onLogin Hook Tests ====================

    public function testOnLoginHookTriggersOnSuccessfulLogin(): void
    {
        $this->registerTrackingCallback('onLogin');

        $hookData = [
            'username' => 'admin',
            'ip_address' => '192.168.1.100',
            'home_dir' => '/admin_home',
            'role' => 'admin',
        ];

        $this->hooks->trigger('onLogin', $hookData);

        $this->assertArrayHasKey('onLogin', $this->triggeredHooks);
        $this->assertEquals('admin', $this->triggeredHooks['onLogin'][0]['username']);
        $this->assertEquals('192.168.1.100', $this->triggeredHooks['onLogin'][0]['ip_address']);
        $this->assertEquals('admin', $this->triggeredHooks['onLogin'][0]['role']);
    }

    // ==================== onLogout Hook Tests ====================

    public function testOnLogoutHookTriggersOnUserLogout(): void
    {
        $this->registerTrackingCallback('onLogout');

        $hookData = [
            'username' => 'regularuser',
            'ip_address' => '10.0.0.50',
        ];

        $this->hooks->trigger('onLogout', $hookData);

        $this->assertArrayHasKey('onLogout', $this->triggeredHooks);
        $this->assertEquals('regularuser', $this->triggeredHooks['onLogout'][0]['username']);
    }

    // ==================== Multiple Hooks Tests ====================

    public function testMultipleHooksCanBeTriggeredSequentially(): void
    {
        $this->registerTrackingCallback('onCreate');
        $this->registerTrackingCallback('onDelete');
        $this->registerTrackingCallback('onRename');

        $this->hooks->trigger('onCreate', ['file_path' => '/a.txt', 'file_name' => 'a.txt', 'type' => 'file', 'user' => 'u1']);
        $this->hooks->trigger('onRename', ['old_path' => '/a.txt', 'new_path' => '/b.txt', 'old_name' => 'a.txt', 'new_name' => 'b.txt', 'directory' => '/', 'user' => 'u1']);
        $this->hooks->trigger('onDelete', ['file_path' => '/b.txt', 'file_name' => 'b.txt', 'type' => 'file', 'user' => 'u1']);

        $this->assertCount(1, $this->triggeredHooks['onCreate']);
        $this->assertCount(1, $this->triggeredHooks['onRename']);
        $this->assertCount(1, $this->triggeredHooks['onDelete']);
    }

    // ==================== Script Execution Tests ====================

    public function testOnCreateScriptExecution(): void
    {
        mkdir($this->tempDir . '/onCreate', 0755, true);

        $scriptContent = '<?php return ["action" => "logged", "item" => $hookData["file_name"]];';
        file_put_contents($this->tempDir . '/onCreate/log_creation.php', $scriptContent);

        $results = $this->hooks->trigger('onCreate', [
            'file_path' => '/new_file.txt',
            'file_name' => 'new_file.txt',
            'type' => 'file',
            'user' => 'creator',
        ]);

        $this->assertNotEmpty($results);
        $this->assertTrue($results[0]['success']);
        $this->assertEquals('logged', $results[0]['result']['action']);
        $this->assertEquals('new_file.txt', $results[0]['result']['item']);
    }

    public function testOnDeleteScriptExecution(): void
    {
        mkdir($this->tempDir . '/onDelete', 0755, true);

        $scriptContent = '<?php return ["deleted" => true, "path" => $hookData["file_path"]];';
        file_put_contents($this->tempDir . '/onDelete/audit_delete.php', $scriptContent);

        $results = $this->hooks->trigger('onDelete', [
            'file_path' => '/removed_file.txt',
            'file_name' => 'removed_file.txt',
            'type' => 'file',
            'user' => 'deleter',
        ]);

        $this->assertTrue($results[0]['success']);
        $this->assertTrue($results[0]['result']['deleted']);
    }

    public function testOnLoginScriptExecution(): void
    {
        mkdir($this->tempDir . '/onLogin', 0755, true);

        $scriptContent = '<?php return ["logged_in" => true, "user" => $hookData["username"], "ip" => $hookData["ip_address"]];';
        file_put_contents($this->tempDir . '/onLogin/login_audit.php', $scriptContent);

        $results = $this->hooks->trigger('onLogin', [
            'username' => 'secureuser',
            'ip_address' => '172.16.0.1',
            'home_dir' => '/',
            'role' => 'user',
        ]);

        $this->assertTrue($results[0]['success']);
        $this->assertEquals('secureuser', $results[0]['result']['user']);
        $this->assertEquals('172.16.0.1', $results[0]['result']['ip']);
    }

    // ==================== Hook Data Validation Tests ====================

    public function testOnCreateHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onCreate', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onCreate', [
            'file_path' => '/test.txt',
            'file_name' => 'test.txt',
            'type' => 'file',
            'user' => 'testuser',
        ]);

        $this->assertArrayHasKey('file_path', $receivedData);
        $this->assertArrayHasKey('file_name', $receivedData);
        $this->assertArrayHasKey('type', $receivedData);
        $this->assertArrayHasKey('user', $receivedData);
    }

    public function testOnCopyHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onCopy', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onCopy', [
            'source_path' => '/source.txt',
            'destination' => '/target',
            'file_name' => 'source.txt',
            'type' => 'file',
            'user' => 'copyuser',
        ]);

        $this->assertArrayHasKey('source_path', $receivedData);
        $this->assertArrayHasKey('destination', $receivedData);
        $this->assertArrayHasKey('file_name', $receivedData);
        $this->assertArrayHasKey('type', $receivedData);
        $this->assertArrayHasKey('user', $receivedData);
    }

    public function testOnMoveHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onMove', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onMove', [
            'source_path' => '/old/path.txt',
            'destination_path' => '/new/path.txt',
            'file_name' => 'path.txt',
            'type' => 'file',
            'user' => 'moveuser',
        ]);

        $this->assertArrayHasKey('source_path', $receivedData);
        $this->assertArrayHasKey('destination_path', $receivedData);
        $this->assertArrayHasKey('file_name', $receivedData);
        $this->assertArrayHasKey('type', $receivedData);
        $this->assertArrayHasKey('user', $receivedData);
    }

    public function testOnRenameHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onRename', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onRename', [
            'old_path' => '/old.txt',
            'new_path' => '/new.txt',
            'old_name' => 'old.txt',
            'new_name' => 'new.txt',
            'directory' => '/',
            'user' => 'renameuser',
        ]);

        $this->assertArrayHasKey('old_path', $receivedData);
        $this->assertArrayHasKey('new_path', $receivedData);
        $this->assertArrayHasKey('old_name', $receivedData);
        $this->assertArrayHasKey('new_name', $receivedData);
        $this->assertArrayHasKey('directory', $receivedData);
        $this->assertArrayHasKey('user', $receivedData);
    }

    public function testOnDownloadHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onDownload', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onDownload', [
            'file_path' => '/download.txt',
            'file_name' => 'download.txt',
            'file_size' => 1024,
            'user' => 'downloader',
        ]);

        $this->assertArrayHasKey('file_path', $receivedData);
        $this->assertArrayHasKey('file_name', $receivedData);
        $this->assertArrayHasKey('file_size', $receivedData);
        $this->assertArrayHasKey('user', $receivedData);
    }

    public function testOnLoginHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onLogin', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onLogin', [
            'username' => 'loginuser',
            'ip_address' => '192.168.1.1',
            'home_dir' => '/home',
            'role' => 'user',
        ]);

        $this->assertArrayHasKey('username', $receivedData);
        $this->assertArrayHasKey('ip_address', $receivedData);
        $this->assertArrayHasKey('home_dir', $receivedData);
        $this->assertArrayHasKey('role', $receivedData);
    }

    public function testOnLogoutHookDataContainsRequiredFields(): void
    {
        $receivedData = null;

        $this->hooks->register('onLogout', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $this->hooks->trigger('onLogout', [
            'username' => 'logoutuser',
            'ip_address' => '192.168.1.1',
        ]);

        $this->assertArrayHasKey('username', $receivedData);
        $this->assertArrayHasKey('ip_address', $receivedData);
    }
}
