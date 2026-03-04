# Hooks Troubleshooting

This guide helps you diagnose and fix common issues with the FileGator hooks system.

## Common Issues

### Hooks Not Executing

**Symptoms:** No hook scripts run when events occur.

**Possible Causes & Solutions:**

1. **Hooks are disabled**
   ```php
   // Check configuration.php
   'config' => [
       'enabled' => true,  // Must be true
   ],
   ```

2. **Wrong hooks path**
   ```php
   // Verify path exists
   'hooks_path' => __DIR__.'/private/hooks',  // Must be valid path
   ```

3. **Invalid hook name**
   - Only these hooks are allowed: `onUpload`, `onDelete`, `onDownload`, `onLogin`, `onLogout`, `onCreate`, `onRename`, `onMove`, `onCopy`
   - Check spelling and case (case-sensitive)

4. **Missing directory structure**
   ```
   private/hooks/
   └── onUpload/        # Directory must exist
       └── myhook.php   # Script must be inside hook directory
   ```

5. **File extension must be .php**
   ```
   ✓ myhook.php    (will execute)
   ✗ myhook.txt    (ignored)
   ✗ myhook.php.bak (ignored)
   ```

---

### Script Errors

**Symptoms:** Hook returns error, processing fails.

**Debugging Steps:**

1. **Check error logs**
   ```
   private/logs/app.log
   private/logs/hooks.log (if you set one up)
   ```

2. **Add debugging to your script**
   ```php
   <?php
   // Temporary debugging
   $debugLog = __DIR__ . '/../../logs/debug.log';
   file_put_contents($debugLog, print_r($hookData, true), FILE_APPEND);
   ```

3. **Test script standalone**
   ```php
   <?php
   // test_hook.php - run from command line
   $hookData = [
       'file_path' => '/test/file.txt',
       'file_name' => 'file.txt',
       'user' => 'test',
   ];

   include '/path/to/your/hook.php';
   ```

---

### Permission Errors

**Symptoms:** "Permission denied" errors.

**Solutions:**

1. **Check file permissions**
   ```bash
   # Scripts need to be readable
   chmod 644 private/hooks/onUpload/*.php

   # Directories need to be traversable
   chmod 755 private/hooks/
   chmod 755 private/hooks/onUpload/
   ```

2. **Check PHP process user**
   ```bash
   # Find which user runs PHP
   ps aux | grep php

   # Ensure hooks directory is owned by/accessible to that user
   chown -R www-data:www-data private/hooks/
   ```

3. **Check SELinux/AppArmor** (Linux)
   ```bash
   # Check if SELinux is blocking
   ausearch -m avc -ts recent

   # Temporarily disable to test
   setenforce 0
   ```

---

### Timeout Issues

**Symptoms:** Hook takes too long, gets killed.

**Solutions:**

1. **Increase timeout in configuration**
   ```php
   'config' => [
       'timeout' => 60,  // Increase if needed
   ],
   ```

2. **Move heavy processing to background**
   ```php
   <?php
   // Instead of:
   // $result = heavyProcess($file);  // Blocks

   // Use:
   exec("php /path/to/process.php " . escapeshellarg($file) . " &");
   return ['status' => 'processing_started'];
   ```

---

### Data Not Available

**Symptoms:** `$hookData` is empty or missing expected fields.

**Solutions:**

1. **Check hook documentation** - Different hooks receive different data
   ```php
   // onUpload provides:
   // file_path, file_name, file_size, user, home_dir

   // onLogin provides:
   // username, ip_address
   ```

2. **Use null coalescing for safety**
   ```php
   $filePath = $hookData['file_path'] ?? null;
   $user = $hookData['user'] ?? 'unknown';

   if (!$filePath) {
       return ['status' => 'error', 'message' => 'No file path'];
   }
   ```

---

### Background Process Not Running

**Symptoms:** External script doesn't execute.

**Debugging:**

1. **Check command syntax**
   ```php
   // Test command in terminal first
   // Then use in hook:
   $command = sprintf(
       'nohup /path/to/script.sh %s >> /tmp/hook.log 2>&1 &',
       escapeshellarg($filePath)
   );

   // Log the command for debugging
   file_put_contents('/tmp/command.log', $command . "\n", FILE_APPEND);

   exec($command);
   ```

2. **Check script is executable**
   ```bash
   chmod +x /path/to/script.sh
   ```

3. **Check script shebang**
   ```bash
   #!/bin/bash
   # or
   #!/usr/bin/env php
   ```

4. **Check full paths**
   - PHP exec() may not have same PATH as your shell
   - Use absolute paths for all commands

---

### Hook Order Wrong

**Symptoms:** Hooks execute in unexpected order.

**Solution:** Use numeric prefixes:
```
01_first.php     # Executes first
02_second.php    # Executes second
99_last.php      # Executes last
```

**Note:** Scripts execute alphabetically by filename.

---

## Debugging Mode

Create a debug hook to trace execution:

```php
<?php
// private/hooks/onUpload/00_debug.php
// Prefix with 00 to run first

$logFile = __DIR__ . '/../../logs/hooks_debug.log';

$entry = sprintf(
    "[%s] Hook: onUpload\nData: %s\n---\n",
    date('Y-m-d H:i:s'),
    json_encode($hookData, JSON_PRETTY_PRINT)
);

file_put_contents($logFile, $entry, FILE_APPEND);

return ['debug' => true];
```

---

## Testing Hooks

### Manual Testing

```php
<?php
// test_hooks.php - Run from command line
require_once __DIR__ . '/vendor/autoload.php';

use Filegator\Services\Hooks\Hooks;

$hooks = new Hooks();
$hooks->init([
    'enabled' => true,
    'hooks_path' => __DIR__ . '/private/hooks',
]);

// Test onUpload
$results = $hooks->trigger('onUpload', [
    'file_path' => '/test/document.pdf',
    'file_name' => 'document.pdf',
    'file_size' => 1024,
    'user' => 'testuser',
    'home_dir' => '/',
]);

print_r($results);
```

### Unit Testing

See `/tests/backend/Unit/HooksTest.php` for unit test examples.

---

## Getting Help

If you're still having issues:

1. Check the [FileGator GitHub Issues](https://github.com/filegator/filegator/issues)
2. Enable debug logging and collect detailed information
3. Create a minimal reproduction case
4. Include:
   - FileGator version
   - PHP version
   - Hook script code (sanitized)
   - Error messages
   - Log output
