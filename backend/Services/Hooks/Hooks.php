<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Hooks;

use Filegator\Services\Service;
use Filegator\Services\Logger\LoggerInterface;

/**
 * Hooks Service - Execute scripts in response to FileGator events
 *
 * This service manages hook scripts that react to events like file uploads.
 * Hook scripts are PHP files stored in a configurable hooks directory.
 *
 * Available hooks:
 * - onUpload: Triggered when a file upload completes
 *   Data: ['file_path' => string, 'file_name' => string, 'file_size' => int, 'user' => string]
 *
 * - onDelete: Triggered when a file is deleted
 *   Data: ['file_path' => string, 'file_name' => string, 'type' => 'file'|'dir', 'user' => string]
 *
 * - onDownload: Triggered when a file is downloaded
 *   Data: ['file_path' => string, 'file_name' => string, 'user' => string]
 *
 * - onLogin: Triggered on successful login
 *   Data: ['username' => string, 'ip_address' => string]
 *
 * Hook scripts receive $hookData array with event information and should return:
 * - true or null: Success, continue processing
 * - false: Failure, stop processing
 * - array: Custom result data
 */
class Hooks implements HooksInterface, Service
{
    /**
     * @var string Path to hooks directory
     */
    protected $hooksPath;

    /**
     * @var bool Whether hooks are enabled
     */
    protected $enabled = true;

    /**
     * @var bool Whether to run hooks asynchronously
     */
    protected $async = false;

    /**
     * @var int Timeout for hook script execution (seconds)
     */
    protected $timeout = 30;

    /**
     * @var array<string, array<array{callback: callable, priority: int}>> In-memory callbacks
     */
    protected $callbacks = [];

    /**
     * @var LoggerInterface|null
     */
    protected $logger;

    /**
     * @var array Allowed hook names
     */
    protected const ALLOWED_HOOKS = [
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

    /**
     * Initialize the Hooks service
     */
    public function init(array $config = [])
    {
        $this->hooksPath = rtrim($config['hooks_path'] ?? '', '/\\');
        $this->enabled = $config['enabled'] ?? true;
        $this->async = $config['async'] ?? false;
        $this->timeout = $config['timeout'] ?? 30;

        // Create hooks directory if it doesn't exist
        if ($this->hooksPath && !is_dir($this->hooksPath)) {
            @mkdir($this->hooksPath, 0755, true);
        }
    }

    /**
     * Set the logger for error reporting
     */
    public function setLogger(?LoggerInterface $logger): void
    {
        $this->logger = $logger;
    }

    /**
     * {@inheritdoc}
     */
    public function trigger(string $hookName, array $data = []): array
    {
        $results = [];

        error_log("[Hooks DEBUG] trigger() called for hook: {$hookName}");
        error_log("[Hooks DEBUG] enabled: " . ($this->enabled ? 'YES' : 'NO'));
        error_log("[Hooks DEBUG] hooksPath: " . ($this->hooksPath ?: 'NOT SET'));

        if (!$this->enabled) {
            error_log("[Hooks DEBUG] Hooks are disabled, returning empty");
            return $results;
        }

        // Validate hook name
        if (!in_array($hookName, self::ALLOWED_HOOKS)) {
            $this->log("Invalid hook name: {$hookName}");
            error_log("[Hooks DEBUG] Invalid hook name: {$hookName}");
            return $results;
        }

        // Check for scripts
        $scripts = $this->getHookScripts($hookName);
        error_log("[Hooks DEBUG] Found " . count($scripts) . " scripts for hook {$hookName}");
        foreach ($scripts as $script) {
            error_log("[Hooks DEBUG] Script: {$script}");
        }

        // Execute in-memory callbacks first
        $results = array_merge($results, $this->executeCallbacks($hookName, $data));

        // Execute hook scripts from directory
        $results = array_merge($results, $this->executeScripts($hookName, $data));

        error_log("[Hooks DEBUG] trigger() completed with " . count($results) . " results");

        return $results;
    }

    /**
     * {@inheritdoc}
     */
    public function register(string $hookName, callable $callback, int $priority = 0): void
    {
        if (!isset($this->callbacks[$hookName])) {
            $this->callbacks[$hookName] = [];
        }

        $this->callbacks[$hookName][] = [
            'callback' => $callback,
            'priority' => $priority,
        ];

        // Sort by priority (higher first)
        usort($this->callbacks[$hookName], function ($a, $b) {
            return $b['priority'] <=> $a['priority'];
        });
    }

    /**
     * {@inheritdoc}
     */
    public function getHooks(string $hookName): array
    {
        $hooks = [];

        // Add callbacks
        if (isset($this->callbacks[$hookName])) {
            foreach ($this->callbacks[$hookName] as $callback) {
                $hooks[] = ['type' => 'callback', 'callback' => $callback['callback']];
            }
        }

        // Add scripts
        foreach ($this->getHookScripts($hookName) as $script) {
            $hooks[] = ['type' => 'script', 'path' => $script];
        }

        return $hooks;
    }

    /**
     * {@inheritdoc}
     */
    public function hasHooks(string $hookName): bool
    {
        return !empty($this->callbacks[$hookName]) || !empty($this->getHookScripts($hookName));
    }

    /**
     * Get all hook script files for a given hook name
     */
    protected function getHookScripts(string $hookName): array
    {
        $scripts = [];

        if (!$this->hooksPath || !is_dir($this->hooksPath)) {
            return $scripts;
        }

        $hookDir = $this->hooksPath . DIRECTORY_SEPARATOR . $hookName;

        if (!is_dir($hookDir)) {
            return $scripts;
        }

        $files = scandir($hookDir);
        foreach ($files as $file) {
            if ($file === '.' || $file === '..') {
                continue;
            }

            $filePath = $hookDir . DIRECTORY_SEPARATOR . $file;

            // Only include .php files that are executable
            if (is_file($filePath) && pathinfo($file, PATHINFO_EXTENSION) === 'php') {
                $scripts[] = $filePath;
            }
        }

        // Sort alphabetically for predictable execution order
        sort($scripts);

        return $scripts;
    }

    /**
     * Execute in-memory callback hooks
     */
    protected function executeCallbacks(string $hookName, array $data): array
    {
        $results = [];

        if (!isset($this->callbacks[$hookName])) {
            return $results;
        }

        foreach ($this->callbacks[$hookName] as $item) {
            try {
                $result = call_user_func($item['callback'], $data);
                $results[] = [
                    'type' => 'callback',
                    'success' => true,
                    'result' => $result,
                ];

                // If callback returns false, stop processing
                if ($result === false) {
                    break;
                }
            } catch (\Throwable $e) {
                $this->log("Hook callback error for {$hookName}: " . $e->getMessage());
                $results[] = [
                    'type' => 'callback',
                    'success' => false,
                    'error' => $e->getMessage(),
                ];
            }
        }

        return $results;
    }

    /**
     * Execute hook scripts from the hooks directory
     */
    protected function executeScripts(string $hookName, array $data): array
    {
        $results = [];
        $scripts = $this->getHookScripts($hookName);

        foreach ($scripts as $scriptPath) {
            try {
                $result = $this->executeScript($scriptPath, $data);
                $results[] = [
                    'type' => 'script',
                    'path' => $scriptPath,
                    'success' => $result['success'],
                    'result' => $result['output'] ?? null,
                    'error' => $result['error'] ?? null,
                ];

                // If script returns action to stop, break
                if (isset($result['output']['action']) && $result['output']['action'] === 'stop') {
                    break;
                }
            } catch (\Throwable $e) {
                $this->log("Hook script error {$scriptPath}: " . $e->getMessage());
                $results[] = [
                    'type' => 'script',
                    'path' => $scriptPath,
                    'success' => false,
                    'error' => $e->getMessage(),
                ];
            }
        }

        return $results;
    }

    /**
     * Execute a single hook script
     */
    protected function executeScript(string $scriptPath, array $data): array
    {
        error_log("[Hooks DEBUG] executeScript: {$scriptPath}");

        if (!file_exists($scriptPath) || !is_readable($scriptPath)) {
            error_log("[Hooks DEBUG] Script not found or not readable: {$scriptPath}");
            return [
                'success' => false,
                'error' => 'Script not found or not readable',
            ];
        }

        // Create isolated scope for script execution
        $hookData = $data;
        $hookResult = null;

        // Set time limit for script execution
        $originalTimeLimit = ini_get('max_execution_time');

        try {
            set_time_limit($this->timeout);

            error_log("[Hooks DEBUG] Executing script with data: " . json_encode($data));

            // Include the script in an isolated scope
            $hookResult = (function ($scriptPath, $hookData) {
                return include $scriptPath;
            })($scriptPath, $hookData);

            set_time_limit((int)$originalTimeLimit);

            error_log("[Hooks DEBUG] Script completed successfully, result: " . json_encode($hookResult));

            return [
                'success' => true,
                'output' => $hookResult,
            ];
        } catch (\Throwable $e) {
            set_time_limit((int)$originalTimeLimit);

            error_log("[Hooks DEBUG] Script threw exception: " . $e->getMessage());
            error_log("[Hooks DEBUG] Exception trace: " . $e->getTraceAsString());

            return [
                'success' => false,
                'error' => $e->getMessage(),
            ];
        }
    }

    /**
     * Log a message if logger is available
     */
    protected function log(string $message): void
    {
        if ($this->logger) {
            $this->logger->log($message);
        }
    }

    /**
     * Check if hooks are enabled
     */
    public function isEnabled(): bool
    {
        return $this->enabled;
    }

    /**
     * Enable or disable hooks
     */
    public function setEnabled(bool $enabled): void
    {
        $this->enabled = $enabled;
    }

    /**
     * Get the hooks directory path
     */
    public function getHooksPath(): string
    {
        return $this->hooksPath;
    }
}
