# Trend Micro File Scanning Hook

This directory contains the Trend Micro Vision One File Security scanning hook for FileGator.

## Official Trend Micro Documentation

- **File Security API**: https://automation.trendmicro.com/xdr/api-v3#tag/File-Security
- **Vision One File Security Overview**: https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security-intro-origin
- **SDK Documentation**: https://github.com/trendmicro/tm-v1-fs-nodejs-sdk
- **API Key Setup**: https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-api-keys

## Hook: 02_scan_upload.php

### Purpose

Automatically scans files uploaded to the `/upload` directory using Trend Micro Vision One File Security API. Clean files are moved to `/scanned` directory, while malware-infected files are deleted and an email alert is sent to administrators.

### Trigger Conditions

- **Event**: `onUpload`
- **Location**: Files uploaded to `/upload` directory only
- **Timing**: After file upload completes

### Workflow

```
File uploaded to /upload
         |
         v
[Trend Micro Scan Hook Triggered]
         |
         v
[Load Configuration]
         |
         v
[Check if enabled & API key configured]
         |
    YES  |  NO
         |  |
         |  +----> Skip (return)
         v
[Validate file path & size]
         |
         v
[Call Trend Micro API]
         |
    +----+----+
    |         |
  CLEAN    MALWARE
    |         |
    v         v
 Move to   Delete &
 /scanned  Email Admin
    |         |
    v         v
 SUCCESS   BLOCKED
```

## Configuration

Configuration is loaded from `/private/hooks/config.php` under the `trend_micro` key.

### Required Settings

```php
'trend_micro' => [
    'enabled' => true,
    'api_key' => getenv('TREND_MICRO_API_KEY') ?: '',
    'region' => 'us',
],
```

### Environment Variables

Set these environment variables for secure credential management:

```bash
# Required
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_REGION=us

# Optional
TREND_MICRO_API_URL=https://custom-endpoint.com
ADMIN_EMAIL=admin@example.com
```

### Supported Regions

Determine your region from your Vision One portal URL (e.g., `portal.eu.xdr.trendmicro.com` = `eu`):

| Region | Portal URL | API Endpoint |
|--------|------------|--------------|
| `us` | `portal.xdr.trendmicro.com` | `api.xdr.trendmicro.com` |
| `eu` | `portal.eu.xdr.trendmicro.com` | `api.eu.xdr.trendmicro.com` |
| `jp` | `portal.jp.xdr.trendmicro.com` | `api.xdr.trendmicro.co.jp` |
| `sg` | `portal.sg.xdr.trendmicro.com` | `api.sg.xdr.trendmicro.com` |
| `au` | `portal.au.xdr.trendmicro.com` | `api.au.xdr.trendmicro.com` |
| `in` | `portal.in.xdr.trendmicro.com` | `api.in.xdr.trendmicro.com` |

> **Reference**: [Regional Domains Documentation](https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-regional-domains)

### Full Configuration Options

```php
'trend_micro' => [
    // Enable/disable scanning
    'enabled' => false,

    // API credentials
    'api_key' => getenv('TREND_MICRO_API_KEY') ?: '',
    'region' => getenv('TREND_MICRO_REGION') ?: 'us',
    'api_url' => getenv('TREND_MICRO_API_URL') ?: '',

    // Scan settings
    'scan_timeout' => 60,                    // seconds
    'max_file_size' => 100 * 1024 * 1024,   // 100MB
    'skip_extensions' => [],                 // extensions to skip

    // Logging
    'log_file' => __DIR__ . '/../logs/trend_micro.log',
    'malware_log' => __DIR__ . '/../logs/malware_detections.log',

    // Error handling
    'on_error' => [
        'action' => 'continue',              // 'continue', 'quarantine', 'delete'
        'quarantine_dir' => __DIR__ . '/../quarantine',
    ],
],
```

## Installation

### 1. Copy Hook File

Copy `02_scan_upload.php` to your FileGator hooks directory:

```bash
cp 02_scan_upload.php /path/to/filegator/private/hooks/onUpload/
chmod 644 /path/to/filegator/private/hooks/onUpload/02_scan_upload.php
```

### 2. Update Configuration

Add the Trend Micro configuration section to `/private/hooks/config.php` (see Configuration section above).

### 3. Set Environment Variables

Configure environment variables in your web server or PHP-FPM configuration:

**Apache (.htaccess or httpd.conf)**:
```apache
SetEnv TREND_MICRO_API_KEY "your-api-key-here"
SetEnv TREND_MICRO_REGION "us"
SetEnv ADMIN_EMAIL "admin@example.com"
```

**Nginx (fastcgi_params)**:
```nginx
fastcgi_param TREND_MICRO_API_KEY "your-api-key-here";
fastcgi_param TREND_MICRO_REGION "us";
fastcgi_param ADMIN_EMAIL "admin@example.com";
```

**PHP-FPM (pool.d/www.conf)**:
```ini
env[TREND_MICRO_API_KEY] = "your-api-key-here"
env[TREND_MICRO_REGION] = "us"
env[ADMIN_EMAIL] = "admin@example.com"
```

### 4. Create Required Directories

```bash
mkdir -p /path/to/filegator/private/logs
mkdir -p /path/to/filegator/private/quarantine
chmod 755 /path/to/filegator/private/logs
chmod 700 /path/to/filegator/private/quarantine
```

### 5. Enable the Hook

Update `/private/hooks/config.php`:

```php
'trend_micro' => [
    'enabled' => true,  // Set to true
    // ... rest of configuration
],
```

## API Integration

### Authentication

The hook uses API key authentication with the Trend Micro Vision One File Security API.

> **Official Documentation**: For complete API details, see the [Trend Micro File Security API Reference](https://automation.trendmicro.com/xdr/api-v3#tag/File-Security).

**How to obtain API key:**
1. Log into [Trend Vision One console](https://portal.xdr.trendmicro.com/)
2. Navigate to **Administration** > **API Keys**
3. Click **Add API Key**
4. Select role with "Run file scan via SDK" permission
5. Set an expiry time (recommended: 1 year)
6. Copy the generated API key

> **Important**: Match your API key region to the `--region` parameter when installing.

### API Endpoint Format

The API URL is constructed based on the selected Vision One region:

```
https://api.{region}.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file
```

**Example endpoints by region:**
| Region | Endpoint |
|--------|----------|
| `us` | `https://api.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file` |
| `eu` | `https://api.eu.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file` |
| `jp` | `https://api.xdr.trendmicro.co.jp/v3.0/sandbox/fileSecurity/file` |
| `sg` | `https://api.sg.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file` |

### API Request Format

```http
POST https://api.{region}.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file
Authorization: Bearer {your-api-key}
Content-Type: application/octet-stream
Content-Length: {file-size}

{binary file content}
```

### API Response Format

**Clean File:**
```json
{
  "scanResult": 0,
  "scanId": "uuid",
  "foundMalwares": [],
  "fileSHA256": "hash"
}
```

**Malware Detected:**
```json
{
  "scanResult": 1,
  "scanId": "uuid",
  "foundMalwares": [
    {
      "fileName": "infected.exe",
      "malwareName": "Trojan.Win32.Generic"
    }
  ],
  "fileSHA256": "hash"
}
```

### Error Handling

The hook handles the following error scenarios:

- **No API Key**: Returns error, skips scan
- **File Too Large**: Skips scan, logs warning
- **API Timeout**: Returns error, continues with upload
- **Rate Limit (429)**: Returns error with retry message
- **Authentication Error (401/403)**: Returns error about invalid API key
- **Connection Error**: Returns error, continues with upload

## Email Notifications

When malware is detected, an email notification is sent to administrators.

### Email Configuration

Configure email settings in `/private/hooks/config.php`:

```php
'notifications' => [
    'email' => [
        'enabled' => true,

        'smtp' => [
            'host' => getenv('SMTP_HOST') ?: 'localhost',
            'port' => getenv('SMTP_PORT') ?: 587,
            'username' => getenv('SMTP_USER') ?: '',
            'password' => getenv('SMTP_PASS') ?: '',
            'encryption' => 'tls',
        ],

        'from' => [
            'address' => 'filegator@example.com',
            'name' => 'FileGator Security',
        ],

        'security_recipients' => [
            'admin@example.com',
        ],
    ],
],
```

### Email Content

The malware alert email includes:
- File name and size
- Username who uploaded the file
- Timestamp of detection
- Scan ID from Trend Micro
- File SHA256 hash
- List of detected threats
- Action taken (file deleted)

### Email Sending Methods

The hook supports two email sending methods:

1. **PHPMailer** (recommended): If PHPMailer class is available
2. **PHP mail()**: Fallback to native PHP mail() function

## Logging

### Log Files

The hook writes to multiple log files:

1. **Main Log** (`/private/logs/trend_micro.log`):
   - Scan initiations
   - Scan completions
   - File movements
   - Errors and exceptions

2. **Malware Log** (`/private/logs/malware_detections.log`):
   - Malware detection events
   - Threat details
   - User information

### Log Format

```
[2025-12-09 10:30:45] SCAN_INITIATED: document.pdf (size: 2048576 bytes, user: john, path: /upload/document.pdf)
[2025-12-09 10:30:47] SCAN_COMPLETED: document.pdf - Result: clean, Malware: NO, ScanID: abc123
[2025-12-09 10:30:47] FILE_MOVED: /upload/document.pdf -> /scanned/document.pdf (clean scan)
```

### Log Rotation

Consider setting up log rotation to prevent log files from growing too large:

```bash
# /etc/logrotate.d/filegator-hooks
/path/to/filegator/private/logs/*.log {
    weekly
    rotate 12
    compress
    delaycompress
    missingok
    notifempty
}
```

## Testing

### Test with Clean File

1. Upload a clean file to `/upload` directory
2. Check `/private/logs/trend_micro.log` for scan events
3. Verify file is moved to `/scanned` directory

### Test with EICAR Test File

Use the EICAR test file to validate malware detection without actual malware:

```bash
# Create EICAR test file
echo 'X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*' > eicar.txt

# Upload to /upload directory
# The hook should:
# 1. Detect it as malware
# 2. Delete the file
# 3. Send email alert
# 4. Log to malware_detections.log
```

### Verify Logs

```bash
# Check main log
tail -f /path/to/filegator/private/logs/trend_micro.log

# Check malware detections
tail -f /path/to/filegator/private/logs/malware_detections.log
```

## Security Considerations

### API Key Security

- Store API key in environment variables, NOT in config files
- Set restrictive file permissions on config files (`chmod 600`)
- Rotate API keys regularly (every 3-6 months)
- Use different keys for development/staging/production
- Never commit API keys to version control

### File Path Validation

The hook validates all file paths to prevent directory traversal attacks:

```php
$fullPath = realpath($repositoryPath . $homeDir . $filePath);
if (!$fullPath || strpos($fullPath, realpath($repositoryPath)) !== 0) {
    // Reject invalid paths
}
```

### Error Handling

The hook uses defensive programming:
- Never exposes API keys in logs or error messages
- Catches all exceptions to prevent hook failures
- Returns structured responses for proper error handling
- Uses safe defaults when configuration is missing

### Rate Limiting

Trend Micro API has rate limits (60-second windows). The hook:
- Detects HTTP 429 responses
- Returns clear error messages
- Recommends implementing retry logic in production

## Troubleshooting

### Hook Not Triggering

**Check:**
1. Hook file is in correct location: `/private/hooks/onUpload/02_scan_upload.php`
2. File permissions are correct: `644`
3. Hooks are enabled in `configuration.php`
4. File is uploaded to `/upload` directory (not other directories)

### API Authentication Errors

**Symptoms:** HTTP 401/403 errors

**Solutions:**
1. Verify API key is correct
2. Check API key has "Run file scan via SDK" permission
3. Ensure API key matches the region
4. Check if API key has expired

### Files Not Moving to /scanned

**Check:**
1. `/scanned` directory exists and is writable
2. No file permission issues
3. No filename conflicts (hook handles this automatically)
4. Check logs for error messages

### Email Notifications Not Sending

**Check:**
1. Email notifications enabled in config
2. SMTP settings are correct
3. Admin email is configured
4. Check server error logs for mail() errors

### High API Rate Limits

**Solution:**
Implement queueing system:
1. Move files to temporary queue directory
2. Process files with rate limiting
3. Use background worker process

## Performance Considerations

### Synchronous vs Asynchronous

The hook runs synchronously by default, which means:
- User waits for scan to complete
- Upload appears to "hang" during scan
- Scan timeout = user wait time

**Recommendation for Production:**
Implement asynchronous scanning:
1. Move file to queue directory immediately
2. Return success to user
3. Background worker scans files
4. Files appear in `/scanned` when ready

### File Size Limits

Large files take longer to scan:
- 1MB file: ~2-5 seconds
- 10MB file: ~5-15 seconds
- 100MB file: ~20-60 seconds

**Recommendation:**
Set appropriate `max_file_size` and `scan_timeout` values based on your needs.

## Support and Resources

### Official Documentation

- [Trend Vision One File Security](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security-intro-origin)
- [API Documentation](https://automation.trendmicro.com/xdr/api-v3/)
- [SDK Documentation](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)

### FileGator Documentation

- [Hooks Documentation](../../hooks/README.md)
- [Configuration Guide](../../hooks/configuration.md)
- [Best Practices](../../hooks/best-practices.md)

## License

This hook implementation is provided as an example for FileGator users. Use and modify according to your needs.

## Contributing

Found a bug or have improvements? Please submit issues or pull requests to the FileGator repository.

---

**Last Updated:** 2025-12-09
**Version:** 1.0
**Author:** FileGator Community
