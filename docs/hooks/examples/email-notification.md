# Email Notification Hook Example

This example demonstrates how to send email notifications when files are uploaded, including all file details like path, size, and uploader information.

## Overview

The email notification hook sends an email to configured recipients whenever a file upload completes. This is useful for:

- Alerting administrators of new uploads
- Triggering external workflows or processes
- Audit and compliance notifications
- Notifying team members of shared files

## Configuration

Edit `private/hooks/config.php` to configure email settings:

```php
'notifications' => [
    'email' => [
        // Enable/disable email notifications
        'enabled' => true,

        // SMTP settings (for advanced setups)
        'smtp' => [
            'host' => getenv('SMTP_HOST') ?: 'localhost',
            'port' => getenv('SMTP_PORT') ?: 587,
            'username' => getenv('SMTP_USER') ?: '',
            'password' => getenv('SMTP_PASS') ?: '',
            'encryption' => 'tls',  // 'tls', 'ssl', or null
        ],

        // Sender information
        'from' => [
            'address' => 'filegator@example.com',
            'name' => 'FileGator',
        ],

        // Recipients for upload notifications
        'upload_recipients' => [
            'admin@example.com',
            'uploads@example.com',
            'workflow-trigger@example.com',
        ],

        // Recipients for security alerts (different hook)
        'security_recipients' => [
            'security@example.com',
        ],
    ],
],
```

## Hook Script

Location: `private/hooks/onUpload/email_notification.php`

The hook script:
1. Loads configuration from `config.php`
2. Validates that email notifications are enabled
3. Formats file details into a readable email
4. Sends to all configured recipients
5. Logs the notification activity

## Email Content

The notification email includes:

```
Subject: [FileGator] New file uploaded: document.pdf

A new file has been uploaded to FileGator.

═══════════════════════════════════════════════════
FILE DETAILS
═══════════════════════════════════════════════════

File Name:    document.pdf
File Size:    2.5 MB (2621440 bytes)
File Type:    pdf
File Path:    /documents/reports/document.pdf

═══════════════════════════════════════════════════
UPLOAD DETAILS
═══════════════════════════════════════════════════

Uploaded By:  john.smith
Home Dir:     /users/john.smith
Upload Time:  2024-01-15 14:30:45 UTC
Server:       files.example.com

═══════════════════════════════════════════════════

This is an automated notification from FileGator.
```

## Using Environment Variables

For security, store sensitive SMTP credentials in environment variables:

### Apache (.htaccess)
```apache
SetEnv SMTP_HOST "smtp.example.com"
SetEnv SMTP_PORT "587"
SetEnv SMTP_USER "user@example.com"
SetEnv SMTP_PASS "your-password"
```

### Nginx (fastcgi_params)
```nginx
fastcgi_param SMTP_HOST "smtp.example.com";
fastcgi_param SMTP_PORT "587";
fastcgi_param SMTP_USER "user@example.com";
fastcgi_param SMTP_PASS "your-password";
```

### PHP-FPM (pool.d/www.conf)
```ini
env[SMTP_HOST] = "smtp.example.com"
env[SMTP_PORT] = "587"
env[SMTP_USER] = "user@example.com"
env[SMTP_PASS] = "your-password"
```

### .env file
```
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=user@example.com
SMTP_PASS=your-password
```

## Advanced: Using PHPMailer

For more robust email delivery, use PHPMailer instead of PHP's `mail()` function:

```php
<?php
/**
 * Email Notification Hook with PHPMailer
 * Requires: composer require phpmailer/phpmailer
 */

use PHPMailer\PHPMailer\PHPMailer;
use PHPMailer\PHPMailer\SMTP;

// Load configuration
$config = include dirname(__DIR__) . '/config.php';
$emailConfig = $config['notifications']['email'] ?? [];

if (!($emailConfig['enabled'] ?? false)) {
    return ['status' => 'disabled'];
}

// Require PHPMailer (adjust path as needed)
require_once dirname(__DIR__, 3) . '/vendor/autoload.php';

$smtp = $emailConfig['smtp'] ?? [];
$from = $emailConfig['from'] ?? [];
$recipients = $emailConfig['upload_recipients'] ?? [];

$mail = new PHPMailer(true);

try {
    // SMTP configuration
    $mail->isSMTP();
    $mail->Host = $smtp['host'] ?? 'localhost';
    $mail->Port = $smtp['port'] ?? 587;

    if (!empty($smtp['username'])) {
        $mail->SMTPAuth = true;
        $mail->Username = $smtp['username'];
        $mail->Password = $smtp['password'];
    }

    if (!empty($smtp['encryption'])) {
        $mail->SMTPSecure = $smtp['encryption'];
    }

    // Sender
    $mail->setFrom(
        $from['address'] ?? 'filegator@localhost',
        $from['name'] ?? 'FileGator'
    );

    // Recipients
    foreach ($recipients as $recipient) {
        $mail->addAddress($recipient);
    }

    // Content
    $mail->isHTML(false);
    $mail->Subject = sprintf('[FileGator] New file uploaded: %s', $hookData['file_name']);
    $mail->Body = buildEmailBody($hookData);

    $mail->send();

    return ['status' => 'sent', 'recipients' => count($recipients)];

} catch (Exception $e) {
    return ['status' => 'error', 'message' => $mail->ErrorInfo];
}

function buildEmailBody($data) {
    return sprintf(
        "File: %s\nSize: %d bytes\nPath: %s\nUser: %s\nTime: %s",
        $data['file_name'],
        $data['file_size'],
        $data['file_path'],
        $data['user'],
        date('Y-m-d H:i:s')
    );
}
```

## Triggering External Processes

The email notification can trigger external workflows:

### 1. Email to Process Automation Tool

Configure a recipient that's monitored by a process automation tool like Zapier, Make (Integromat), or n8n:

```php
'upload_recipients' => [
    'trigger-xyz123@hooks.zapier.com',
    'workflow@hooks.make.com',
],
```

### 2. Email to Ticketing System

Send to a ticketing system's email intake:

```php
'upload_recipients' => [
    'support+uploads@yourcompany.freshdesk.com',
    'jira+PROJ-uploads@yourcompany.atlassian.net',
],
```

### 3. Email to Custom Parser

Send to a custom email parser that processes attachments or triggers actions:

```php
'upload_recipients' => [
    'file-processor@internal.example.com',
],
```

## Logging

Email activity is logged to `private/logs/email_notifications.log`:

```
[2024-01-15 14:30:45] Upload notification for 'document.pdf' by 'john' - Sent: 3, Failed: 0
[2024-01-15 14:35:12] Upload notification for 'image.png' by 'jane' - Sent: 2, Failed: 1
[2024-01-15 14:35:12] Errors: Failed to send to: invalid-email
```

## Testing

1. Enable email notifications in `config.php`
2. Add your email to `upload_recipients`
3. Upload a test file
4. Check your inbox and `private/logs/email_notifications.log`

### Testing with MailHog (Development)

For local development, use MailHog to capture emails:

```bash
# Install MailHog
go install github.com/mailhog/MailHog@latest

# Run MailHog
MailHog
```

Configure PHP to use MailHog:
```ini
; php.ini
sendmail_path = "/usr/local/bin/mhsendmail"
```

Or configure SMTP in `config.php`:
```php
'smtp' => [
    'host' => 'localhost',
    'port' => 1025,
    'username' => '',
    'password' => '',
    'encryption' => null,
],
```

Access MailHog UI at http://localhost:8025 to view captured emails.

## Customizing the Email

### HTML Email Version

```php
// Build HTML body
$htmlBody = <<<HTML
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; }
        .header { background: #4a90d9; color: white; padding: 20px; }
        .content { padding: 20px; background: #f5f5f5; }
        .details { background: white; padding: 15px; margin: 10px 0; }
        .label { font-weight: bold; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>New File Upload</h2>
        </div>
        <div class="content">
            <div class="details">
                <p><span class="label">File:</span> {$hookData['file_name']}</p>
                <p><span class="label">Size:</span> {$formattedSize}</p>
                <p><span class="label">Path:</span> {$hookData['file_path']}</p>
                <p><span class="label">Uploaded by:</span> {$hookData['user']}</p>
                <p><span class="label">Time:</span> {$timestamp}</p>
            </div>
        </div>
    </div>
</body>
</html>
HTML;
```

### Conditional Notifications

Only notify for certain file types or sizes:

```php
// Only notify for files > 10MB
if ($hookData['file_size'] < 10 * 1024 * 1024) {
    return ['status' => 'skipped', 'reason' => 'file_too_small'];
}

// Only notify for specific extensions
$notifyExtensions = ['pdf', 'doc', 'docx', 'xls', 'xlsx'];
$ext = strtolower(pathinfo($hookData['file_name'], PATHINFO_EXTENSION));
if (!in_array($ext, $notifyExtensions)) {
    return ['status' => 'skipped', 'reason' => 'extension_not_monitored'];
}
```
