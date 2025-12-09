# Available Hooks

This document details all available hooks, their trigger points, and the data passed to hook scripts.

## File Operation Hooks

### onUpload

Triggered when a file upload completes successfully.

**When**: After all chunks are assembled and the file is stored in the repository.

**Data provided**:
```php
$hookData = [
    'file_path' => '/path/to/file.txt',  // Destination path in repository
    'file_name' => 'file.txt',            // Original filename
    'file_size' => 1048576,               // File size in bytes
    'user' => 'john',                     // Username who uploaded
    'home_dir' => '/users/john',          // User's home directory
];
```

**Use cases**:
- Virus scanning uploaded files
- Validating file contents
- Processing/converting uploaded files
- Notifying administrators
- Creating thumbnails for images
- Indexing file content for search

**Example**:
```php
<?php
// Log upload to external service
$logEntry = sprintf(
    "File uploaded: %s by %s (%d bytes)",
    $hookData['file_name'],
    $hookData['user'],
    $hookData['file_size']
);

file_put_contents('/var/log/uploads.log', $logEntry . "\n", FILE_APPEND);

return ['status' => 'logged'];
```

---

### onDelete

Triggered before a file or directory is deleted.

**When**: After the delete request is validated but before actual deletion.

**Data provided**:
```php
$hookData = [
    'file_path' => '/path/to/file.txt',  // Path being deleted
    'file_name' => 'file.txt',           // Name of file/folder
    'type' => 'file',                    // 'file' or 'dir'
    'user' => 'john',                    // Username performing deletion
];
```

**Use cases**:
- Creating backups before deletion
- Audit logging
- Preventing deletion of protected files
- Cleaning up related resources

**Example**:
```php
<?php
// Backup file before deletion
$backupDir = '/var/backups/filegator/' . date('Y-m-d');
if (!is_dir($backupDir)) {
    mkdir($backupDir, 0755, true);
}

$repoPath = __DIR__ . '/../../repository';
$fullPath = $repoPath . $hookData['file_path'];

if (file_exists($fullPath) && $hookData['type'] === 'file') {
    copy($fullPath, $backupDir . '/' . $hookData['file_name']);
}

return ['backed_up' => true];
```

---

### onDownload

Triggered when a file download is initiated.

**When**: Before the file stream is sent to the client.

**Data provided**:
```php
$hookData = [
    'file_path' => '/path/to/file.txt',  // Path being downloaded
    'file_name' => 'file.txt',           // Filename
    'user' => 'john',                    // Username downloading
];
```

**Use cases**:
- Download tracking/analytics
- Access logging for compliance
- Bandwidth throttling
- Checking download permissions

---

### onCreate

Triggered when a new file or directory is created.

**When**: After the file/directory creation completes.

**Data provided**:
```php
$hookData = [
    'file_path' => '/path/',             // Parent directory
    'file_name' => 'newfile.txt',        // Created item name
    'type' => 'file',                    // 'file' or 'dir'
    'user' => 'john',                    // Username
];
```

---

### onRename

Triggered when a file or directory is renamed.

**When**: After the rename operation completes.

**Data provided**:
```php
$hookData = [
    'from' => 'oldname.txt',             // Original name
    'to' => 'newname.txt',               // New name
    'destination' => '/path/',           // Directory containing the item
    'user' => 'john',                    // Username
];
```

---

### onMove

Triggered when a file or directory is moved.

**When**: After the move operation completes.

**Data provided**:
```php
$hookData = [
    'from' => '/old/path/file.txt',      // Original path
    'to' => '/new/path/file.txt',        // New path
    'user' => 'john',                    // Username
];
```

---

### onCopy

Triggered when a file or directory is copied.

**When**: After the copy operation completes.

**Data provided**:
```php
$hookData = [
    'source' => '/source/file.txt',      // Source path
    'destination' => '/dest/',           // Destination directory
    'type' => 'file',                    // 'file' or 'dir'
    'user' => 'john',                    // Username
];
```

---

## Authentication Hooks

### onLogin

Triggered when a user successfully logs in.

**When**: After credentials are verified and session is created.

**Data provided**:
```php
$hookData = [
    'username' => 'john',                // Username
    'ip_address' => '192.168.1.100',     // Client IP
];
```

**Use cases**:
- Login notifications
- Security monitoring
- Session tracking
- Two-factor authentication triggers

---

### onLogout

Triggered when a user logs out.

**When**: When the logout action is performed.

**Data provided**:
```php
$hookData = [
    'username' => 'john',                // Username
];
```

---

## Hook Return Values

Hooks can return different values to control execution:

| Return Value | Behavior |
|--------------|----------|
| `true` or `null` | Success, continue to next hook |
| `false` | Stop hook chain execution |
| `array` | Custom result data (continues execution) |
| `['action' => 'stop']` | Stop hook chain execution |
| `['action' => 'continue']` | Continue to next hook |

**Example - Stopping execution**:
```php
<?php
// Prevent deletion of protected files
if (strpos($hookData['file_path'], '/protected/') === 0) {
    return false; // Stops further hooks
}

return true;
```

**Example - Returning data**:
```php
<?php
return [
    'action' => 'continue',
    'status' => 'processed',
    'scan_result' => 'clean',
    'processed_at' => date('Y-m-d H:i:s'),
];
```
