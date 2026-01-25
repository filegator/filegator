# Hooks Best Practices

Follow these guidelines to create reliable, efficient, and maintainable hooks.

## 1. Keep Hooks Lightweight

Hooks run synchronously during request processing. Heavy operations should be offloaded to background processes.

**Bad** - Blocking the request:
```php
<?php
// DON'T: This blocks the upload response
$result = shell_exec("clamscan " . escapeshellarg($filePath));
// User waits for scan to complete
```

**Good** - Launching background process:
```php
<?php
// DO: Launch scan in background
$logFile = __DIR__ . '/../../logs/scan.log';
$command = sprintf(
    'nohup clamscan %s >> %s 2>&1 &',
    escapeshellarg($filePath),
    escapeshellarg($logFile)
);
exec($command);
// User gets immediate response
```

## 2. Use Explicit Execution Order

Prefix scripts with numbers for predictable execution:

```
private/hooks/onUpload/
├── 01_validate_extension.php    # First: Validate file type
├── 02_scan_virus.php            # Second: Initiate virus scan
├── 03_process_image.php         # Third: Generate thumbnails
└── 99_notify.php                # Last: Send notifications
```

## 3. Handle Errors Gracefully

Never let hook errors crash the main application:

```php
<?php
try {
    // Your hook logic
    $result = someRiskyOperation();
} catch (\Exception $e) {
    // Log the error
    error_log("Hook error: " . $e->getMessage());

    // Return gracefully
    return [
        'action' => 'continue',
        'status' => 'error',
        'error' => $e->getMessage(),
    ];
}
```

## 4. Validate All Input

Never trust hook data without validation:

```php
<?php
// Validate required fields
if (empty($hookData['file_path']) || empty($hookData['user'])) {
    return [
        'status' => 'error',
        'message' => 'Missing required data',
    ];
}

// Sanitize paths
$filePath = realpath($repositoryPath . $hookData['file_path']);
if (!$filePath || strpos($filePath, $repositoryPath) !== 0) {
    return [
        'status' => 'error',
        'message' => 'Invalid file path',
    ];
}

// Validate file exists
if (!file_exists($filePath)) {
    return [
        'status' => 'error',
        'message' => 'File not found',
    ];
}
```

## 5. Use Timeouts for External Services

When calling external APIs or services:

```php
<?php
$ctx = stream_context_create([
    'http' => [
        'timeout' => 5, // 5 second timeout
    ]
]);

$response = @file_get_contents($apiUrl, false, $ctx);

if ($response === false) {
    // Handle timeout/failure gracefully
    return ['status' => 'timeout'];
}
```

## 6. Log Hook Activity

Maintain logs for debugging and auditing:

```php
<?php
function hookLog($message, $data = []) {
    $logFile = __DIR__ . '/../../logs/hooks.log';
    $entry = sprintf(
        "[%s] %s %s\n",
        date('Y-m-d H:i:s'),
        $message,
        $data ? json_encode($data) : ''
    );
    file_put_contents($logFile, $entry, FILE_APPEND);
}

// Usage
hookLog('onUpload triggered', [
    'file' => $hookData['file_name'],
    'user' => $hookData['user'],
]);
```

## 7. Avoid Circular Dependencies

Don't create hooks that trigger other hooks that trigger the original:

```php
<?php
// BAD: Moving a file in onUpload could trigger onMove,
// which might do something that triggers onUpload again

// GOOD: Use flags or checks to prevent loops
$processingFlag = sys_get_temp_dir() . '/processing_' . md5($hookData['file_path']);
if (file_exists($processingFlag)) {
    return ['status' => 'skipped', 'reason' => 'already processing'];
}

touch($processingFlag);
try {
    // Do your processing
} finally {
    unlink($processingFlag);
}
```

## 8. Use Constants and Configuration

Externalize configuration for flexibility:

```php
<?php
// config.php in hooks directory
return [
    'max_file_size' => 10 * 1024 * 1024, // 10MB
    'allowed_extensions' => ['jpg', 'png', 'pdf', 'doc'],
    'notification_email' => 'admin@example.com',
];

// In your hook
$config = include __DIR__ . '/config.php';

if ($hookData['file_size'] > $config['max_file_size']) {
    return ['status' => 'rejected', 'reason' => 'file_too_large'];
}
```

## 9. Clean Up After Yourself

Remove temporary files and resources:

```php
<?php
$tempFile = tempnam(sys_get_temp_dir(), 'hook_');

try {
    // Use temp file
    file_put_contents($tempFile, $someData);
    // Process...
} finally {
    // Always clean up
    if (file_exists($tempFile)) {
        unlink($tempFile);
    }
}
```

## 10. Document Your Hooks

Add comprehensive comments:

```php
<?php
/**
 * File Extension Validator
 *
 * Validates uploaded files have allowed extensions.
 * Blocks upload if extension is not in whitelist.
 *
 * Configuration:
 *   Edit $allowedExtensions array below to customize.
 *
 * Returns:
 *   - array with 'action' => 'stop' if blocked
 *   - array with 'status' => 'allowed' if valid
 *
 * @hook onUpload
 * @priority 01 (runs first)
 * @author Your Name
 * @version 1.0.0
 */

$allowedExtensions = ['jpg', 'jpeg', 'png', 'gif', 'pdf', 'doc', 'docx'];

// ... implementation
```

## Security Considerations

1. **Never execute user input** - Sanitize all data before use
2. **Validate file paths** - Prevent directory traversal attacks
3. **Use escapeshellarg()** - When passing to shell commands
4. **Limit permissions** - Hooks should have minimal filesystem access
5. **Audit regularly** - Review hook scripts for security issues
