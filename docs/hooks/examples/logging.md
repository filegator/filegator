# Audit Logging Hook Example

This example shows how to create comprehensive audit logs for compliance and security monitoring.

## Overview

Audit logging captures all file operations with user and timestamp information, useful for:
- Compliance requirements (GDPR, HIPAA, etc.)
- Security monitoring
- Usage analytics
- Troubleshooting

## Configuration

```php
// In private/hooks/config.php
'audit' => [
    'enabled' => true,
    'log_file' => __DIR__ . '/../logs/audit.log',
    'events' => [
        'upload' => true,
        'download' => true,
        'delete' => true,
        'create' => true,
        'rename' => true,
        'move' => true,
        'copy' => true,
        'login' => true,
        'logout' => true,
    ],
    'log_ip' => true,
    'format' => 'json',  // 'json' or 'text'
],
```

## Upload Audit Hook

Location: `private/hooks/onUpload/audit_log.php`

```php
<?php
/**
 * Audit Log Hook - Upload Events
 */

$config = include dirname(__DIR__) . '/config.php';
$auditConfig = $config['audit'] ?? [];

if (!($auditConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

if (!($auditConfig['events']['upload'] ?? true)) {
    return ['status' => 'event_disabled'];
}

$logFile = $auditConfig['log_file'] ?? dirname(__DIR__, 2) . '/logs/audit.log';
$format = $auditConfig['format'] ?? 'json';

// Build log entry
$entry = [
    'timestamp' => date('c'),
    'event' => 'upload',
    'user' => $hookData['user'],
    'file_name' => $hookData['file_name'],
    'file_path' => $hookData['file_path'],
    'file_size' => $hookData['file_size'],
    'home_dir' => $hookData['home_dir'],
];

// Add IP if configured
if ($auditConfig['log_ip'] ?? true) {
    $entry['ip_address'] = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
}

// Format and write
if ($format === 'json') {
    $line = json_encode($entry) . "\n";
} else {
    $line = sprintf(
        "[%s] UPLOAD user=%s file=%s path=%s size=%d\n",
        $entry['timestamp'],
        $entry['user'],
        $entry['file_name'],
        $entry['file_path'],
        $entry['file_size']
    );
}

file_put_contents($logFile, $line, FILE_APPEND | LOCK_EX);

return ['status' => 'logged', 'event' => 'upload'];
```

## Delete Audit Hook

Location: `private/hooks/onDelete/audit_log.php`

```php
<?php
/**
 * Audit Log Hook - Delete Events
 */

$config = include dirname(__DIR__) . '/config.php';
$auditConfig = $config['audit'] ?? [];

if (!($auditConfig['enabled'] ?? false) || !($auditConfig['events']['delete'] ?? true)) {
    return ['status' => 'disabled'];
}

$logFile = $auditConfig['log_file'] ?? dirname(__DIR__, 2) . '/logs/audit.log';

$entry = [
    'timestamp' => date('c'),
    'event' => 'delete',
    'user' => $hookData['user'],
    'file_name' => $hookData['file_name'],
    'file_path' => $hookData['file_path'],
    'type' => $hookData['type'],
];

if ($auditConfig['log_ip'] ?? true) {
    $entry['ip_address'] = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
}

file_put_contents($logFile, json_encode($entry) . "\n", FILE_APPEND | LOCK_EX);

return ['status' => 'logged', 'event' => 'delete'];
```

## Download Audit Hook

Location: `private/hooks/onDownload/audit_log.php`

```php
<?php
/**
 * Audit Log Hook - Download Events
 */

$config = include dirname(__DIR__) . '/config.php';
$auditConfig = $config['audit'] ?? [];

if (!($auditConfig['enabled'] ?? false) || !($auditConfig['events']['download'] ?? true)) {
    return ['status' => 'disabled'];
}

$logFile = $auditConfig['log_file'] ?? dirname(__DIR__, 2) . '/logs/audit.log';

$entry = [
    'timestamp' => date('c'),
    'event' => 'download',
    'user' => $hookData['user'],
    'file_name' => $hookData['file_name'],
    'file_path' => $hookData['file_path'],
    'ip_address' => $_SERVER['REMOTE_ADDR'] ?? 'unknown',
];

file_put_contents($logFile, json_encode($entry) . "\n", FILE_APPEND | LOCK_EX);

return ['status' => 'logged', 'event' => 'download'];
```

## Login Audit Hook

Location: `private/hooks/onLogin/audit_log.php`

```php
<?php
/**
 * Audit Log Hook - Login Events
 */

$config = include dirname(__DIR__) . '/config.php';
$auditConfig = $config['audit'] ?? [];

if (!($auditConfig['enabled'] ?? false) || !($auditConfig['events']['login'] ?? true)) {
    return ['status' => 'disabled'];
}

$logFile = $auditConfig['log_file'] ?? dirname(__DIR__, 2) . '/logs/audit.log';

$entry = [
    'timestamp' => date('c'),
    'event' => 'login',
    'username' => $hookData['username'],
    'ip_address' => $hookData['ip_address'],
    'user_agent' => $_SERVER['HTTP_USER_AGENT'] ?? 'unknown',
];

file_put_contents($logFile, json_encode($entry) . "\n", FILE_APPEND | LOCK_EX);

return ['status' => 'logged', 'event' => 'login'];
```

## Sample Log Output

### JSON Format
```json
{"timestamp":"2024-01-15T10:30:00+00:00","event":"upload","user":"john","file_name":"report.pdf","file_path":"/documents/report.pdf","file_size":1048576,"ip_address":"192.168.1.100"}
{"timestamp":"2024-01-15T10:31:00+00:00","event":"download","user":"jane","file_name":"report.pdf","file_path":"/documents/report.pdf","ip_address":"192.168.1.101"}
{"timestamp":"2024-01-15T10:32:00+00:00","event":"delete","user":"admin","file_name":"old.txt","file_path":"/temp/old.txt","type":"file","ip_address":"192.168.1.1"}
```

### Text Format
```
[2024-01-15T10:30:00+00:00] UPLOAD user=john file=report.pdf path=/documents/report.pdf size=1048576
[2024-01-15T10:31:00+00:00] DOWNLOAD user=jane file=report.pdf path=/documents/report.pdf
[2024-01-15T10:32:00+00:00] DELETE user=admin file=old.txt path=/temp/old.txt type=file
```

## Log Rotation

Set up log rotation to prevent log files from growing too large:

### logrotate configuration
Create `/etc/logrotate.d/filegator`:

```
/path/to/filegator/private/logs/audit.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 www-data www-data
}
```

## Sending Logs to External Services

### Elasticsearch

```php
<?php
// Send to Elasticsearch
$ch = curl_init('http://elasticsearch:9200/filegator-audit/_doc');
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode($entry),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
]);
curl_exec($ch);
curl_close($ch);
```

### Splunk

```php
<?php
// Send to Splunk HEC
$ch = curl_init('https://splunk:8088/services/collector/event');
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode(['event' => $entry]),
    CURLOPT_HTTPHEADER => [
        'Content-Type: application/json',
        'Authorization: Splunk ' . $config['audit']['splunk_token'],
    ],
    CURLOPT_RETURNTRANSFER => true,
]);
curl_exec($ch);
curl_close($ch);
```

## Querying Logs

### Using jq for JSON logs
```bash
# All uploads by user john
cat audit.log | jq 'select(.event=="upload" and .user=="john")'

# Downloads in last hour
cat audit.log | jq 'select(.event=="download")' | head -20

# Count events by type
cat audit.log | jq -r '.event' | sort | uniq -c
```

### Using grep for text logs
```bash
# Find all upload events
grep "UPLOAD" audit.log

# Find activity by user
grep "user=john" audit.log

# Find by date
grep "2024-01-15" audit.log
```
