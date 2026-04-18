<?php

/**
 * Hook: Trend Micro Vision One File Security Scan
 *
 * Triggered: After file upload to /upload directory
 * Action: Scan file using Trend Micro Vision One File Security API
 * Clean: Move to /scanned directory
 * Malware: Delete file and send email notification to admin
 * Error: Log error and optionally quarantine file
 *
 * Environment Variables:
 *   - FG_FILE_PATH: Destination folder (cwd) where file was uploaded (e.g., "/upload")
 *   - FG_FILE_NAME: Name of the uploaded file
 *   - FG_FILE_SIZE: Size of the file in bytes
 *   - FG_USER: Username who uploaded the file
 *   - FG_HOMEDIR: User's home directory
 *   - FG_REPOSITORY: Absolute path to repository root
 *
 * Configuration:
 *   Loaded from private/hooks/config.php under 'trend_micro' key
 */

// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get Trend Micro settings
$tmConfig = $config['trend_micro'] ?? [];
$globalConfig = $config['global'] ?? [];
$notificationsConfig = $config['notifications'] ?? [];

// Check if Trend Micro scanning is enabled
if (!($tmConfig['enabled'] ?? false)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Trend Micro scanning is disabled in configuration',
    ];
}

// Extract hook data (passed from FileGator)
$filePath = $hookData['file_path'] ?? '';
$fileName = $hookData['file_name'] ?? '';
$fileSize = $hookData['file_size'] ?? 0;
$user = $hookData['user'] ?? 'unknown';
$homeDir = $hookData['home_dir'] ?? '/';

// Only process files in /upload directory
// Note: file_path is the destination folder (cwd), not the full path with filename
// Handle both "/upload" and "/upload/" formats
$uploadFolder = rtrim($filePath, '/');
if ($uploadFolder !== '/upload' && strpos($uploadFolder . '/', '/upload/') !== 0) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => "Not in upload folder (path: {$filePath}) - Trend Micro scan not triggered",
    ];
}

// Build absolute file path
// Note: $filePath is the folder, $fileName is the actual file name
// dirname levels: __DIR__ = .../private/hooks/onUpload
//   dirname(__DIR__, 1) = .../private/hooks
//   dirname(__DIR__, 2) = .../private
//   dirname(__DIR__, 3) = .../filegator (root)
$repositoryPath = dirname(__DIR__, 3) . '/repository';
$relativeFilePath = rtrim($filePath, '/') . '/' . $fileName;
$fullPath = realpath($repositoryPath . $homeDir . $relativeFilePath);

// Debug logging
error_log("[Trend Micro Hook] repositoryPath: {$repositoryPath}");
error_log("[Trend Micro Hook] homeDir: {$homeDir}");
error_log("[Trend Micro Hook] filePath: {$filePath}");
error_log("[Trend Micro Hook] fileName: {$fileName}");
error_log("[Trend Micro Hook] relativeFilePath: {$relativeFilePath}");
error_log("[Trend Micro Hook] fullPath: " . ($fullPath ?: 'FALSE (realpath failed)'));

// Validate path security (prevent directory traversal)
if (!$fullPath || !file_exists($fullPath) || strpos($fullPath, realpath($repositoryPath)) !== 0) {
    error_log("[Trend Micro Hook] Invalid or inaccessible file path: {$relativeFilePath}");
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => "Invalid file path: {$relativeFilePath}",
    ];
}

// Validate API key is configured
// Note: The config.php file should load .env automatically via putenv()
$apiKey = $tmConfig['api_key'] ?? getenv('TREND_MICRO_API_KEY') ?: '';

// Debug: Show where we're looking for the API key
error_log("[Trend Micro Hook] Config file: " . dirname(__DIR__) . '/config.php');
error_log("[Trend Micro Hook] tmConfig['api_key']: " . (isset($tmConfig['api_key']) && $tmConfig['api_key'] ? 'SET (length: ' . strlen($tmConfig['api_key']) . ')' : 'NOT SET'));
error_log("[Trend Micro Hook] getenv('TREND_MICRO_API_KEY'): " . (getenv('TREND_MICRO_API_KEY') ? 'SET (length: ' . strlen(getenv('TREND_MICRO_API_KEY')) . ')' : 'NOT SET'));
error_log("[Trend Micro Hook] .env file exists: " . (file_exists(dirname(__DIR__, 3) . '/.env') ? 'YES' : 'NO'));

if (empty($apiKey)) {
    error_log("[Trend Micro Hook] API key not configured");
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Trend Micro API key not configured (set TREND_MICRO_API_KEY environment variable or check .env file)',
    ];
}

// Check file size limit
$maxFileSize = $tmConfig['max_file_size'] ?? (100 * 1024 * 1024); // Default 100MB
if ($fileSize > $maxFileSize) {
    $logMessage = sprintf(
        "[%s] SCAN_SKIPPED: File too large (%d bytes > %d bytes): %s (user: %s)\n",
        date('Y-m-d H:i:s'),
        $fileSize,
        $maxFileSize,
        $fileName,
        $user
    );
    logToFile($tmConfig['log_file'] ?? null, $logMessage);

    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => "File too large for scanning ({$fileSize} bytes)",
    ];
}

// Log scan initiation
$logMessage = sprintf(
    "[%s] SCAN_INITIATED: %s (size: %d bytes, user: %s, path: %s)\n",
    date('Y-m-d H:i:s'),
    $fileName,
    $fileSize,
    $user,
    $filePath
);
logToFile($tmConfig['log_file'] ?? null, $logMessage);

// Perform Trend Micro scan
try {
    $scanResult = scanFileWithTrendMicro($fullPath, $fileName, $apiKey, $tmConfig);

    // Log scan result
    $logMessage = sprintf(
        "[%s] SCAN_COMPLETED: %s - Result: %s, Malware: %s, ScanID: %s\n",
        date('Y-m-d H:i:s'),
        $fileName,
        $scanResult['status'],
        $scanResult['malware_found'] ? 'YES' : 'NO',
        $scanResult['scan_id'] ?? 'N/A'
    );
    logToFile($tmConfig['log_file'] ?? null, $logMessage);

    // Handle clean files
    if ($scanResult['status'] === 'clean') {
        // Move to /scanned directory
        $scannedDir = $repositoryPath . $homeDir . '/scanned';
        if (!is_dir($scannedDir)) {
            mkdir($scannedDir, 0755, true);
        }

        $destPath = $scannedDir . '/' . $fileName;

        // Handle filename conflicts
        $counter = 1;
        $baseName = pathinfo($fileName, PATHINFO_FILENAME);
        $extension = pathinfo($fileName, PATHINFO_EXTENSION);
        while (file_exists($destPath)) {
            $destPath = $scannedDir . '/' . $baseName . '_' . $counter . ($extension ? '.' . $extension : '');
            $counter++;
        }

        if (rename($fullPath, $destPath)) {
            $logMessage = sprintf(
                "[%s] FILE_MOVED: %s -> /scanned/%s (clean scan)\n",
                date('Y-m-d H:i:s'),
                $filePath,
                basename($destPath)
            );
            logToFile($tmConfig['log_file'] ?? null, $logMessage);

            return [
                'action' => 'continue',
                'status' => 'success',
                'result' => 'clean',
                'message' => 'File scanned successfully - moved to /scanned',
                'scan_id' => $scanResult['scan_id'] ?? null,
                'destination' => '/scanned/' . basename($destPath),
            ];
        } else {
            error_log("[Trend Micro Hook] Failed to move clean file: {$fullPath}");
            return [
                'action' => 'continue',
                'status' => 'error',
                'message' => 'File is clean but failed to move to /scanned directory',
            ];
        }
    }

    // Handle malware detection
    if ($scanResult['status'] === 'malware') {
        // Log malware detection
        $threatDetails = isset($scanResult['threats']) && is_array($scanResult['threats'])
            ? implode(', ', array_map(function($t) { return $t['malwareName'] ?? 'Unknown'; }, $scanResult['threats']))
            : 'Unknown threat';

        $logMessage = sprintf(
            "[%s] MALWARE_DETECTED: %s - Threats: %s (user: %s, scan_id: %s)\n",
            date('Y-m-d H:i:s'),
            $fileName,
            $threatDetails,
            $user,
            $scanResult['scan_id'] ?? 'N/A'
        );
        logToFile($tmConfig['malware_log'] ?? ($tmConfig['log_file'] ?? null), $logMessage);

        // Delete the infected file
        if (unlink($fullPath)) {
            $logMessage = sprintf(
                "[%s] FILE_DELETED: %s (malware detected and removed)\n",
                date('Y-m-d H:i:s'),
                $filePath
            );
            logToFile($tmConfig['log_file'] ?? null, $logMessage);
        }

        // Send email notification to admin
        sendMalwareNotification($fileName, $user, $scanResult, $notificationsConfig);

        return [
            'action' => 'stop',
            'status' => 'malware_detected',
            'result' => 'malware',
            'message' => 'Malware detected and file deleted',
            'scan_id' => $scanResult['scan_id'] ?? null,
            'threats' => $scanResult['threats'] ?? [],
        ];
    }

    // Handle scan errors
    $logMessage = sprintf(
        "[%s] SCAN_ERROR: %s - Error: %s\n",
        date('Y-m-d H:i:s'),
        $fileName,
        $scanResult['error'] ?? 'Unknown error'
    );
    logToFile($tmConfig['log_file'] ?? null, $logMessage);

    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Scan failed: ' . ($scanResult['error'] ?? 'Unknown error'),
        'error_details' => $scanResult['error'] ?? null,
    ];

} catch (Exception $e) {
    error_log("[Trend Micro Hook] Exception during scan: " . $e->getMessage());

    $logMessage = sprintf(
        "[%s] SCAN_EXCEPTION: %s - Exception: %s\n",
        date('Y-m-d H:i:s'),
        $fileName,
        $e->getMessage()
    );
    logToFile($tmConfig['log_file'] ?? null, $logMessage);

    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Scan exception: ' . $e->getMessage(),
    ];
}

/**
 * Scan file with Trend Micro Vision One File Security API
 *
 * Uses the Trend Micro Vision One File Security PHP SDK for scanning.
 * The SDK uses Node.js with gRPC for communication with Trend Micro servers.
 *
 * Installation:
 *   cd /path/to/filegator/private
 *   composer require trendandrew/file-security-sdk
 *   cd vendor/trendandrew/file-security-sdk/service && npm install
 *
 * @param string $filePath Absolute path to file
 * @param string $fileName File name
 * @param string $apiKey Trend Micro API key
 * @param array $config Configuration array
 * @return array Result array with keys: status, malware_found, scan_id, threats, error
 */
function scanFileWithTrendMicro($filePath, $fileName, $apiKey, $config) {
    // Path: hooks/onUpload/02_scan_upload.php is at private/hooks/onUpload/
    // dirname(__DIR__, 2) goes up 2 levels: onUpload -> hooks -> private
    $privateDir = dirname(__DIR__, 2);

    // Check for Composer autoloader
    $composerAutoload = $privateDir . '/vendor/autoload.php';
    if (file_exists($composerAutoload)) {
        require_once $composerAutoload;
        if (class_exists('TrendMicroScanner')) {
            error_log("[Trend Micro Hook] Using SDK via Composer autoloader");
            return scanFileWithSDK($filePath, $fileName, $apiKey, $config);
        }
    }

    // SDK not found - cannot proceed
    error_log("[Trend Micro Hook] ERROR: TrendMicroScanner SDK not found");
    error_log("[Trend Micro Hook] Install via Composer:");
    error_log("[Trend Micro Hook]   cd $privateDir");
    error_log("[Trend Micro Hook]   composer require trendandrew/file-security-sdk");
    error_log("[Trend Micro Hook]   cd vendor/trendandrew/file-security-sdk/service && npm install");

    return [
        'status' => 'error',
        'error' => "TrendMicroScanner SDK not found. Install via: composer require trendandrew/file-security-sdk",
        'malware_found' => false,
    ];
}

/**
 * Scan file using the Trend Micro PHP SDK
 *
 * @param string $filePath Absolute path to file
 * @param string $fileName File name
 * @param string $apiKey Trend Micro API key
 * @param array $config Configuration array
 * @return array Result array
 */
function scanFileWithSDK($filePath, $fileName, $apiKey, $config) {

    try {
        $region = $config['region'] ?? getenv('TREND_MICRO_REGION') ?: 'us';
        $timeout = $config['scan_timeout'] ?? 300;
        $debug = $config['debug'] ?? false;

        $scanner = new TrendMicroScanner($region, $apiKey, $timeout, $debug);

        // Set log file if configured
        $logFile = $config['log_file'] ?? null;
        if ($logFile) {
            $scanner->setLogFile($logFile);
        }

        // Perform scan
        $result = $scanner->scanFile($filePath);

        $scanner->close();

        // Convert SDK result to hook format
        if ($result->hasMalware()) {
            $threats = [];
            foreach ($result->getFoundMalwares() as $malware) {
                $threats[] = [
                    'malwareName' => $malware->getMalwareName(),
                    'fileName' => $malware->getFileName(),
                    'type' => $malware->getType(),
                    'filter' => $malware->getFilter(),
                ];
            }

            return [
                'status' => 'malware',
                'malware_found' => true,
                'scan_id' => $result->getScanId(),
                'threats' => $threats,
                'file_sha256' => $result->getFileSha256(),
                'scan_timestamp' => date('c'),
            ];
        }

        return [
            'status' => 'clean',
            'malware_found' => false,
            'scan_id' => $result->getScanId(),
            'file_sha256' => $result->getFileSha256(),
            'scan_timestamp' => date('c'),
        ];

    } catch (TrendAndrew\FileSecurity\Exception\AuthenticationException $e) {
        return [
            'status' => 'error',
            'error' => 'Authentication failed: ' . $e->getMessage(),
            'malware_found' => false,
        ];
    } catch (TrendAndrew\FileSecurity\Exception\ConnectionException $e) {
        return [
            'status' => 'error',
            'error' => 'Connection failed: ' . $e->getMessage(),
            'malware_found' => false,
        ];
    } catch (TrendAndrew\FileSecurity\Exception\TimeoutException $e) {
        return [
            'status' => 'error',
            'error' => 'Scan timed out: ' . $e->getMessage(),
            'malware_found' => false,
        ];
    } catch (Exception $e) {
        return [
            'status' => 'error',
            'error' => 'SDK error: ' . $e->getMessage(),
            'malware_found' => false,
        ];
    }
}

/**
 * Send malware detection notification email
 *
 * @param string $fileName Name of infected file
 * @param string $user Username who uploaded
 * @param array $scanResult Scan result details
 * @param array $notificationsConfig Notification configuration
 */
function sendMalwareNotification($fileName, $user, $scanResult, $notificationsConfig) {
    $emailConfig = $notificationsConfig['email'] ?? [];

    // Check if email notifications are enabled
    if (!($emailConfig['enabled'] ?? false)) {
        return;
    }

    // Get admin email
    $adminEmail = $emailConfig['security_recipients'][0] ??
                  getenv('ADMIN_EMAIL') ?:
                  ($emailConfig['from']['address'] ?? null);

    if (empty($adminEmail)) {
        error_log("[Trend Micro Hook] Cannot send malware alert - no admin email configured");
        return;
    }

    // Build email content
    $threats = isset($scanResult['threats']) && is_array($scanResult['threats'])
        ? array_map(function($t) { return $t['malwareName'] ?? 'Unknown'; }, $scanResult['threats'])
        : ['Unknown threat'];

    $subject = "[SECURITY ALERT] Malware Detected: {$fileName}";

    $body = "MALWARE DETECTION ALERT\n";
    $body .= "========================\n\n";
    $body .= "A malicious file has been detected and removed from the system.\n\n";
    $body .= "File Details:\n";
    $body .= "  File Name: {$fileName}\n";
    $body .= "  Uploaded By: {$user}\n";
    $body .= "  Timestamp: " . date('Y-m-d H:i:s') . "\n";
    $body .= "  Scan ID: " . ($scanResult['scan_id'] ?? 'N/A') . "\n";
    $body .= "  File SHA256: " . ($scanResult['file_sha256'] ?? 'N/A') . "\n\n";
    $body .= "Detected Threats:\n";
    foreach ($threats as $threat) {
        $body .= "  - {$threat}\n";
    }
    $body .= "\nAction Taken:\n";
    $body .= "  The file has been automatically deleted to protect the system.\n\n";
    $body .= "Please investigate this incident and contact the user if necessary.\n\n";
    $body .= "---\n";
    $body .= "This is an automated security notification from FileGator.\n";

    // Try to use SMTP if configured, otherwise use native mail()
    $smtpConfig = $emailConfig['smtp'] ?? [];
    if (!empty($smtpConfig['host']) && !empty($smtpConfig['host'])) {
        sendEmailViaSMTP($adminEmail, $subject, $body, $smtpConfig, $emailConfig['from'] ?? []);
    } else {
        // Use native PHP mail()
        $fromAddress = $emailConfig['from']['address'] ?? 'filegator@localhost';
        $fromName = $emailConfig['from']['name'] ?? 'FileGator Security';
        $headers = "From: {$fromName} <{$fromAddress}>\r\n";
        $headers .= "Content-Type: text/plain; charset=UTF-8\r\n";

        mail($adminEmail, $subject, $body, $headers);
    }
}

/**
 * Send email via SMTP
 *
 * @param string $to Recipient email
 * @param string $subject Email subject
 * @param string $body Email body
 * @param array $smtpConfig SMTP configuration
 * @param array $fromConfig From address configuration
 */
function sendEmailViaSMTP($to, $subject, $body, $smtpConfig, $fromConfig) {
    // Check if PHPMailer is available
    if (class_exists('PHPMailer\PHPMailer\PHPMailer')) {
        try {
            $mail = new PHPMailer\PHPMailer\PHPMailer(true);
            $mail->isSMTP();
            $mail->Host = $smtpConfig['host'] ?? 'localhost';
            $mail->Port = $smtpConfig['port'] ?? 587;
            $mail->SMTPAuth = !empty($smtpConfig['username']);
            $mail->Username = $smtpConfig['username'] ?? '';
            $mail->Password = $smtpConfig['password'] ?? '';
            $mail->SMTPSecure = $smtpConfig['encryption'] ?? 'tls';
            $mail->setFrom($fromConfig['address'] ?? 'filegator@localhost', $fromConfig['name'] ?? 'FileGator Security');
            $mail->addAddress($to);
            $mail->Subject = $subject;
            $mail->Body = $body;
            $mail->send();
        } catch (Exception $e) {
            error_log("[Trend Micro Hook] Failed to send email via PHPMailer: " . $e->getMessage());
        }
    } else {
        // Fallback to native mail()
        $fromAddress = $fromConfig['address'] ?? 'filegator@localhost';
        $fromName = $fromConfig['name'] ?? 'FileGator Security';
        $headers = "From: {$fromName} <{$fromAddress}>\r\n";
        $headers .= "Content-Type: text/plain; charset=UTF-8\r\n";
        mail($to, $subject, $body, $headers);
    }
}

/**
 * Log message to file
 *
 * @param string|null $logFile Path to log file
 * @param string $message Log message
 */
function logToFile($logFile, $message) {
    if (empty($logFile)) {
        return;
    }

    // Create log directory if it doesn't exist
    $logDir = dirname($logFile);
    if (!is_dir($logDir)) {
        mkdir($logDir, 0755, true);
    }

    @file_put_contents($logFile, $message, FILE_APPEND);
}
