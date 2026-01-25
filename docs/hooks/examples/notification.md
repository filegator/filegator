# Notification Hook Example

This example shows how to send notifications when files are uploaded.

## Overview

Notification hooks can alert administrators via email, Slack, Discord, or custom webhooks when events occur.

## Configuration

```php
// In private/hooks/config.php
'notifications' => [
    'email' => [
        'enabled' => true,
        'smtp' => [
            'host' => 'smtp.example.com',
            'port' => 587,
            'username' => 'user@example.com',
            'password' => 'secret',
            'encryption' => 'tls',
        ],
        'from' => [
            'address' => 'filegator@example.com',
            'name' => 'FileGator',
        ],
        'upload_recipients' => [
            'admin@example.com',
        ],
    ],

    'slack' => [
        'enabled' => true,
        'webhook_url' => 'https://hooks.slack.com/services/XXX/YYY/ZZZ',
        'channel' => '#uploads',
    ],

    'discord' => [
        'enabled' => false,
        'webhook_url' => 'https://discord.com/api/webhooks/XXX/YYY',
    ],
],
```

## Slack Notification Hook

Location: `private/hooks/onUpload/notify_slack.php`

```php
<?php
/**
 * Slack Notification Hook
 * Sends upload notifications to Slack
 */

$config = include dirname(__DIR__) . '/config.php';
$slackConfig = $config['notifications']['slack'] ?? [];

if (!($slackConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

$webhookUrl = $slackConfig['webhook_url'] ?? '';
if (empty($webhookUrl)) {
    return ['status' => 'error', 'message' => 'No webhook URL'];
}

// Build message
$message = [
    'channel' => $slackConfig['channel'] ?? '#general',
    'username' => 'FileGator Bot',
    'icon_emoji' => ':file_folder:',
    'attachments' => [
        [
            'color' => '#36a64f',
            'title' => 'New File Upload',
            'fields' => [
                [
                    'title' => 'File',
                    'value' => $hookData['file_name'],
                    'short' => true,
                ],
                [
                    'title' => 'Size',
                    'value' => formatBytes($hookData['file_size']),
                    'short' => true,
                ],
                [
                    'title' => 'User',
                    'value' => $hookData['user'],
                    'short' => true,
                ],
                [
                    'title' => 'Path',
                    'value' => $hookData['file_path'],
                    'short' => true,
                ],
            ],
            'footer' => 'FileGator',
            'ts' => time(),
        ],
    ],
];

// Send to Slack
$ch = curl_init($webhookUrl);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode($message),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 10,
]);

$response = curl_exec($ch);
$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

function formatBytes($bytes) {
    $units = ['B', 'KB', 'MB', 'GB'];
    $i = 0;
    while ($bytes >= 1024 && $i < count($units) - 1) {
        $bytes /= 1024;
        $i++;
    }
    return round($bytes, 2) . ' ' . $units[$i];
}

return [
    'status' => $httpCode === 200 ? 'sent' : 'error',
    'channel' => $slackConfig['channel'],
];
```

## Email Notification Hook

Location: `private/hooks/onUpload/notify_email.php`

```php
<?php
/**
 * Email Notification Hook
 * Sends upload notifications via email
 */

$config = include dirname(__DIR__) . '/config.php';
$emailConfig = $config['notifications']['email'] ?? [];

if (!($emailConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

$recipients = $emailConfig['upload_recipients'] ?? [];
if (empty($recipients)) {
    return ['status' => 'no_recipients'];
}

$smtp = $emailConfig['smtp'] ?? [];
$from = $emailConfig['from'] ?? [];

// Build email
$subject = sprintf('FileGator: New upload by %s', $hookData['user']);
$body = sprintf(
    "A new file has been uploaded:\n\n" .
    "File: %s\n" .
    "Size: %s bytes\n" .
    "Path: %s\n" .
    "User: %s\n" .
    "Time: %s\n",
    $hookData['file_name'],
    number_format($hookData['file_size']),
    $hookData['file_path'],
    $hookData['user'],
    date('Y-m-d H:i:s')
);

// Send using PHP mail() - for production, use PHPMailer or similar
$headers = [
    'From: ' . ($from['name'] ?? 'FileGator') . ' <' . ($from['address'] ?? 'noreply@example.com') . '>',
    'Content-Type: text/plain; charset=UTF-8',
];

$sent = 0;
foreach ($recipients as $email) {
    if (mail($email, $subject, $body, implode("\r\n", $headers))) {
        $sent++;
    }
}

return [
    'status' => 'sent',
    'recipients' => $sent,
];
```

## Discord Notification Hook

Location: `private/hooks/onUpload/notify_discord.php`

```php
<?php
/**
 * Discord Notification Hook
 */

$config = include dirname(__DIR__) . '/config.php';
$discordConfig = $config['notifications']['discord'] ?? [];

if (!($discordConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

$webhookUrl = $discordConfig['webhook_url'] ?? '';
if (empty($webhookUrl)) {
    return ['status' => 'error', 'message' => 'No webhook URL'];
}

$message = [
    'embeds' => [
        [
            'title' => 'New File Upload',
            'color' => 3066993, // Green
            'fields' => [
                ['name' => 'File', 'value' => $hookData['file_name'], 'inline' => true],
                ['name' => 'User', 'value' => $hookData['user'], 'inline' => true],
                ['name' => 'Size', 'value' => round($hookData['file_size'] / 1024, 2) . ' KB', 'inline' => true],
                ['name' => 'Path', 'value' => '`' . $hookData['file_path'] . '`'],
            ],
            'timestamp' => date('c'),
            'footer' => ['text' => 'FileGator'],
        ],
    ],
];

$ch = curl_init($webhookUrl);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode($message),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 10,
]);

$response = curl_exec($ch);
$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

return ['status' => $httpCode >= 200 && $httpCode < 300 ? 'sent' : 'error'];
```

## Generic Webhook Hook

Location: `private/hooks/onUpload/notify_webhook.php`

```php
<?php
/**
 * Generic Webhook Notification
 */

$config = include dirname(__DIR__) . '/config.php';
$webhookConfig = $config['notifications']['webhook'] ?? [];

if (!($webhookConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

$url = $webhookConfig['url'] ?? '';
if (empty($url)) {
    return ['status' => 'error', 'message' => 'No URL'];
}

$payload = [
    'event' => 'upload',
    'timestamp' => date('c'),
    'data' => [
        'file_name' => $hookData['file_name'],
        'file_path' => $hookData['file_path'],
        'file_size' => $hookData['file_size'],
        'user' => $hookData['user'],
    ],
];

$headers = array_merge(
    ['Content-Type: application/json'],
    array_map(
        fn($k, $v) => "$k: $v",
        array_keys($webhookConfig['headers'] ?? []),
        array_values($webhookConfig['headers'] ?? [])
    )
);

$ch = curl_init($url);
curl_setopt_array($ch, [
    CURLOPT_CUSTOMREQUEST => $webhookConfig['method'] ?? 'POST',
    CURLOPT_POSTFIELDS => json_encode($payload),
    CURLOPT_HTTPHEADER => $headers,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 10,
]);

$response = curl_exec($ch);
$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

return [
    'status' => $httpCode >= 200 && $httpCode < 300 ? 'sent' : 'error',
    'http_code' => $httpCode,
];
```

## Getting Webhook URLs

### Slack
1. Go to https://api.slack.com/apps
2. Create new app or select existing
3. Enable "Incoming Webhooks"
4. Create webhook for desired channel
5. Copy webhook URL

### Discord
1. Server Settings → Integrations → Webhooks
2. Create New Webhook
3. Select channel
4. Copy webhook URL

## Testing Notifications

```bash
# Test Slack webhook directly
curl -X POST -H 'Content-type: application/json' \
  --data '{"text":"Test from FileGator"}' \
  YOUR_WEBHOOK_URL
```
