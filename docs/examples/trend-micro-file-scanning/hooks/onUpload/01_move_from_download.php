<?php

/**
 * Hook: Move Files from /download to /upload
 *
 * This hook is triggered after a file upload completes.
 * It automatically moves files uploaded to the /download directory into
 * the /upload directory for Trend Micro scanning.
 *
 * Workflow:
 * 1. User uploads file to /download directory
 * 2. This hook triggers and moves file to /upload
 * 3. File move triggers second onUpload hook for scanning
 *
 * Configuration is loaded from: private/hooks/config.php
 *
 * The $hookData array contains:
 * - file_path: The destination folder (cwd) where the file was stored (e.g., "/download")
 * - file_name: The original file name (e.g., "file.pdf")
 * - file_size: The file size in bytes
 * - user: The username who uploaded the file
 * - home_dir: The user's home directory (e.g., "/")
 *
 * Environment variables available:
 * - FG_FILE_PATH: Relative path of uploaded file
 * - FG_FILE_NAME: Filename
 * - FG_FILE_SIZE: Size in bytes
 * - FG_USER: Username
 * - FG_HOMEDIR: User's home directory
 * - FG_REPOSITORY: Absolute repository path
 *
 * Return values:
 * - array with status information
 */

// Load configuration
$configFile = dirname(__DIR__, 2) . '/config.php';
$config = file_exists($configFile) ? include $configFile : [];

// Get file movement settings
$moveConfig = $config['file_movement']['download_to_upload'] ?? [];

// Check if this feature is enabled
if (!($moveConfig['enabled'] ?? true)) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => 'Auto-move from /download to /upload is disabled',
    ];
}

// Get hook data
$filePath = $hookData['file_path'] ?? '';
$fileName = $hookData['file_name'] ?? '';
$fileSize = $hookData['file_size'] ?? 0;
$user = $hookData['user'] ?? 'unknown';
$homeDir = $hookData['home_dir'] ?? '/';

// Only process files uploaded to /download directory
// Note: file_path is the destination folder (cwd), not the full path with filename
// Handle both "/download" and "/download/" formats
$downloadFolder = rtrim($filePath, '/');
if ($downloadFolder !== '/download' && strpos($downloadFolder . '/', '/download/') !== 0) {
    return [
        'action' => 'continue',
        'status' => 'skipped',
        'message' => "Not in /download folder (path: {$filePath}) - no action needed",
    ];
}

// Build absolute paths
// Note: $filePath is the folder, $fileName is the actual file name
// dirname levels: __DIR__ = .../private/hooks/onUpload
//   dirname(__DIR__, 1) = .../private/hooks
//   dirname(__DIR__, 2) = .../private
//   dirname(__DIR__, 3) = .../filegator (root)
$repositoryPath = dirname(__DIR__, 3) . '/repository';
$relativeFilePath = rtrim($filePath, '/') . '/' . $fileName;

// Construct source path
$sourcePath = $repositoryPath . $homeDir . $relativeFilePath;

// Debug logging
error_log("[Move Hook] repositoryPath: {$repositoryPath}");
error_log("[Move Hook] homeDir: {$homeDir}");
error_log("[Move Hook] filePath: {$filePath}");
error_log("[Move Hook] fileName: {$fileName}");
error_log("[Move Hook] relativeFilePath: {$relativeFilePath}");
error_log("[Move Hook] sourcePath (before realpath): {$sourcePath}");

$sourcePath = realpath($sourcePath);
error_log("[Move Hook] sourcePath (after realpath): " . ($sourcePath ?: 'FALSE'));

// Validate source path exists and is within repository
if (!$sourcePath || !file_exists($sourcePath)) {
    error_log("[Move Hook] Source file not found: {$repositoryPath}{$homeDir}{$relativeFilePath}");
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => "Source file not found: {$relativeFilePath}",
        'file_path' => $relativeFilePath,
    ];
}

// Security check: ensure file is within repository
if (strpos($sourcePath, realpath($repositoryPath)) !== 0) {
    error_log("[Hook] Security violation: File path outside repository: {$sourcePath}");
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Invalid source path - security violation',
    ];
}

// Prepare destination directory
$destDir = $repositoryPath . $homeDir . '/upload';
if (!is_dir($destDir)) {
    if (!@mkdir($destDir, 0755, true)) {
        error_log("[Hook] Failed to create destination directory: {$destDir}");
        return [
            'action' => 'continue',
            'status' => 'error',
            'message' => 'Failed to create /upload directory',
        ];
    }
}

// Handle filename conflicts by appending timestamp
$destPath = $destDir . '/' . $fileName;
if (file_exists($destPath)) {
    $timestamp = time();
    $fileInfo = pathinfo($fileName);
    $baseName = $fileInfo['filename'];
    $extension = isset($fileInfo['extension']) ? '.' . $fileInfo['extension'] : '';
    $fileName = $baseName . '_' . $timestamp . $extension;
    $destPath = $destDir . '/' . $fileName;

    // Log filename conflict resolution
    $logMessage = sprintf(
        "[%s] Filename conflict resolved for '%s' - renamed to '%s'\n",
        date('Y-m-d H:i:s'),
        $hookData['file_name'],
        $fileName
    );
    @file_put_contents(dirname(__DIR__, 3) . '/logs/hooks.log', $logMessage, FILE_APPEND);
}

// Move the file
$moveSuccess = @rename($sourcePath, $destPath);

if (!$moveSuccess) {
    error_log("[Hook] Failed to move file: {$sourcePath} -> {$destPath}");
    return [
        'action' => 'continue',
        'status' => 'error',
        'message' => 'Failed to move file to /upload directory',
        'source' => $filePath,
    ];
}

// Log the successful move
$auditConfig = $config['audit'] ?? [];
if ($auditConfig['enabled'] ?? true) {
    $auditLog = $auditConfig['log_file'] ?? dirname(__DIR__, 3) . '/logs/audit.log';
    $logMessage = sprintf(
        "[%s] FILE_MOVED: %s -> /upload/%s (user: %s, size: %d bytes)\n",
        date('Y-m-d H:i:s'),
        $filePath,
        $fileName,
        $user,
        $fileSize
    );
    @file_put_contents($auditLog, $logMessage, FILE_APPEND);
}

// Return success result
return [
    'action' => 'continue',
    'status' => 'success',
    'operation' => 'file_moved',
    'from' => $filePath,
    'to' => '/upload/' . $fileName,
    'user' => $user,
    'file_size' => $fileSize,
    'message' => sprintf(
        'File moved from %s to /upload/%s',
        $filePath,
        $fileName
    ),
];
