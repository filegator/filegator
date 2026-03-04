<?php

/**
 * Hook Configuration File
 *
 * This file contains configuration settings for all hook scripts.
 * Hook scripts can load these settings using:
 *
 *   $config = include __DIR__ . '/config.php';
 *   $apiKey = $config['antivirus']['api_key'];
 *
 * Environment variables can be used for sensitive data:
 *
 *   'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: 'default_key',
 */

return [

    /*
    |--------------------------------------------------------------------------
    | Global Settings
    |--------------------------------------------------------------------------
    |
    | Settings that apply to all hooks
    |
    */
    'global' => [
        // Enable verbose logging for hooks
        'debug' => false,

        // Log file path for hook activity
        'log_file' => __DIR__ . '/../logs/hooks.log',

        // Default timeout for external HTTP requests (seconds)
        'http_timeout' => 10,
    ],

    /*
    |--------------------------------------------------------------------------
    | Antivirus / Security Scanning
    |--------------------------------------------------------------------------
    |
    | Configuration for virus scanning hooks
    |
    */
    'antivirus' => [
        // Enable/disable antivirus scanning
        'enabled' => true,

        // Scanner type: 'clamav', 'virustotal', 'custom'
        'scanner' => 'clamav',

        // ClamAV settings
        'clamav' => [
            // Path to clamscan binary
            'binary' => '/usr/bin/clamscan',
            // Use daemon (faster): 'clamdscan'
            'daemon_binary' => '/usr/bin/clamdscan',
            // Use daemon mode
            'use_daemon' => false,
            // Remove infected files automatically
            'auto_remove' => true,
            // Quarantine directory (instead of delete)
            'quarantine_dir' => __DIR__ . '/../quarantine',
        ],

        // VirusTotal API settings
        'virustotal' => [
            'api_key' => getenv('VIRUSTOTAL_API_KEY') ?: '',
            'api_url' => 'https://www.virustotal.com/api/v3',
            // Wait for scan result or just submit
            'wait_for_result' => false,
        ],

        // Custom scanner command
        'custom' => [
            // Command template. Placeholders: {file_path}, {log_file}
            'command' => '/path/to/scanner --scan {file_path} >> {log_file} 2>&1',
        ],

        // File types to skip scanning (by extension)
        'skip_extensions' => ['txt', 'md', 'json'],

        // Maximum file size to scan (bytes, 0 = no limit)
        'max_file_size' => 100 * 1024 * 1024, // 100MB
    ],

    /*
    |--------------------------------------------------------------------------
    | File Processing
    |--------------------------------------------------------------------------
    |
    | Configuration for file processing hooks (thumbnails, conversions, etc.)
    |
    */
    'processing' => [
        // Image thumbnail generation
        'thumbnails' => [
            'enabled' => false,
            'destination' => __DIR__ . '/../../repository/.thumbnails',
            'sizes' => [
                'small' => ['width' => 150, 'height' => 150],
                'medium' => ['width' => 300, 'height' => 300],
            ],
            'quality' => 85,
            'extensions' => ['jpg', 'jpeg', 'png', 'gif', 'webp'],
        ],

        // Document conversion
        'conversion' => [
            'enabled' => false,
            // LibreOffice path for document conversion
            'libreoffice_path' => '/usr/bin/libreoffice',
            'output_format' => 'pdf',
        ],

        // Move files after processing
        'post_upload_move' => [
            'enabled' => false,
            'destination' => '/verified',  // Relative to user's home
        ],
    ],

    /*
    |--------------------------------------------------------------------------
    | Notifications
    |--------------------------------------------------------------------------
    |
    | Configuration for notification hooks
    |
    */
    'notifications' => [
        // Email notifications
        'email' => [
            'enabled' => false,

            // SMTP settings
            'smtp' => [
                'host' => getenv('SMTP_HOST') ?: 'localhost',
                'port' => getenv('SMTP_PORT') ?: 587,
                'username' => getenv('SMTP_USER') ?: '',
                'password' => getenv('SMTP_PASS') ?: '',
                'encryption' => 'tls', // 'tls', 'ssl', or null
            ],

            // Sender
            'from' => [
                'address' => 'filegator@example.com',
                'name' => 'FileGator',
            ],

            // Recipients for upload notifications
            'upload_recipients' => [
                // 'admin@example.com',
            ],

            // Recipients for security alerts
            'security_recipients' => [
                // 'security@example.com',
            ],
        ],

        // Slack notifications
        'slack' => [
            'enabled' => false,
            'webhook_url' => getenv('SLACK_WEBHOOK_URL') ?: '',
            'channel' => '#filegator-uploads',
            'username' => 'FileGator Bot',
        ],

        // Discord notifications
        'discord' => [
            'enabled' => false,
            'webhook_url' => getenv('DISCORD_WEBHOOK_URL') ?: '',
        ],

        // Webhook notifications (generic)
        'webhook' => [
            'enabled' => false,
            'url' => '',
            'method' => 'POST',
            'headers' => [
                'Content-Type' => 'application/json',
                // 'Authorization' => 'Bearer YOUR_TOKEN',
            ],
        ],
    ],

    /*
    |--------------------------------------------------------------------------
    | External Services
    |--------------------------------------------------------------------------
    |
    | API keys and settings for external service integrations
    |
    */
    'services' => [
        // AWS S3 backup
        's3' => [
            'enabled' => false,
            'key' => getenv('AWS_ACCESS_KEY_ID') ?: '',
            'secret' => getenv('AWS_SECRET_ACCESS_KEY') ?: '',
            'region' => getenv('AWS_REGION') ?: 'us-east-1',
            'bucket' => getenv('AWS_S3_BUCKET') ?: '',
        ],

        // Google Cloud Storage
        'gcs' => [
            'enabled' => false,
            'credentials_file' => '',
            'bucket' => '',
        ],

        // Custom API integration
        'custom_api' => [
            'enabled' => false,
            'base_url' => '',
            'api_key' => getenv('CUSTOM_API_KEY') ?: '',
        ],
    ],

    /*
    |--------------------------------------------------------------------------
    | Logging & Auditing
    |--------------------------------------------------------------------------
    |
    | Configuration for audit logging hooks
    |
    */
    'audit' => [
        // Enable audit logging
        'enabled' => true,

        // Log file for audit trail
        'log_file' => __DIR__ . '/../logs/audit.log',

        // Events to log
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

        // Include user IP in logs
        'log_ip' => true,

        // Log to database (requires custom implementation)
        'database' => [
            'enabled' => false,
            'table' => 'audit_log',
        ],

        // Send to external logging service
        'external' => [
            'enabled' => false,
            'service' => '', // 'elasticsearch', 'splunk', 'datadog'
            'endpoint' => '',
            'api_key' => '',
        ],
    ],

    /*
    |--------------------------------------------------------------------------
    | File Validation
    |--------------------------------------------------------------------------
    |
    | Rules for validating uploaded files
    |
    */
    'validation' => [
        // Allowed file extensions (empty = all allowed)
        'allowed_extensions' => [],

        // Blocked file extensions
        'blocked_extensions' => [
            'exe', 'bat', 'cmd', 'sh', 'php', 'phar',
            'jar', 'vbs', 'ps1', 'dll', 'msi',
        ],

        // Maximum file size per extension (bytes)
        'max_size_by_extension' => [
            'default' => 100 * 1024 * 1024,  // 100MB
            'jpg' => 20 * 1024 * 1024,        // 20MB for images
            'png' => 20 * 1024 * 1024,
            'gif' => 10 * 1024 * 1024,
            'pdf' => 50 * 1024 * 1024,        // 50MB for PDFs
        ],

        // Check file content matches extension (basic MIME check)
        'verify_mime' => true,
    ],

    /*
    |--------------------------------------------------------------------------
    | Trend Micro Vision One File Security
    |--------------------------------------------------------------------------
    |
    | Configuration for Trend Micro Vision One File Security API integration.
    | Scans uploaded files for malware using Trend Micro's cloud scanning service.
    |
    | API Key: Obtain from Trend Vision One console under Administration > API Keys
    | Required Permission: "Run file scan via SDK"
    |
    */
    'trend_micro' => [
        // Enable/disable Trend Micro scanning
        'enabled' => false,

        // API credentials (use environment variables for security)
        'api_key' => getenv('TREND_MICRO_API_KEY') ?: '',

        // Region: 'us-east-1', 'eu-central-1', 'ap-northeast-1', 'ap-southeast-1',
        //         'ap-southeast-2', 'ap-south-1', 'me-central-1'
        'region' => getenv('TREND_MICRO_REGION') ?: 'us-east-1',

        // Custom API endpoint (optional, auto-configured from region if empty)
        'api_url' => getenv('TREND_MICRO_API_URL') ?: '',

        // Scan timeout in seconds (max time to wait for scan result)
        'scan_timeout' => 60,

        // Maximum file size to scan (bytes, default: 100MB)
        'max_file_size' => 100 * 1024 * 1024,

        // File extensions to skip scanning (empty array = scan all)
        'skip_extensions' => [],

        // Log file for Trend Micro scan activity
        'log_file' => __DIR__ . '/../logs/trend_micro.log',

        // Separate log file for malware detections
        'malware_log' => __DIR__ . '/../logs/malware_detections.log',

        // Action on scan errors
        'on_error' => [
            'action' => 'continue',  // 'continue' = allow upload, 'quarantine', 'delete'
            'quarantine_dir' => __DIR__ . '/../quarantine',
        ],
    ],

];
