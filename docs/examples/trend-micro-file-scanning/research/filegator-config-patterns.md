# FileGator Configuration Patterns Research

## Executive Summary

FileGator follows a secure, two-tier configuration approach:
1. **Main configuration**: `configuration.php` (root level) - Application-wide settings
2. **Hook configuration**: `private/hooks/config.php` - Hook-specific settings, API keys, and credentials

Sensitive data like API keys should be stored in the `private/hooks/config.php` file using environment variables where possible, with file permissions set to restrict access.

---

## 1. Configuration Storage Locations

### Primary Configuration Files

| File | Location | Purpose | Permission |
|------|----------|---------|------------|
| `configuration.php` | `/mnt/ai/filegator/configuration.php` | Main application config | 644 (readable) |
| `hooks/config.php` | `/mnt/ai/filegator/private/hooks/config.php` | Hook-specific settings, API keys | 644 (should be 600) |
| `users.json` | `/mnt/ai/filegator/private/users.json` | User accounts | 644 (should be 600) |
| `acl_config.php` | `/mnt/ai/filegator/private/acl_config.php` | Access control rules | 600 (secure) |

### Directory Structure

```
/mnt/ai/filegator/
├── configuration.php           # Main config (app-wide settings)
├── private/                    # Protected directory
│   ├── .htaccess              # "deny from all" - blocks web access
│   ├── .gitignore             # /tmp and /users.json excluded from git
│   ├── hooks/
│   │   ├── config.php         # Hook settings & API keys
│   │   └── onUpload/
│   │       ├── antivirus_scan.php
│   │       └── email_notification.php
│   ├── logs/                  # Log files
│   │   ├── app.log
│   │   ├── hooks.log
│   │   ├── antivirus.log
│   │   └── email_notifications.log
│   ├── sessions/              # PHP sessions
│   ├── tmp/                   # Temporary files
│   └── users.json             # User credentials
└── repository/                # File storage
```

---

## 2. Security Architecture

### Web Access Protection

The `private/` directory is protected by `.htaccess`:

```apache
deny from all
```

This prevents direct web access to:
- Configuration files
- User credentials
- Session data
- Log files
- Hook scripts

### File Permissions

**Recommended permissions:**

```bash
# Main config (needs web server read)
chmod 644 configuration.php

# Sensitive configs (only web server user)
chmod 600 private/hooks/config.php
chmod 600 private/users.json
chmod 600 private/acl_config.php

# Directories
chmod 755 private/
chmod 755 private/hooks/
chmod 755 private/logs/

# Hook scripts (executable by web server)
chmod 640 private/hooks/onUpload/*.php
```

**Current permissions observed:**
- `private/acl_config.php`: `600` (secure) ✓
- `private/hooks/config.php`: `644` (readable by all, should be 600) ⚠️

---

## 3. Loading Configuration in Hook Scripts

### Standard Pattern

All existing hooks follow this pattern:

```php
<?php
// Load configuration from parent directory
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Access specific settings with defaults
$avConfig = $config['antivirus'] ?? [];
$globalConfig = $config['global'] ?? [];
```

### Example from `antivirus_scan.php` (lines 30-36):

```php
// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get antivirus settings with defaults
$avConfig = $config['antivirus'] ?? [];
$globalConfig = $config['global'] ?? [];
```

### Example from `email_notification.php` (lines 26-31):

```php
// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get email notification settings
$notifyConfig = $config['notifications']['email'] ?? [];
```

### Path Resolution

Hook scripts are located at: `/mnt/ai/filegator/private/hooks/onUpload/script.php`

Using `dirname(__DIR__)` resolves to: `/mnt/ai/filegator/private/hooks/`

Therefore `config.php` path: `/mnt/ai/filegator/private/hooks/config.php`

---

## 4. Environment Variable Support

### Built-in Support

FileGator's hook configuration uses PHP's `getenv()` for environment variables with fallback defaults:

```php
// From private/hooks/config.php (lines 12-14)
/**
 * Environment variables can be used for sensitive data:
 *
 *   'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: 'default_key',
 */
```

### Examples from Existing Configuration

#### VirusTotal API Key (line 69):
```php
'virustotal' => [
    'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: '',
    'api_url' => 'https://www.virustotal.com/api/v3',
],
```

#### SMTP Credentials (lines 139-142):
```php
'smtp' => [
    'host' => getenv('SMTP_HOST') ?: 'localhost',
    'port' => getenv('SMTP_PORT') ?: 587,
    'username' => getenv('SMTP_USER') ?: '',
    'password' => getenv('SMTP_PASS') ?: '',
    'encryption' => 'tls',
],
```

#### AWS Credentials (lines 201-204):
```php
's3' => [
    'enabled' => false,
    'key' => getenv('AWS_ACCESS_KEY_ID') ?: '',
    'secret' => getenv('AWS_SECRET_ACCESS_KEY') ?: '',
    'region' => getenv('AWS_REGION') ?: 'us-east-1',
    'bucket' => getenv('AWS_S3_BUCKET') ?: '',
],
```

#### Webhook URLs (lines 166, 174):
```php
'slack' => [
    'enabled' => false,
    'webhook_url' => getenv('SLACK_WEBHOOK_URL') ?: '',
],

'discord' => [
    'enabled' => false,
    'webhook_url' => getenv('DISCORD_WEBHOOK_URL') ?: '',
],
```

### Setting Environment Variables

#### Apache (.htaccess or httpd.conf):
```apache
SetEnv VIRUSTOTAL_API_KEY "your-api-key-here"
SetEnv SMTP_HOST "smtp.example.com"
SetEnv TREND_MICRO_API_KEY "your-trend-key"
SetEnv TREND_MICRO_REGION "us-1"
```

#### Nginx (fastcgi_params):
```nginx
fastcgi_param VIRUSTOTAL_API_KEY "your-api-key-here";
fastcgi_param SMTP_HOST "smtp.example.com";
fastcgi_param TREND_MICRO_API_KEY "your-trend-key";
```

#### PHP-FPM (pool.d/www.conf):
```ini
env[VIRUSTOTAL_API_KEY] = "your-api-key-here"
env[SMTP_HOST] = "smtp.example.com"
env[TREND_MICRO_API_KEY] = "your-trend-key"
env[TREND_MICRO_REGION] = "us-1"
```

#### Docker (.env file):
```env
VIRUSTOTAL_API_KEY=your-api-key-here
SMTP_HOST=smtp.example.com
TREND_MICRO_API_KEY=your-trend-key
TREND_MICRO_REGION=us-1
```

---

## 5. Configuration Structure for External Services

### Standard Pattern

FileGator uses a consistent pattern for external service configuration:

```php
'services' => [
    'service_name' => [
        'enabled' => false,              // Feature flag
        'api_key' => getenv('API_KEY') ?: '',  // From environment
        'base_url' => 'https://api.service.com',
        // Additional service-specific settings
    ],
],
```

### Example: Custom API Integration (lines 217-219):
```php
'custom_api' => [
    'enabled' => false,
    'base_url' => '',
    'api_key' => getenv('CUSTOM_API_KEY') ?: '',
],
```

---

## 6. How Existing Hooks Handle Sensitive Data

### Pattern 1: Check if Enabled

All hooks check if their feature is enabled before processing:

```php
// From email_notification.php (lines 33-40)
if (!($notifyConfig['enabled'] ?? false)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Email notifications are disabled',
    ];
}
```

### Pattern 2: Validate Required Configuration

Hooks validate that required sensitive data is present:

```php
// From antivirus_scan.php (lines 143-149)
$apiKey = $vtConfig['api_key'] ?? '';

if (empty($apiKey)) {
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'VirusTotal API key not configured in hooks/config.php',
    ];
}
```

### Pattern 3: Safe Defaults

Configuration uses safe defaults (empty strings, false) when environment variables are not set:

```php
'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: '',
// If not set, defaults to empty string
// Hook will detect this and return error/skip
```

### Pattern 4: Logging Without Exposing Secrets

Hooks log activity without exposing sensitive credentials:

```php
// From antivirus_scan.php (lines 95-103)
$logMessage = sprintf(
    "[%s] Initiating antivirus scan for: %s (uploaded by: %s, size: %d bytes)\n",
    date('Y-m-d H:i:s'),
    $fullPath,
    $hookData['user'],
    $hookData['file_size']
);
@file_put_contents($avLogFile, $logMessage, FILE_APPEND);
// Note: No API keys logged
```

---

## 7. Hook Configuration Best Practices

Based on documentation in `docs/hooks/best-practices.md`:

### Keep Hooks Lightweight

Hooks run synchronously. Heavy operations (API calls, scans) should be:
- Launched in background processes
- Use asynchronous execution
- Return immediately to avoid blocking uploads

### Use Timeouts for External Services

When calling external APIs:

```php
$ctx = stream_context_create([
    'http' => [
        'timeout' => 5, // 5 second timeout
    ]
]);

$response = @file_get_contents($apiUrl, false, $ctx);
```

### Handle Errors Gracefully

Never let hook errors crash the application:

```php
try {
    // API call or processing
} catch (\Exception $e) {
    error_log("Hook error: " . $e->getMessage());
    return [
        'action' => 'continue',
        'status' => 'error',
        'error' => $e->getMessage(),
    ];
}
```

### Validate All Input

Sanitize and validate hook data:

```php
// Validate required fields
if (empty($hookData['file_path']) || empty($hookData['user'])) {
    return ['status' => 'error', 'message' => 'Missing required data'];
}

// Sanitize paths
$filePath = realpath($repositoryPath . $hookData['file_path']);
if (!$filePath || strpos($filePath, $repositoryPath) !== 0) {
    return ['status' => 'error', 'message' => 'Invalid file path'];
}
```

---

## 8. Recommended Approach for Trend Micro Configuration

Based on FileGator patterns, here's the recommended approach:

### Add to `private/hooks/config.php`

```php
/*
|--------------------------------------------------------------------------
| Trend Micro Vision One File Security
|--------------------------------------------------------------------------
|
| Configuration for Trend Micro File Security scanning
|
*/
'trend_micro' => [
    // Enable/disable Trend Micro scanning
    'enabled' => false,

    // API credentials (use environment variables)
    'api_key' => getenv('TREND_MICRO_API_KEY') ?: '',

    // Region: 'us-1', 'eu-1', 'ap-1', 'au-1', 'in-1', 'sg-1', 'jp-1'
    'region' => getenv('TREND_MICRO_REGION') ?: 'us-1',

    // API endpoint (auto-configured based on region if empty)
    'api_url' => getenv('TREND_MICRO_API_URL') ?: '',

    // Scanning behavior
    'scan_timeout' => 30, // seconds
    'async_scan' => true,  // Use async scanning
    'wait_for_result' => false, // Return immediately, check later

    // File handling
    'max_file_size' => 100 * 1024 * 1024, // 100MB
    'skip_extensions' => ['txt', 'md', 'json'],

    // Actions on detection
    'quarantine_infected' => true,
    'quarantine_dir' => __DIR__ . '/../quarantine',
    'delete_infected' => false, // Quarantine instead of delete

    // Logging
    'log_file' => __DIR__ . '/../logs/trend_micro.log',
    'log_level' => 'info', // 'debug', 'info', 'warning', 'error'
],
```

### Load in Hook Script

```php
<?php
// In private/hooks/onUpload/02_scan_upload.php

// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get Trend Micro settings
$tmConfig = $config['trend_micro'] ?? [];

// Check if enabled
if (!($tmConfig['enabled'] ?? false)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Trend Micro scanning is disabled',
    ];
}

// Validate API key
$apiKey = $tmConfig['api_key'] ?? '';
if (empty($apiKey)) {
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Trend Micro API key not configured',
    ];
}

// Continue with scanning logic...
```

---

## 9. Git Security

### Files Excluded from Git

From `private/.gitignore`:

```gitignore
/tmp
/users.json
```

This ensures:
- Temporary files are not committed
- User credentials are not stored in repository
- Only example/template files are versioned

### Files Safe to Commit

- `configuration.php` (without secrets)
- `configuration_sample.php` (template)
- `private/hooks/config.php` (with environment variable placeholders)
- Hook scripts (`.php` files in `private/hooks/onUpload/`)

**Never commit:**
- Actual API keys or passwords (use environment variables)
- `private/users.json` (actual user accounts)
- Log files
- Session data
- Temporary files

---

## 10. Summary & Recommendations

### Where to Store Trend Micro Configuration

✅ **Store in**: `/mnt/ai/filegator/private/hooks/config.php`

Add a new `'trend_micro'` section following the existing patterns for `'antivirus'`, `'notifications'`, and `'services'`.

### How to Load Configuration

```php
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];
$tmConfig = $config['trend_micro'] ?? [];
```

### Environment Variables

```bash
# Required
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_REGION=us-1

# Optional (override defaults)
TREND_MICRO_API_URL=https://custom-endpoint.com
```

### File Permissions

```bash
# Set secure permissions
chmod 600 /mnt/ai/filegator/private/hooks/config.php
chmod 640 /mnt/ai/filegator/private/hooks/onUpload/*.php
chmod 755 /mnt/ai/filegator/private/hooks/
```

### Security Checklist

- ✅ Store API keys in environment variables
- ✅ Use `getenv()` with safe defaults
- ✅ Validate configuration before use
- ✅ Log activity without exposing secrets
- ✅ Set restrictive file permissions
- ✅ Keep config files out of webroot (.htaccess deny)
- ✅ Never commit actual credentials to git
- ✅ Handle API timeouts gracefully
- ✅ Validate all input data
- ✅ Return structured error responses

---

## References

- Main configuration: `/mnt/ai/filegator/configuration.php`
- Sample configuration: `/mnt/ai/filegator/configuration_sample.php`
- Hook configuration: `/mnt/ai/filegator/private/hooks/config.php`
- Configuration docs: `/mnt/ai/filegator/docs/hooks/configuration.md`
- Best practices: `/mnt/ai/filegator/docs/hooks/best-practices.md`
- Email example: `/mnt/ai/filegator/docs/hooks/examples/email-notification.md`
- Antivirus example: `/mnt/ai/filegator/private/hooks/onUpload/antivirus_scan.php`
- Email example: `/mnt/ai/filegator/private/hooks/onUpload/email_notification.php`
