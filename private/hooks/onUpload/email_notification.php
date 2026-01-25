<?php

/**
 * Email Notification Hook
 *
 * This hook is triggered when a file upload completes.
 * It sends an email notification with file details to configured recipients.
 *
 * Configuration is loaded from: private/hooks/config.php
 * Edit that file to configure:
 * - SMTP settings (host, port, credentials)
 * - Sender address and name
 * - Recipient email addresses
 *
 * The $hookData array contains:
 * - file_path: The destination path where the file was stored
 * - file_name: The original file name
 * - file_size: The file size in bytes
 * - user: The username who uploaded the file
 * - home_dir: The user's home directory
 *
 * Return values:
 * - array with status information
 */

// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get email notification settings
$notifyConfig = $config['notifications']['email'] ?? [];

// Check if email notifications are enabled
if (!($notifyConfig['enabled'] ?? false)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Email notifications are disabled',
    ];
}

// Get recipients
$recipients = $notifyConfig['upload_recipients'] ?? [];
if (empty($recipients)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'No email recipients configured',
    ];
}

// Get sender info
$fromAddress = $notifyConfig['from']['address'] ?? 'filegator@localhost';
$fromName = $notifyConfig['from']['name'] ?? 'FileGator';

// Format file size for readability
function formatFileSize($bytes) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $i = 0;
    while ($bytes >= 1024 && $i < count($units) - 1) {
        $bytes /= 1024;
        $i++;
    }
    return round($bytes, 2) . ' ' . $units[$i];
}

// Get file extension
$extension = pathinfo($hookData['file_name'], PATHINFO_EXTENSION);

// Build email subject
$subject = sprintf(
    '[FileGator] New file uploaded: %s',
    $hookData['file_name']
);

// Build email body (plain text)
$body = <<<EMAIL
A new file has been uploaded to FileGator.

═══════════════════════════════════════════════════
FILE DETAILS
═══════════════════════════════════════════════════

File Name:    {$hookData['file_name']}
File Size:    %s ({$hookData['file_size']} bytes)
File Type:    {$extension}
File Path:    {$hookData['file_path']}

═══════════════════════════════════════════════════
UPLOAD DETAILS
═══════════════════════════════════════════════════

Uploaded By:  {$hookData['user']}
Home Dir:     {$hookData['home_dir']}
Upload Time:  %s
Server:       %s

═══════════════════════════════════════════════════

This is an automated notification from FileGator.
EMAIL;

$body = sprintf(
    $body,
    formatFileSize($hookData['file_size']),
    date('Y-m-d H:i:s T'),
    $_SERVER['SERVER_NAME'] ?? gethostname() ?? 'unknown'
);

// Build email headers
$headers = [
    'From: ' . $fromName . ' <' . $fromAddress . '>',
    'Reply-To: ' . $fromAddress,
    'X-Mailer: FileGator-Hooks/1.0',
    'X-Priority: 3',
    'Content-Type: text/plain; charset=UTF-8',
    'MIME-Version: 1.0',
];

// Log file for email activity
$logFile = dirname(__DIR__, 2) . '/logs/email_notifications.log';

// Send emails
$sent = 0;
$failed = 0;
$errors = [];

foreach ($recipients as $recipient) {
    $recipient = trim($recipient);
    if (empty($recipient) || !filter_var($recipient, FILTER_VALIDATE_EMAIL)) {
        $errors[] = "Invalid email: {$recipient}";
        $failed++;
        continue;
    }

    // Attempt to send email using PHP's mail() function
    // For production, consider using PHPMailer, SwiftMailer, or similar
    $result = @mail(
        $recipient,
        $subject,
        $body,
        implode("\r\n", $headers)
    );

    if ($result) {
        $sent++;
    } else {
        $failed++;
        $errors[] = "Failed to send to: {$recipient}";
    }
}

// Log the notification
$logMessage = sprintf(
    "[%s] Upload notification for '%s' by '%s' - Sent: %d, Failed: %d\n",
    date('Y-m-d H:i:s'),
    $hookData['file_name'],
    $hookData['user'],
    $sent,
    $failed
);
@file_put_contents($logFile, $logMessage, FILE_APPEND);

// Log errors if any
if (!empty($errors)) {
    $errorLog = "[" . date('Y-m-d H:i:s') . "] Errors: " . implode('; ', $errors) . "\n";
    @file_put_contents($logFile, $errorLog, FILE_APPEND);
}

return [
    'action' => 'continue',
    'status' => $sent > 0 ? 'sent' : 'failed',
    'sent' => $sent,
    'failed' => $failed,
    'recipients' => count($recipients),
    'errors' => $errors,
    'message' => $sent > 0
        ? "Notification sent to {$sent} recipient(s)"
        : 'Failed to send notifications',
];
