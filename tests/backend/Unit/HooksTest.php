<?php

namespace Tests\Unit;

use Filegator\Services\Hooks\Hooks;
use PHPUnit\Framework\TestCase;

class HooksTest extends TestCase
{
    protected $hooks;
    protected $tempDir;

    protected function setUp(): void
    {
        parent::setUp();

        // Create a temporary directory for test hooks
        $this->tempDir = sys_get_temp_dir() . '/filegator_hooks_test_' . uniqid();
        mkdir($this->tempDir, 0755, true);
        mkdir($this->tempDir . '/onUpload', 0755, true);

        $this->hooks = new Hooks();
        $this->hooks->init([
            'enabled' => true,
            'hooks_path' => $this->tempDir,
            'timeout' => 10,
        ]);
    }

    protected function tearDown(): void
    {
        parent::tearDown();

        // Clean up temp directory
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

    public function testHooksAreEnabledByDefault(): void
    {
        $hooks = new Hooks();
        $hooks->init([
            'hooks_path' => $this->tempDir,
        ]);

        $this->assertTrue($hooks->isEnabled());
    }

    public function testHooksCanBeDisabled(): void
    {
        $hooks = new Hooks();
        $hooks->init([
            'enabled' => false,
            'hooks_path' => $this->tempDir,
        ]);

        $this->assertFalse($hooks->isEnabled());
    }

    public function testRegisterCallback(): void
    {
        $called = false;

        $this->hooks->register('onUpload', function ($data) use (&$called) {
            $called = true;
            return true;
        });

        $this->assertTrue($this->hooks->hasHooks('onUpload'));

        $this->hooks->trigger('onUpload', ['file_path' => '/test.txt']);

        $this->assertTrue($called);
    }

    public function testCallbackReceivesData(): void
    {
        $receivedData = null;

        $this->hooks->register('onUpload', function ($data) use (&$receivedData) {
            $receivedData = $data;
            return true;
        });

        $testData = [
            'file_path' => '/test.txt',
            'file_name' => 'test.txt',
            'file_size' => 1024,
        ];

        $this->hooks->trigger('onUpload', $testData);

        $this->assertEquals($testData, $receivedData);
    }

    public function testMultipleCallbacksWithPriority(): void
    {
        $order = [];

        $this->hooks->register('onUpload', function ($data) use (&$order) {
            $order[] = 'low';
            return true;
        }, -10);

        $this->hooks->register('onUpload', function ($data) use (&$order) {
            $order[] = 'high';
            return true;
        }, 10);

        $this->hooks->register('onUpload', function ($data) use (&$order) {
            $order[] = 'normal';
            return true;
        }, 0);

        $this->hooks->trigger('onUpload', []);

        $this->assertEquals(['high', 'normal', 'low'], $order);
    }

    public function testCallbackReturningFalseStopsExecution(): void
    {
        $executed = [];

        $this->hooks->register('onUpload', function ($data) use (&$executed) {
            $executed[] = 'first';
            return false; // Stop execution
        }, 10);

        $this->hooks->register('onUpload', function ($data) use (&$executed) {
            $executed[] = 'second';
            return true;
        }, 0);

        $this->hooks->trigger('onUpload', []);

        $this->assertEquals(['first'], $executed);
    }

    public function testTriggerWithInvalidHookNameReturnsEmpty(): void
    {
        $results = $this->hooks->trigger('invalidHookName', []);

        $this->assertEmpty($results);
    }

    public function testScriptExecution(): void
    {
        // Create a test hook script
        $scriptContent = '<?php return ["status" => "success", "processed" => $hookData["file_path"]];';
        file_put_contents($this->tempDir . '/onUpload/test_hook.php', $scriptContent);

        $results = $this->hooks->trigger('onUpload', ['file_path' => '/test.txt']);

        $this->assertNotEmpty($results);
        $this->assertTrue($results[0]['success']);
        $this->assertEquals('success', $results[0]['result']['status']);
        $this->assertEquals('/test.txt', $results[0]['result']['processed']);
    }

    public function testScriptWithErrorHandling(): void
    {
        // Create a script that throws an exception
        $scriptContent = '<?php throw new Exception("Test error");';
        file_put_contents($this->tempDir . '/onUpload/error_hook.php', $scriptContent);

        $results = $this->hooks->trigger('onUpload', []);

        $this->assertNotEmpty($results);
        $this->assertFalse($results[0]['success']);
        $this->assertStringContainsString('Test error', $results[0]['error']);
    }

    public function testGetHooksReturnsCallbacksAndScripts(): void
    {
        // Add a callback
        $this->hooks->register('onUpload', function ($data) {
            return true;
        });

        // Create a script
        file_put_contents($this->tempDir . '/onUpload/test_hook.php', '<?php return true;');

        $hooks = $this->hooks->getHooks('onUpload');

        $this->assertCount(2, $hooks);
        $this->assertEquals('callback', $hooks[0]['type']);
        $this->assertEquals('script', $hooks[1]['type']);
    }

    public function testHasHooksReturnsFalseForNoHooks(): void
    {
        $this->assertFalse($this->hooks->hasHooks('onDownload'));
    }

    public function testDisabledHooksDoNotExecute(): void
    {
        $this->hooks->setEnabled(false);

        $executed = false;
        $this->hooks->register('onUpload', function ($data) use (&$executed) {
            $executed = true;
            return true;
        });

        $this->hooks->trigger('onUpload', []);

        $this->assertFalse($executed);
    }

    public function testScriptsExecuteAlphabetically(): void
    {
        $order = [];

        // Create scripts that will execute in alphabetical order
        file_put_contents(
            $this->tempDir . '/onUpload/a_first.php',
            '<?php global $order; $GLOBALS["test_order"][] = "a"; return true;'
        );
        file_put_contents(
            $this->tempDir . '/onUpload/b_second.php',
            '<?php global $order; $GLOBALS["test_order"][] = "b"; return true;'
        );
        file_put_contents(
            $this->tempDir . '/onUpload/c_third.php',
            '<?php global $order; $GLOBALS["test_order"][] = "c"; return true;'
        );

        $GLOBALS['test_order'] = [];
        $this->hooks->trigger('onUpload', []);

        $this->assertEquals(['a', 'b', 'c'], $GLOBALS['test_order']);
        unset($GLOBALS['test_order']);
    }

    public function testGetHooksPath(): void
    {
        $this->assertEquals($this->tempDir, $this->hooks->getHooksPath());
    }

    public function testTriggerReturnsResults(): void
    {
        $this->hooks->register('onUpload', function ($data) {
            return ['processed' => true];
        });

        $results = $this->hooks->trigger('onUpload', []);

        $this->assertIsArray($results);
        $this->assertTrue($results[0]['success']);
        $this->assertEquals(['processed' => true], $results[0]['result']);
    }

    public function testAllowedHooks(): void
    {
        $allowedHooks = [
            'onUpload',
            'onDelete',
            'onDownload',
            'onLogin',
            'onLogout',
            'onCreate',
            'onRename',
            'onMove',
            'onCopy',
        ];

        foreach ($allowedHooks as $hookName) {
            mkdir($this->tempDir . '/' . $hookName, 0755, true);
            file_put_contents($this->tempDir . '/' . $hookName . '/test.php', '<?php return true;');

            $results = $this->hooks->trigger($hookName, []);

            // Should execute without being filtered out
            $this->assertNotEmpty($results, "Hook {$hookName} should be allowed");
        }
    }
}
