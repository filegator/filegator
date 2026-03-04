# Hook Configuration

Hooks are configured through two files:

1. **`configuration.php`** - Main FileGator config (enables hooks service)
2. **`private/hooks/config.php`** - Hook-specific settings (API keys, destinations, etc.)

## Main Configuration

Enable the hooks service in your `configuration.php`:

```php
'services' => [
    // ... other services ...

    'Filegator\Services\Hooks\HooksInterface' => [
        'handler' => '\Filegator\Services\Hooks\Hooks',
        'config' => [
            'enabled' => true,                        // Enable/disable hooks globally
            'hooks_path' => __DIR__.'/private/hooks', // Path to hooks directory
            'timeout' => 30,                          // Max execution time (seconds)
            'async' => false,                         // Reserved for future use
        ],
    ],
],
```

## Hook Configuration File

The `private/hooks/config.php` file contains all settings that hook scripts can use. This is where you configure API keys, destinations, and other parameters.

### File Location

```
private/
└── hooks/
    ├── config.php      <-- Configuration file
    ├── onUpload/
    │   └── ...
    └── onDelete/
        └── ...
```

### Loading Configuration in Hooks

Hook scripts can load configuration like this:

```php
<?php
// In your hook script
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Access specific settings
$avConfig = $config['antivirus'] ?? [];
$emailRecipients = $config['notifications']['email']['upload_recipients'] ?? [];
```

### Configuration Sections

#### Global Settings

```php
'global' => [
    'debug' => false,                              // Enable verbose logging
    'log_file' => __DIR__ . '/../logs/hooks.log', // Hook activity log
    'http_timeout' => 10,                          // Default HTTP timeout
],
```

#### Antivirus Settings

```php
'antivirus' => [
    'enabled' => true,
    'scanner' => 'clamav',  // 'clamav', 'virustotal', 'custom'

    'clamav' => [
        'binary' => '/usr/bin/clamscan',
        'daemon_binary' => '/usr/bin/clamdscan',
        'use_daemon' => false,
        'auto_remove' => true,
        'quarantine_dir' => __DIR__ . '/../quarantine',
    ],

    'virustotal' => [
        'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: '',
        'api_url' => 'https://www.virustotal.com/api/v3',
        'wait_for_result' => false,
    ],

    'custom' => [
        'command' => '/path/to/scanner --scan {file_path} >> {log_file} 2>&1',
    ],

    'skip_extensions' => ['txt', 'md', 'json'],
    'max_file_size' => 100 * 1024 * 1024,  // 100MB
],
```

#### Notification Settings

```php
'notifications' => [
    'email' => [
        'enabled' => false,
        'smtp' => [
            'host' => getenv('SMTP_HOST') ?: 'localhost',
            'port' => getenv('SMTP_PORT') ?: 587,
            'username' => getenv('SMTP_USER') ?: '',
            'password' => getenv('SMTP_PASS') ?: '',
            'encryption' => 'tls',
        ],
        'from' => [
            'address' => 'filegator@example.com',
            'name' => 'FileGator',
        ],
        'upload_recipients' => ['admin@example.com'],
        'security_recipients' => ['security@example.com'],
    ],

    'slack' => [
        'enabled' => false,
        'webhook_url' => getenv('SLACK_WEBHOOK_URL') ?: '',
        'channel' => '#filegator-uploads',
    ],

    'discord' => [
        'enabled' => false,
        'webhook_url' => getenv('DISCORD_WEBHOOK_URL') ?: '',
    ],

    'webhook' => [
        'enabled' => false,
        'url' => 'https://your-service.com/webhook',
        'method' => 'POST',
        'headers' => [
            'Content-Type' => 'application/json',
            'Authorization' => 'Bearer YOUR_TOKEN',
        ],
    ],
],
```

#### External Services

```php
'services' => [
    's3' => [
        'enabled' => false,
        'key' => getenv('AWS_ACCESS_KEY_ID') ?: '',
        'secret' => getenv('AWS_SECRET_ACCESS_KEY') ?: '',
        'region' => getenv('AWS_REGION') ?: 'us-east-1',
        'bucket' => getenv('AWS_S3_BUCKET') ?: '',
    ],

    'custom_api' => [
        'enabled' => false,
        'base_url' => 'https://api.your-service.com',
        'api_key' => getenv('CUSTOM_API_KEY') ?: '',
    ],
],
```

#### File Validation

```php
'validation' => [
    'allowed_extensions' => [],  // Empty = all allowed
    'blocked_extensions' => [
        'exe', 'bat', 'cmd', 'sh', 'php', 'phar',
        'jar', 'vbs', 'ps1', 'dll', 'msi',
    ],
    'max_size_by_extension' => [
        'default' => 100 * 1024 * 1024,
        'jpg' => 20 * 1024 * 1024,
        'pdf' => 50 * 1024 * 1024,
    ],
    'verify_mime' => true,
],
```

#### Audit Logging

```php
'audit' => [
    'enabled' => true,
    'log_file' => __DIR__ . '/../logs/audit.log',
    'events' => [
        'upload' => true,
        'download' => true,
        'delete' => true,
    ],
    'log_ip' => true,
],
```

## Using Environment Variables

For security, sensitive data like API keys should be stored in environment variables:

```php
// In config.php
'api_key' => getenv('MY_API_KEY') ?: 'default_value',
```

Set environment variables in your server configuration:

### Apache (.htaccess or httpd.conf)
```apache
SetEnv VIRUSTOTAL_API_KEY "your-api-key-here"
SetEnv SMTP_HOST "smtp.example.com"
```

### Nginx (fastcgi_params)
```nginx
fastcgi_param VIRUSTOTAL_API_KEY "your-api-key-here";
fastcgi_param SMTP_HOST "smtp.example.com";
```

### PHP-FPM (pool.d/www.conf)
```ini
env[VIRUSTOTAL_API_KEY] = "your-api-key-here"
env[SMTP_HOST] = "smtp.example.com"
```

### .env file (with php-dotenv)
```
VIRUSTOTAL_API_KEY=your-api-key-here
SMTP_HOST=smtp.example.com
```

## Complete Example Configuration

See the default `private/hooks/config.php` for a complete example with all available options.

## Accessing Configuration in Hook Scripts

```php
<?php
/**
 * Example: Using configuration in a hook
 */

// Load config
$config = include dirname(__DIR__) . '/config.php';

// Check if feature is enabled
if (!($config['notifications']['email']['enabled'] ?? false)) {
    return ['status' => 'notifications_disabled'];
}

// Get SMTP settings
$smtp = $config['notifications']['email']['smtp'];
$recipients = $config['notifications']['email']['upload_recipients'];

// Use settings
$mailer = new Mailer([
    'host' => $smtp['host'],
    'port' => $smtp['port'],
    'username' => $smtp['username'],
    'password' => $smtp['password'],
]);

foreach ($recipients as $email) {
    $mailer->send($email, 'New Upload', 'File uploaded: ' . $hookData['file_name']);
}

return ['status' => 'notified', 'recipients' => count($recipients)];
```

## Configuration Caching

For performance, you can cache the parsed configuration:

```php
<?php
// Use APCu or similar for caching
$cacheKey = 'filegator_hooks_config';

if (function_exists('apcu_fetch') && $config = apcu_fetch($cacheKey)) {
    // Use cached config
} else {
    $config = include dirname(__DIR__) . '/config.php';
    if (function_exists('apcu_store')) {
        apcu_store($cacheKey, $config, 300); // Cache for 5 minutes
    }
}
```
