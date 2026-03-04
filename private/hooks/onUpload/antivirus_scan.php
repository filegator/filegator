<?php

/**
 * Antivirus Scan Hook
 *
 * This hook is triggered when a file upload completes.
 * It launches an external antivirus scan process on the uploaded file.
 *
 * Configuration is loaded from: private/hooks/config.php
 * Edit that file to configure:
 * - Scanner type (clamav, virustotal, custom)
 * - API keys
 * - Quarantine directories
 * - File size limits
 * - Extensions to skip
 *
 * The $hookData array contains:
 * - file_path: The destination path where the file was stored
 * - file_name: The original file name
 * - file_size: The file size in bytes
 * - user: The username who uploaded the file
 * - home_dir: The user's home directory
 *
 * Return values:
 * - true or null: Success, continue with other hooks
 * - false: Failure, stop hook processing
 * - array: Custom result data
 */

// Load configuration
$configFile = dirname(__DIR__) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get antivirus settings with defaults
$avConfig = $config['antivirus'] ?? [];
$globalConfig = $config['global'] ?? [];

// Check if antivirus scanning is enabled
if (!($avConfig['enabled'] ?? true)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Antivirus scanning is disabled',
    ];
}

// Get the repository path
$repositoryPath = dirname(__DIR__, 3) . '/repository';

// Build the full file path
$fullPath = $repositoryPath . $hookData['home_dir'] . $hookData['file_path'];
$fullPath = realpath($fullPath);

// Validate the file exists and is within the repository
if (!$fullPath || strpos($fullPath, realpath($repositoryPath)) !== 0) {
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Invalid file path',
    ];
}

if (!file_exists($fullPath)) {
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'File not found: ' . $fullPath,
    ];
}

// Check if file extension should be skipped
$extension = strtolower(pathinfo($fullPath, PATHINFO_EXTENSION));
$skipExtensions = $avConfig['skip_extensions'] ?? [];
if (in_array($extension, $skipExtensions)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Extension skipped from scanning: ' . $extension,
    ];
}

// Check file size limit
$maxSize = $avConfig['max_file_size'] ?? 0;
if ($maxSize > 0 && $hookData['file_size'] > $maxSize) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'File too large for scanning: ' . $hookData['file_size'] . ' bytes',
    ];
}

// Determine log file
$avLogFile = dirname(__DIR__, 2) . '/logs/antivirus.log';

// Log the scan initiation
$logMessage = sprintf(
    "[%s] Initiating antivirus scan for: %s (uploaded by: %s, size: %d bytes)\n",
    date('Y-m-d H:i:s'),
    $fullPath,
    $hookData['user'],
    $hookData['file_size']
);
@file_put_contents($avLogFile, $logMessage, FILE_APPEND);

// Determine scanner type from config
$scanner = $avConfig['scanner'] ?? 'clamav';
$command = null;

switch ($scanner) {
    case 'clamav':
        $clamConfig = $avConfig['clamav'] ?? [];
        $useDaemon = $clamConfig['use_daemon'] ?? false;
        $binary = $useDaemon
            ? ($clamConfig['daemon_binary'] ?? '/usr/bin/clamdscan')
            : ($clamConfig['binary'] ?? '/usr/bin/clamscan');

        $autoRemove = $clamConfig['auto_remove'] ?? false;
        $quarantineDir = $clamConfig['quarantine_dir'] ?? null;

        // Create quarantine directory if needed
        if ($quarantineDir && !is_dir($quarantineDir)) {
            @mkdir($quarantineDir, 0755, true);
        }

        // Build ClamAV command
        $removeFlag = $autoRemove ? '--remove=yes' : '';
        $moveFlag = ($quarantineDir && !$autoRemove) ? '--move=' . escapeshellarg($quarantineDir) : '';

        $command = sprintf(
            'nohup %s %s %s --quiet %s >> %s 2>&1 &',
            escapeshellarg($binary),
            $removeFlag,
            $moveFlag,
            escapeshellarg($fullPath),
            escapeshellarg($avLogFile)
        );
        break;

    case 'virustotal':
        $vtConfig = $avConfig['virustotal'] ?? [];
        $apiKey = $vtConfig['api_key'] ?? '';

        if (empty($apiKey)) {
            return [
                'action' => 'continue',
                'status' => 'error',
                'message' => 'VirusTotal API key not configured in hooks/config.php',
            ];
        }

        // VirusTotal requires a separate implementation
        $logMessage = sprintf(
            "[%s] VirusTotal scan queued for: %s\n",
            date('Y-m-d H:i:s'),
            $fullPath
        );
        @file_put_contents($avLogFile, $logMessage, FILE_APPEND);

        return [
            'action' => 'continue',
            'status' => 'queued',
            'scanner' => 'virustotal',
            'message' => 'File queued for VirusTotal scan',
        ];

    case 'custom':
        $customConfig = $avConfig['custom'] ?? [];
        $commandTemplate = $customConfig['command'] ?? '';

        if (empty($commandTemplate)) {
            return [
                'action' => 'continue',
                'status' => 'error',
                'message' => 'Custom scanner command not configured in hooks/config.php',
            ];
        }

        $command = str_replace(
            ['{file_path}', '{log_file}'],
            [escapeshellarg($fullPath), escapeshellarg($avLogFile)],
            $commandTemplate
        );
        $command = 'nohup ' . $command . ' &';
        break;

    default:
        // Fallback: use scan_worker.sh if exists
        $scanScript = dirname(__FILE__) . '/scan_worker.sh';

        if (file_exists($scanScript) && is_executable($scanScript)) {
            $command = sprintf(
                'nohup %s %s %s >> %s 2>&1 &',
                escapeshellarg($scanScript),
                escapeshellarg($fullPath),
                escapeshellarg($hookData['user']),
                escapeshellarg($avLogFile)
            );
        }
}

// Execute the scan command
if ($command) {
    exec($command);

    $logMessage = sprintf(
        "[%s] Scan process launched (scanner: %s) for: %s\n",
        date('Y-m-d H:i:s'),
        $scanner,
        $fullPath
    );
    @file_put_contents($avLogFile, $logMessage, FILE_APPEND);

    return [
        'action' => 'continue',
        'status' => 'scan_initiated',
        'scanner' => $scanner,
        'file' => $fullPath,
        'message' => 'Antivirus scan process launched',
    ];
}

return [
    'action' => 'continue',
    'status' => 'no_scanner',
    'message' => 'No antivirus scanner configured or available',
];
