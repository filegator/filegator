# Antivirus Scan Hook Example

This example demonstrates how to scan uploaded files for viruses using various antivirus solutions.

## Overview

The antivirus hook launches a background process to scan uploaded files. This prevents blocking the upload response while still ensuring files are scanned.

## Configuration

Configure the scanner in `private/hooks/config.php`:

```php
'antivirus' => [
    'enabled' => true,
    'scanner' => 'clamav',  // Options: 'clamav', 'virustotal', 'custom'

    'clamav' => [
        'binary' => '/usr/bin/clamscan',
        'daemon_binary' => '/usr/bin/clamdscan',
        'use_daemon' => false,
        'auto_remove' => true,
        'quarantine_dir' => __DIR__ . '/../quarantine',
    ],

    'virustotal' => [
        'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: '',
        'wait_for_result' => false,
    ],

    'skip_extensions' => ['txt', 'md', 'json'],
    'max_file_size' => 100 * 1024 * 1024,
],
```

## Hook Script

Location: `private/hooks/onUpload/antivirus_scan.php`

```php
<?php
/**
 * Antivirus Scan Hook
 * Scans uploaded files for malware
 */

// Load configuration
$config = include dirname(__DIR__) . '/config.php';
$avConfig = $config['antivirus'] ?? [];

// Skip if disabled
if (!($avConfig['enabled'] ?? true)) {
    return ['status' => 'skipped', 'message' => 'AV disabled'];
}

// Build file path
$repoPath = dirname(__DIR__, 3) . '/repository';
$fullPath = realpath($repoPath . $hookData['home_dir'] . $hookData['file_path']);

// Validate path
if (!$fullPath || strpos($fullPath, realpath($repoPath)) !== 0) {
    return ['status' => 'error', 'message' => 'Invalid path'];
}

// Skip certain extensions
$ext = strtolower(pathinfo($fullPath, PATHINFO_EXTENSION));
if (in_array($ext, $avConfig['skip_extensions'] ?? [])) {
    return ['status' => 'skipped', 'message' => 'Extension skipped'];
}

// Launch scan based on configured scanner
$scanner = $avConfig['scanner'] ?? 'clamav';
$logFile = dirname(__DIR__, 2) . '/logs/antivirus.log';

switch ($scanner) {
    case 'clamav':
        $binary = $avConfig['clamav']['binary'] ?? '/usr/bin/clamscan';
        $quarantine = $avConfig['clamav']['quarantine_dir'] ?? '';
        $moveFlag = $quarantine ? "--move=" . escapeshellarg($quarantine) : '--remove=yes';

        $cmd = sprintf(
            'nohup %s %s --quiet %s >> %s 2>&1 &',
            escapeshellarg($binary),
            $moveFlag,
            escapeshellarg($fullPath),
            escapeshellarg($logFile)
        );
        exec($cmd);
        break;

    // Add other scanner implementations...
}

return [
    'status' => 'scan_initiated',
    'scanner' => $scanner,
    'file' => basename($fullPath),
];
```

## Background Worker Script

Location: `private/hooks/onUpload/scan_worker.sh`

```bash
#!/bin/bash
FILE_PATH="$1"
USER="$2"
LOG_FILE="$(dirname "$0")/../../logs/antivirus.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

log "Scanning: $FILE_PATH (user: $USER)"

# Run ClamAV scan
if command -v clamscan &> /dev/null; then
    RESULT=$(clamscan --no-summary "$FILE_PATH" 2>&1)
    STATUS=$?

    if [ $STATUS -eq 1 ]; then
        log "INFECTED: $FILE_PATH"
        rm -f "$FILE_PATH"
        log "DELETED: $FILE_PATH"
    elif [ $STATUS -eq 0 ]; then
        log "CLEAN: $FILE_PATH"
    else
        log "ERROR: $RESULT"
    fi
fi
```

## Flow Diagram

```
User Upload
    │
    ▼
┌─────────────────┐
│ Upload Complete │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Trigger onUpload│
│ Hook            │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────┐
│ Launch Background├────►│ Scan Process     │
│ Scan Process    │     │ (runs async)     │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐     ┌──────────────────┐
│ Return Response │     │ Check Results    │
│ to User         │     │ Delete if Bad    │
└─────────────────┘     └──────────────────┘
```

## ClamAV Installation

### Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install clamav clamav-daemon
sudo freshclam  # Update virus definitions
```

### CentOS/RHEL
```bash
sudo yum install clamav clamav-update
sudo freshclam
```

### Using ClamAV Daemon (faster)

For better performance with many uploads, use the ClamAV daemon:

```bash
# Start daemon
sudo systemctl start clamav-daemon

# Configure to use daemon
'clamav' => [
    'use_daemon' => true,
    'daemon_binary' => '/usr/bin/clamdscan',
],
```

## VirusTotal Integration

For VirusTotal API scanning:

1. Get API key from https://www.virustotal.com/gui/my-apikey
2. Set environment variable: `VIRUSTOTAL_API_KEY=your-key-here`
3. Configure:

```php
'antivirus' => [
    'scanner' => 'virustotal',
    'virustotal' => [
        'api_key' => getenv('VIRUSTOTAL_API_KEY'),
    ],
],
```

## Testing

1. Upload a test file
2. Check `private/logs/antivirus.log`
3. Use EICAR test file for malware detection testing:
   - Download from https://www.eicar.org/download-anti-malware-testfile/
   - Upload to verify scan works

## Handling Scan Results

The background process handles results independently:

- **Clean file**: No action, file remains
- **Infected file**: Deleted or moved to quarantine
- **Scan error**: Logged for review

To notify users of scan results, implement a separate notification system or check the log file.
