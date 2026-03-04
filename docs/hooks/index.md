# Plugin Event Hooks System

FileGator's plugin event hooks system allows you to execute custom PHP scripts or callbacks when specific events occur in the application. This enables integration with external services, custom validation, logging, security scanning, and more.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [Available Hooks](#available-hooks)
- [Creating Hook Scripts](#creating-hook-scripts)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The hooks system provides two ways to react to FileGator events:

1. **Script-based hooks**: PHP files stored in the `private/hooks` directory that are automatically executed when events occur.

2. **Callback-based hooks**: In-memory PHP callbacks registered programmatically (useful for plugins or custom integrations).

### How It Works

When an event occurs (e.g., a file upload completes), FileGator:

1. Checks if hooks are enabled
2. Validates the hook name is allowed
3. Executes all registered callback hooks (sorted by priority)
4. Executes all PHP scripts in the corresponding hook directory (alphabetically)
5. Passes event data to each hook for processing

## Configuration

Enable hooks in your `configuration.php`:

```php
'Filegator\Services\Hooks\HooksInterface' => [
    'handler' => '\Filegator\Services\Hooks\Hooks',
    'config' => [
        'enabled' => true,                      // Enable/disable hooks globally
        'hooks_path' => __DIR__.'/private/hooks', // Path to hooks directory
        'timeout' => 30,                        // Max execution time per script (seconds)
        'async' => false,                       // Reserved for future async support
    ],
],
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Whether hooks are active |
| `hooks_path` | string | `''` | Path to the hooks directory |
| `timeout` | int | `30` | Maximum script execution time in seconds |
| `async` | bool | `false` | Reserved for future asynchronous hook execution |

## Available Hooks

| Hook Name | Trigger Point | Data Provided |
|-----------|--------------|---------------|
| `onUpload` | File upload completes | `file_path`, `file_name`, `file_size`, `user`, `home_dir` |
| `onDelete` | File/folder is deleted | `file_path`, `file_name`, `type`, `user` |
| `onDownload` | File download starts | `file_path`, `file_name`, `user` |
| `onCreate` | File/folder is created | `file_path`, `file_name`, `type`, `user` |
| `onRename` | File/folder is renamed | `from`, `to`, `destination`, `user` |
| `onMove` | File/folder is moved | `from`, `to`, `user` |
| `onCopy` | File/folder is copied | `source`, `destination`, `type`, `user` |
| `onLogin` | User logs in successfully | `username`, `ip_address` |
| `onLogout` | User logs out | `username` |

For detailed information about each hook, see [Available Hooks](./available-hooks.md).

## Creating Hook Scripts

### Directory Structure

```
private/
└── hooks/
    ├── onUpload/
    │   ├── 01_validate_extension.php
    │   ├── 02_antivirus_scan.php
    │   └── 03_notify_admin.php
    ├── onDelete/
    │   └── backup_before_delete.php
    ├── onDownload/
    │   └── log_downloads.php
    └── onLogin/
        └── security_check.php
```

### Script Template

```php
<?php
/**
 * Hook Script: [Description]
 *
 * Available data in $hookData:
 * - Varies by hook type (see documentation)
 *
 * Return values:
 * - true or null: Success, continue with next hook
 * - false: Stop hook chain execution
 * - array: Custom result data
 */

// Access hook data
$filePath = $hookData['file_path'] ?? null;
$fileName = $hookData['file_name'] ?? null;
$user = $hookData['user'] ?? 'unknown';

// Your hook logic here
// ...

// Return result
return [
    'action' => 'continue',  // 'continue' or 'stop'
    'status' => 'success',
    'message' => 'Hook completed successfully',
];
```

### Execution Order

Scripts execute in **alphabetical order** by filename. Use numeric prefixes for explicit ordering:

```
01_first.php
02_second.php
03_third.php
```

## Examples

For complete examples, see:

- [Email Notification Hook](./examples/email-notification.md) - Send upload details via email
- [Antivirus Scan Hook](./examples/antivirus-scan.md) - Scan uploads for malware
- [Slack/Discord Notification Hook](./examples/notification.md) - Notify via webhooks
- [Audit Logging Hook](./examples/logging.md) - Comprehensive audit trails

## Best Practices

1. **Use prefixed filenames** for explicit execution order
2. **Keep hooks lightweight** - offload heavy processing to background jobs
3. **Handle errors gracefully** - don't crash the main application
4. **Log hook activity** for debugging and auditing
5. **Validate all input** before processing
6. **Use timeouts** for external service calls

See [Best Practices](./best-practices.md) for more details.

## Troubleshooting

Common issues and solutions are documented in [Troubleshooting](./troubleshooting.md).

## API Reference

For developers building integrations, see the [API Reference](./api-reference.md).
