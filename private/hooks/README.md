# FileGator Plugin Event Hooks

This directory contains hook scripts that are executed in response to FileGator events.

## Available Hooks

| Hook Name | Trigger | Data Available |
|-----------|---------|----------------|
| `onUpload` | File upload completes | `file_path`, `file_name`, `file_size`, `user`, `home_dir` |
| `onDownload` | File download starts | `file_path`, `file_name`, `file_size`, `user`, `batch_download` |
| `onDelete` | File/directory deleted | `file_path`, `file_name`, `type`, `user` |
| `onCreate` | File/directory created | `file_path`, `file_name`, `type`, `user` |
| `onRename` | File/directory renamed | `old_path`, `new_path`, `old_name`, `new_name`, `directory`, `user` |
| `onMove` | File/directory moved | `source_path`, `destination_path`, `file_name`, `type`, `user` |
| `onCopy` | File/directory copied | `source_path`, `destination`, `file_name`, `type`, `user` |
| `onLogin` | User logs in | `username`, `ip_address`, `home_dir`, `role` |
| `onLogout` | User logs out | `username`, `ip_address` |

## Creating a Hook Script

1. Create a PHP file in the appropriate hook directory (e.g., `onUpload/my_hook.php`)
2. The script receives `$hookData` array with event information
3. Return `true` or `null` for success, `false` to stop processing, or an array with custom data

### Example Hook Script

```php
<?php
// onUpload/log_upload.php

$logEntry = sprintf(
    "[%s] User '%s' uploaded '%s'\n",
    date('Y-m-d H:i:s'),
    $hookData['user'],
    $hookData['file_name']
);

file_put_contents('/path/to/uploads.log', $logEntry, FILE_APPEND);

return true;
```

## Hook Execution Order

1. **Callbacks** (registered via PHP) execute first, sorted by priority (higher = earlier)
2. **Scripts** (PHP files in hook directories) execute next, in alphabetical order

## Configuration

Hooks are configured in `configuration.php`:

```php
'Filegator\Services\Hooks\HooksInterface' => [
    'handler' => '\Filegator\Services\Hooks\Hooks',
    'config' => [
        'enabled' => true,           // Enable/disable hooks
        'hooks_path' => __DIR__.'/private/hooks',
        'timeout' => 30,             // Script timeout in seconds
    ],
],
```

## Best Practices

1. **Keep hooks fast** - Long-running hooks delay user operations
2. **Handle errors gracefully** - Use try/catch blocks
3. **Use logging** - Log errors for debugging
4. **Test thoroughly** - Test hooks in development before production
5. **Use meaningful names** - Prefix scripts with numbers for ordering (e.g., `01_first.php`)

## Example Hook Scripts

Each hook directory contains `.example` files demonstrating common use cases.
To enable an example, rename it to remove the `.example` extension.
