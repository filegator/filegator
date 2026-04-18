# Trend Micro File Scanning Example - Architecture Design

## Executive Summary

This document outlines the complete architecture for a production-ready Trend Micro file scanning deployment using FileGator. The solution implements a three-zone security architecture where files uploaded from external gateways are automatically scanned before being made available to internal users.

**Target Deployment**: Self-contained example package for FileGator integrators

**Key Features**:
- IP-based access control with three distinct zones
- Automated file movement through security pipeline
- Trend Micro Cloud One File Security integration
- Email notification on malware detection
- CLI installer with configuration validation

---

## Table of Contents

1. [Use Case Overview](#use-case-overview)
2. [System Architecture](#system-architecture)
3. [Directory Structure](#directory-structure)
4. [Configuration Design](#configuration-design)
5. [ACL Design](#acl-design)
6. [Hook Scripts Design](#hook-scripts-design)
7. [Installer Design](#installer-design)
8. [Security Considerations](#security-considerations)
9. [Deployment Flow](#deployment-flow)
10. [Testing Strategy](#testing-strategy)

---

## 1. Use Case Overview

### Business Context

A company receives files from external partners via a gateway server (reverse proxy). Files must be scanned for malware before internal staff can access them. The workflow ensures:

1. External partners can only upload files (no visibility into file system)
2. All uploads are automatically scanned by Trend Micro
3. Clean files are made available to internal staff
4. Malware is quarantined and admin is notified

### User Profile

**Username**: `john`
**Home Directory**: `/`
**Role**: Standard user (not admin)

### Network Zones

| Zone | IP Address | Description |
|------|-----------|-------------|
| **Gateway** | 192.168.1.100 (configurable) | External access proxy/reverse proxy |
| **Internal** | All other local IPs | Corporate network users |
| **Untrusted** | Public internet | Blocked entirely |

### Folder Access Matrix

| Folder | Purpose | Gateway Access | Internal Access |
|--------|---------|----------------|-----------------|
| `/` (home) | Root directory | Read only | Read only |
| `/upload` | Staging zone for external uploads | Upload, Read, Download | Hidden (no access) |
| `/scanned` | Clean files ready for download | Hidden (no access) | Read, Download |
| `/download` | File drop for internal users | Hidden (no access) | Read, Upload |

### Workflow Pipeline

```
External Upload Flow:
Gateway → /upload → Trend Micro scan → /scanned (clean) or delete (malware)

Internal Upload Flow:
Internal Network → /download → Hook moves to /upload → Trend Micro scan → /scanned (clean) or delete (malware)
```

---

## 2. System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    FileGator Instance                        │
│                                                              │
│  ┌────────────┐    ┌────────────┐    ┌────────────┐        │
│  │  Gateway   │    │  Internal  │    │   Hooks    │        │
│  │  Access    │    │  Network   │    │  Engine    │        │
│  │            │    │            │    │            │        │
│  │ /download  │    │ /download  │    │ Automated  │        │
│  │ (upload)   │    │ (upload)   │    │ Pipeline   │        │
│  │            │    │            │    │            │        │
│  │ /upload    │    │            │    │            │        │
│  │ (visible)  │    │ (hidden)   │    │            │        │
│  └─────┬──────┘    └─────┬──────┘    └──────┬─────┘        │
│        │                 │                   │              │
│        │                 │                   │              │
│        └─────────────────┴───────────────────┘              │
│                          │                                  │
│                          ▼                                  │
│              ┌──────────────────────┐                       │
│              │   Security Pipeline   │                       │
│              │                      │                       │
│              │  1. Move to /upload  │                       │
│              │  2. Scan with TM     │                       │
│              │  3. Clean → /scanned │                       │
│              │  4. Malware → Delete │                       │
│              └──────────────────────┘                       │
│                          │                                  │
│                          ▼                                  │
│              ┌──────────────────────┐                       │
│              │   Trend Micro API    │                       │
│              │   File Security      │                       │
│              └──────────────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

### Component Interactions

```
┌──────────────┐
│ Upload Event │
└──────┬───────┘
       │
       ▼
┌────────────────────────┐
│ Hook: onUpload         │
│ Location: /download    │
└──────┬─────────────────┘
       │
       ▼
┌────────────────────────┐      ┌─────────────────────┐
│ Move File Operation    │─────►│ Destination:        │
│ From: /download/file   │      │ /upload/file        │
└──────┬─────────────────┘      └─────────────────────┘
       │
       │ Triggers second hook
       ▼
┌────────────────────────┐
│ Hook: onUpload         │
│ Location: /upload      │
└──────┬─────────────────┘
       │
       ▼
┌────────────────────────┐      ┌─────────────────────┐
│ Trend Micro Scan       │─────►│ API Call            │
│                        │      │ Wait for result     │
└──────┬─────────────────┘      └─────────────────────┘
       │
       ├─────────────┬──────────────┐
       │             │              │
       ▼             ▼              ▼
   ┌──────┐    ┌──────────┐   ┌─────────┐
   │ Clean│    │ Malware  │   │  Error  │
   └───┬──┘    └─────┬────┘   └────┬────┘
       │             │              │
       ▼             ▼              ▼
   ┌──────────┐  ┌─────────┐   ┌────────────┐
   │ Move to  │  │ Delete  │   │ Quarantine │
   │ /scanned │  │ & Email │   │ & Log      │
   └──────────┘  └─────────┘   └────────────┘
```

### Data Flow Diagram

```
External Partner                Internal User
      │                              │
      │ HTTPS (via Gateway)          │ HTTPS (Direct)
      ▼                              ▼
┌─────────────────────────────────────────┐
│     FileGator with Path ACL             │
├─────────────────────────────────────────┤
│                                         │
│  Gateway IP: 192.168.1.100              │
│  ┌──────────────────────────┐           │
│  │ Can access: /download    │           │
│  │            /upload       │           │
│  │ Can upload files         │           │
│  └──────────────────────────┘           │
│                                         │
│  Internal IPs: 192.168.1.0/24           │
│  ┌──────────────────────────┐           │
│  │ Can access: /download    │           │
│  │            /scanned      │           │
│  │ Can upload & download    │           │
│  └──────────────────────────┘           │
│                                         │
└─────────────────┬───────────────────────┘
                  │
                  ▼
         ┌────────────────┐
         │ Hook Pipeline  │
         └────────┬───────┘
                  │
                  ▼
         ┌────────────────┐
         │ Trend Micro    │
         │ Cloud One API  │
         └────────────────┘
```

---

## 3. Directory Structure

### Example Package Structure

```
docs/examples/trend-micro-file-scanning/
├── DESIGN.md                           # This file
├── README.md                           # User-facing documentation
├── install.php                         # CLI installer script
├── config/
│   ├── acl_config.php.template        # ACL configuration template
│   ├── hooks_config.php.template      # Hooks configuration template
│   └── users.json.template            # User configuration template
├── hooks/
│   ├── onUpload/
│   │   ├── 01_move_from_download.php # Move files from /download to /upload
│   │   ├── 02_scan_upload.php        # Scan files in /upload with Trend Micro
│   │   └── README.md                  # Hook documentation
│   └── config.php.template            # Hooks config with Trend Micro settings
├── scripts/
│   ├── test_installation.php          # Validation script
│   ├── simulate_upload.php            # Testing tool
│   └── check_tm_api.php               # Trend Micro API connectivity test
├── lib/
│   └── TrendMicroScanner.php          # Trend Micro API wrapper class
└── docs/
    ├── CONFIGURATION.md               # Detailed configuration guide
    ├── TROUBLESHOOTING.md             # Common issues and solutions
    └── API_INTEGRATION.md             # Trend Micro API integration guide
```

### FileGator Installation Integration

After running installer, files are placed in:

```
/mnt/ai/filegator/
├── private/
│   ├── acl_config.php                 # Generated from template
│   ├── hooks/
│   │   ├── config.php                 # Generated, includes TM settings
│   │   └── onUpload/
│   │       ├── 01_move_from_download.php
│   │       └── 02_scan_upload.php
│   └── users.json                     # Updated with 'john' user
├── repository/                        # File storage
│   └── john/                          # John's home directory
│       ├── upload/                    # Visible to gateway only
│       ├── scanned/                   # Visible to internal only
│       └── download/                  # Upload destination for both zones
└── configuration.php                  # Updated to enable hooks & ACL
```

---

## 4. Configuration Design

### 4.1 Main Configuration (configuration.php)

The installer will update the existing `configuration.php` to enable required services:

```php
// Additions to existing configuration.php
'services' => [
    // ... existing services ...

    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => [
            'enabled' => true,  // REQUIRED: Must be true to enable PathACL
            'acl_config_file' => __DIR__.'/private/acl_config.php',
        ],
    ],

    'Filegator\Services\Hooks\HooksInterface' => [
        'handler' => '\Filegator\Services\Hooks\Hooks',
        'config' => [
            'enabled' => true,  // Enabled by installer
            'hooks_path' => __DIR__.'/private/hooks',
            'timeout' => 60,  // Increased for API calls
            'async' => false,
        ],
    ],
],
```

### 4.2 Hooks Configuration Template

**File**: `config/hooks_config.php.template`

```php
<?php
/**
 * Trend Micro File Scanning - Hooks Configuration
 *
 * This file contains configuration for the automated file scanning pipeline.
 * Generated by install.php - DO NOT edit API keys here, use environment variables.
 */

return [
    'global' => [
        'debug' => false,
        'log_file' => __DIR__ . '/../logs/hooks.log',
        'http_timeout' => 30,
    ],

    'trend_micro' => [
        'enabled' => true,

        // API Configuration
        'api_key' => getenv('TREND_MICRO_API_KEY') ?: '',
        'region' => getenv('TREND_MICRO_REGION') ?: 'us-1',  // us-1, eu-1, sg-1, au-1

        // API Endpoints by region
        'endpoints' => [
            'us-1' => 'https://filesecurity.api.trendmicro.com/v1',
            'eu-1' => 'https://filesecurity.eu-1.api.trendmicro.com/v1',
            'sg-1' => 'https://filesecurity.sg-1.api.trendmicro.com/v1',
            'au-1' => 'https://filesecurity.au-1.api.trendmicro.com/v1',
        ],

        // Scan settings
        'scan_timeout' => 120,  // Maximum time to wait for scan result (seconds)
        'max_file_size' => 100 * 1024 * 1024,  // 100MB limit
        'skip_extensions' => [],  // Extensions to skip (empty = scan all)

        // Action on malware detection
        'on_malware' => [
            'action' => 'delete',  // 'delete' or 'quarantine'
            'quarantine_dir' => __DIR__ . '/../quarantine',
            'log_file' => __DIR__ . '/../logs/malware_detections.log',
        ],

        // Action on scan errors
        'on_error' => [
            'action' => 'quarantine',  // 'quarantine', 'allow', or 'delete'
            'log_file' => __DIR__ . '/../logs/scan_errors.log',
        ],
    ],

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
                'address' => getenv('ADMIN_EMAIL') ?: 'filegator@example.com',
                'name' => 'FileGator Security',
            ],

            // Who receives malware alerts
            'malware_recipients' => [
                getenv('ADMIN_EMAIL') ?: 'admin@example.com',
            ],
        ],
    ],

    'file_movement' => [
        // Settings for automatic file movement
        'download_to_upload' => [
            'enabled' => true,
            'delete_source' => true,  // Remove from /download after move
        ],

        'scanned_destination' => [
            'folder' => '/scanned',
            'preserve_structure' => false,  // Flatten directory structure
        ],
    ],

    'audit' => [
        'enabled' => true,
        'log_file' => __DIR__ . '/../logs/audit.log',
        'events' => [
            'upload' => true,
            'scan_clean' => true,
            'scan_malware' => true,
            'scan_error' => true,
            'file_moved' => true,
        ],
    ],
];
```

### 4.3 Environment Variables

**File**: `.env.example` (created by installer)

```bash
# Trend Micro Cloud One File Security
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_REGION=us-1

# Email Configuration (for malware alerts)
ADMIN_EMAIL=admin@example.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password

# Gateway IP Address
GATEWAY_IP=192.168.1.100
```

---

## 5. ACL Design

### 5.1 ACL Configuration Template

**File**: `config/acl_config.php.template`

```php
<?php
/**
 * Path-Based ACL Configuration for Trend Micro File Scanning
 *
 * User: john
 * Gateway IP: {{GATEWAY_IP}} (replaced by installer)
 * Internal Network: All other local IPs
 *
 * Permission Matrix:
 * - / (home):     Gateway=Read, Internal=Read
 * - /upload:      Gateway=Upload+Read+Download, Internal=Hidden
 * - /scanned:     Gateway=Hidden, Internal=Read+Download
 * - /download:    Gateway=Hidden, Internal=Read+Upload
 */

return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'default_inherit' => true,
        'deny_overrides_allow' => true,
        'cache_enabled' => true,
        'cache_ttl' => 300,
    ],

    'groups' => [
        'gateway' => [],      // Will be checked via IP rules
        'internal' => [],     // Will be checked via IP rules
    ],

    'path_rules' => [
        // Root directory: Read-only access for both zones
        '/' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read'],
                    'priority' => 10,
                    'description' => 'Default read access to home directory',
                ],
            ],
        ],

        // /upload - Visible only to gateway IP
        '/upload' => [
            'inherit' => false,
            'rules' => [
                // Gateway: Upload, Read, Download
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'upload', 'download'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Gateway can view and download from upload folder',
                ],
                // Internal network: Explicitly denied (hidden)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{INTERNAL_NETWORK}}'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => [],  // No permissions = hidden
                    'priority' => 40,
                    'override_inherited' => true,
                    'description' => 'Internal network cannot see upload folder',
                ],
            ],
        ],

        // /scanned - Visible only to internal network
        '/scanned' => [
            'inherit' => false,
            'rules' => [
                // Internal network: Read and Download
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{INTERNAL_NETWORK}}'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => ['read', 'download'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Internal users can download scanned files',
                ],
                // Gateway: Explicitly denied (hidden)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => [],  // No permissions = hidden
                    'priority' => 40,
                    'override_inherited' => true,
                    'description' => 'Gateway cannot see scanned folder',
                ],
            ],
        ],

        // /download - Hidden from both, but upload allowed
        '/download' => [
            'inherit' => false,
            'rules' => [
                // Internal network: Read and Upload
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{INTERNAL_NETWORK}}'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => ['read', 'upload'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Internal users upload here for scanning',
                ],
                // Gateway: Read and Upload (also accessible from gateway)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'upload'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Gateway uploads here, files auto-moved to /upload',
                ],
            ],
        ],
    ],
];
```

### 5.2 ACL Logic Explanation

**Key Design Decisions**:

1. **Folder Visibility**: Folders without 'read' permission are effectively hidden in the UI
2. **Gateway Isolation**: Gateway IP can only see `/upload` for monitoring
3. **Internal Access**: Internal users can only download from `/scanned`
4. **Upload Pipeline**: Both zones upload to `/download`, hooks handle movement
5. **No Delete Permission**: Users cannot delete files (security policy)

**Permission Hierarchy**:
```
Priority 50 = Specific zone rules (higher priority)
Priority 40 = Deny rules for other zones
Priority 10 = Default read access to root
```

---

## 6. Hook Scripts Design

### 6.1 Hook #1: Move from Download to Upload

**File**: `hooks/onUpload/01_move_from_download.php`

**Purpose**: Automatically move files uploaded to `/download` into `/upload` for scanning

**Logic**:
```php
<?php
/**
 * Hook: Move files from /download to /upload
 *
 * Triggered: After file upload completes
 * Condition: File is in /download directory
 * Action: Move file to /upload directory
 * Result: Triggers second onUpload hook for scanning
 */

// Load configuration
$config = include dirname(__DIR__) . '/config.php';
$moveConfig = $config['file_movement']['download_to_upload'] ?? [];

// Check if this feature is enabled
if (!($moveConfig['enabled'] ?? true)) {
    return ['status' => 'skipped', 'message' => 'Auto-move disabled'];
}

// Build paths
$uploadPath = $hookData['file_path'] ?? '';
$fileName = $hookData['file_name'] ?? '';
$homeDir = $hookData['home_dir'] ?? '/';

// Only process files uploaded to /download
if (strpos($uploadPath, '/download/') !== 0) {
    return ['status' => 'skipped', 'message' => 'Not in download folder'];
}

// Calculate source and destination
$repoPath = dirname(__DIR__, 3) . '/repository';
$sourcePath = realpath($repoPath . $homeDir . $uploadPath);
$destDir = $repoPath . $homeDir . '/upload';
$destPath = $destDir . '/' . $fileName;

// Validate paths
if (!$sourcePath || strpos($sourcePath, realpath($repoPath)) !== 0) {
    error_log("[Hook] Invalid source path: $sourcePath");
    return ['status' => 'error', 'message' => 'Invalid source path'];
}

// Create destination directory if needed
if (!is_dir($destDir)) {
    mkdir($destDir, 0755, true);
}

// Move file
$success = rename($sourcePath, $destPath);

if ($success) {
    // Log the move
    $auditLog = $config['audit']['log_file'] ?? '';
    if ($auditLog) {
        $logMsg = sprintf(
            "[%s] FILE_MOVED: %s -> %s (user: %s)\n",
            date('Y-m-d H:i:s'),
            $uploadPath,
            '/upload/' . $fileName,
            $hookData['user'] ?? 'unknown'
        );
        file_put_contents($auditLog, $logMsg, FILE_APPEND);
    }

    return [
        'status' => 'success',
        'action' => 'moved',
        'from' => $uploadPath,
        'to' => '/upload/' . $fileName,
    ];
} else {
    error_log("[Hook] Failed to move file: $sourcePath -> $destPath");
    return ['status' => 'error', 'message' => 'Move failed'];
}
```

### 6.2 Hook #2: Trend Micro Scan

**File**: `hooks/onUpload/02_scan_upload.php`

**Purpose**: Scan files in `/upload` with Trend Micro, move clean files to `/scanned`

**Logic**:
```php
<?php
/**
 * Hook: Trend Micro File Scan
 *
 * Triggered: After file upload to /upload (via move from hook #1)
 * Action: Scan file using Trend Micro Cloud One File Security
 * Clean: Move to /scanned
 * Malware: Delete and email admin
 * Error: Quarantine and log
 */

// Load configuration
$config = include dirname(__DIR__) . '/config.php';
$tmConfig = $config['trend_micro'] ?? [];

// Check if scanning is enabled
if (!($tmConfig['enabled'] ?? true)) {
    return ['status' => 'skipped', 'message' => 'TM scanning disabled'];
}

// Only process files in /upload directory
$uploadPath = $hookData['file_path'] ?? '';
if (strpos($uploadPath, '/upload/') !== 0) {
    return ['status' => 'skipped', 'message' => 'Not in upload folder'];
}

// Build file path
$fileName = $hookData['file_name'] ?? '';
$homeDir = $hookData['home_dir'] ?? '/';
$repoPath = dirname(__DIR__, 3) . '/repository';
$fullPath = realpath($repoPath . $homeDir . $uploadPath);

// Validate path
if (!$fullPath || !file_exists($fullPath)) {
    return ['status' => 'error', 'message' => 'File not found'];
}

// Check file size
$fileSize = filesize($fullPath);
$maxSize = $tmConfig['max_file_size'] ?? (100 * 1024 * 1024);
if ($fileSize > $maxSize) {
    return ['status' => 'error', 'message' => 'File too large for scanning'];
}

// Load Trend Micro scanner library via Composer autoload
require_once dirname(__DIR__, 2) . '/vendor/autoload.php';

$scanner = new \TrendMicro\FileScanner([
    'api_key' => $tmConfig['api_key'] ?? '',
    'region' => $tmConfig['region'] ?? 'us-1',
    'timeout' => $tmConfig['scan_timeout'] ?? 120,
]);

// Perform scan
try {
    $result = $scanner->scanFile($fullPath);

    // Log scan result
    $auditLog = $config['audit']['log_file'] ?? '';
    if ($auditLog) {
        $logMsg = sprintf(
            "[%s] SCAN_RESULT: %s - Status: %s, Malware: %s\n",
            date('Y-m-d H:i:s'),
            $fileName,
            $result['status'],
            $result['malware_found'] ? 'YES' : 'NO'
        );
        file_put_contents($auditLog, $logMsg, FILE_APPEND);
    }

    // Handle scan result
    if ($result['status'] === 'clean') {
        // Move to /scanned directory
        $scannedDir = $repoPath . $homeDir . '/scanned';
        if (!is_dir($scannedDir)) {
            mkdir($scannedDir, 0755, true);
        }

        $destPath = $scannedDir . '/' . $fileName;
        rename($fullPath, $destPath);

        return [
            'status' => 'success',
            'action' => 'moved_to_scanned',
            'scan_result' => 'clean',
            'destination' => '/scanned/' . $fileName,
        ];

    } elseif ($result['status'] === 'malware') {
        // Log malware detection
        $malwareLog = $tmConfig['on_malware']['log_file'] ?? '';
        if ($malwareLog) {
            $logMsg = sprintf(
                "[%s] MALWARE_DETECTED: %s - Threat: %s\n",
                date('Y-m-d H:i:s'),
                $fileName,
                $result['threat_name'] ?? 'Unknown'
            );
            file_put_contents($malwareLog, $logMsg, FILE_APPEND);
        }

        // Delete or quarantine
        $action = $tmConfig['on_malware']['action'] ?? 'delete';
        if ($action === 'quarantine') {
            $quarantineDir = $tmConfig['on_malware']['quarantine_dir'] ?? '';
            if ($quarantineDir && !is_dir($quarantineDir)) {
                mkdir($quarantineDir, 0700, true);
            }
            rename($fullPath, $quarantineDir . '/' . $fileName . '.quarantine');
        } else {
            unlink($fullPath);
        }

        // Send email notification
        sendMalwareAlert($fileName, $result, $config);

        return [
            'status' => 'success',
            'action' => $action,
            'scan_result' => 'malware',
            'threat_name' => $result['threat_name'] ?? 'Unknown',
        ];

    } else {
        // Scan error - quarantine for manual review
        $errorLog = $tmConfig['on_error']['log_file'] ?? '';
        if ($errorLog) {
            $logMsg = sprintf(
                "[%s] SCAN_ERROR: %s - Error: %s\n",
                date('Y-m-d H:i:s'),
                $fileName,
                $result['error'] ?? 'Unknown error'
            );
            file_put_contents($errorLog, $logMsg, FILE_APPEND);
        }

        return [
            'status' => 'error',
            'action' => 'scan_failed',
            'error' => $result['error'] ?? 'Unknown error',
        ];
    }

} catch (\Exception $e) {
    error_log("[Hook] Scan exception: " . $e->getMessage());
    return ['status' => 'error', 'message' => $e->getMessage()];
}

/**
 * Send email alert for malware detection
 */
function sendMalwareAlert($fileName, $scanResult, $config) {
    $emailConfig = $config['notifications']['email'] ?? [];
    if (!($emailConfig['enabled'] ?? false)) {
        return;
    }

    // Email implementation here (using PHPMailer or similar)
    // See email-notification.md example for full implementation
}
```

### 6.3 Trend Micro API Wrapper Library

**Package**: `trendandrew/file-security-sdk` (install via Composer)

**Repository**: https://github.com/trendandrew/tm-v1-fs-php-sdk

**Purpose**: Encapsulate Trend Micro Vision One File Security API

```php
<?php
namespace TrendMicro;

/**
 * Trend Micro Cloud One File Security Scanner
 *
 * API Documentation: https://cloudone.trendmicro.com/docs/file-storage-security/
 */
class FileScanner {
    private $apiKey;
    private $endpoint;
    private $timeout;

    public function __construct($config) {
        $this->apiKey = $config['api_key'] ?? '';
        $this->timeout = $config['timeout'] ?? 120;

        $region = $config['region'] ?? 'us-1';
        $endpoints = [
            'us-1' => 'https://filesecurity.api.trendmicro.com/v1',
            'eu-1' => 'https://filesecurity.eu-1.api.trendmicro.com/v1',
            'sg-1' => 'https://filesecurity.sg-1.api.trendmicro.com/v1',
            'au-1' => 'https://filesecurity.au-1.api.trendmicro.com/v1',
        ];
        $this->endpoint = $endpoints[$region] ?? $endpoints['us-1'];
    }

    /**
     * Scan a file for malware
     *
     * @param string $filePath Absolute path to file
     * @return array Result with keys: status, malware_found, threat_name, error
     */
    public function scanFile($filePath) {
        if (!file_exists($filePath)) {
            return ['status' => 'error', 'error' => 'File not found'];
        }

        // Upload file for scanning
        $uploadResult = $this->uploadFile($filePath);
        if (!$uploadResult['success']) {
            return [
                'status' => 'error',
                'error' => $uploadResult['error'] ?? 'Upload failed'
            ];
        }

        $scanId = $uploadResult['scan_id'];

        // Poll for scan result
        $result = $this->waitForScanResult($scanId);

        return $result;
    }

    /**
     * Upload file to Trend Micro API
     */
    private function uploadFile($filePath) {
        $url = $this->endpoint . '/scan';

        $ch = curl_init();
        curl_setopt_array($ch, [
            CURLOPT_URL => $url,
            CURLOPT_POST => true,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => $this->timeout,
            CURLOPT_HTTPHEADER => [
                'Authorization: Bearer ' . $this->apiKey,
                'Content-Type: application/octet-stream',
            ],
            CURLOPT_POSTFIELDS => file_get_contents($filePath),
        ]);

        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($httpCode !== 200 && $httpCode !== 201) {
            return [
                'success' => false,
                'error' => "HTTP $httpCode: $response"
            ];
        }

        $data = json_decode($response, true);
        return [
            'success' => true,
            'scan_id' => $data['scan_id'] ?? null,
        ];
    }

    /**
     * Poll for scan result
     */
    private function waitForScanResult($scanId) {
        $url = $this->endpoint . '/scan/' . $scanId;
        $maxAttempts = 60;  // 60 attempts * 2 seconds = 2 minutes max
        $attempt = 0;

        while ($attempt < $maxAttempts) {
            sleep(2);  // Wait 2 seconds between polls

            $ch = curl_init();
            curl_setopt_array($ch, [
                CURLOPT_URL => $url,
                CURLOPT_RETURNTRANSFER => true,
                CURLOPT_TIMEOUT => 10,
                CURLOPT_HTTPHEADER => [
                    'Authorization: Bearer ' . $this->apiKey,
                ],
            ]);

            $response = curl_exec($ch);
            $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
            curl_close($ch);

            if ($httpCode !== 200) {
                return [
                    'status' => 'error',
                    'error' => "HTTP $httpCode: $response"
                ];
            }

            $data = json_decode($response, true);
            $scanStatus = $data['status'] ?? '';

            if ($scanStatus === 'completed') {
                $malwareFound = $data['malware_found'] ?? false;
                return [
                    'status' => $malwareFound ? 'malware' : 'clean',
                    'malware_found' => $malwareFound,
                    'threat_name' => $data['threat_name'] ?? null,
                    'scan_time' => $data['scan_time'] ?? null,
                ];
            }

            $attempt++;
        }

        // Timeout waiting for result
        return [
            'status' => 'error',
            'error' => 'Scan timeout - result not available'
        ];
    }
}
```

---

## 7. Installer Design

### 7.1 CLI Installer Script

**File**: `install.php`

**Purpose**: Automated installation and configuration

**CLI Arguments**:
```bash
php install.php \
  --gateway-ip=192.168.1.100 \
  --internal-network=192.168.1.0/24 \
  --api-key=YOUR_TM_API_KEY \
  --admin-email=admin@example.com \
  --filegator-path=/var/www/filegator \
  [--region=us-1] \
  [--smtp-host=localhost] \
  [--smtp-port=587] \
  [--dry-run]
```

**Installer Logic**:

```php
<?php
/**
 * Trend Micro File Scanning Example - Installer
 *
 * Automates installation of hooks, ACL config, and user setup
 */

class TrendMicroInstaller {
    private $config = [];
    private $filegatorPath;
    private $dryRun = false;

    public function __construct($args) {
        $this->parseArguments($args);
        $this->validateEnvironment();
    }

    /**
     * Main installation routine
     */
    public function install() {
        echo "Trend Micro File Scanning Example - Installer\n";
        echo "==============================================\n\n";

        $this->displayConfiguration();

        if ($this->dryRun) {
            echo "\n[DRY RUN] No files will be modified\n\n";
        }

        // Installation steps
        $this->createDirectories();
        $this->installHookScripts();
        $this->installHooksConfig();
        $this->installACLConfig();
        $this->createUser();
        $this->updateMainConfig();
        $this->testTrendMicroAPI();
        $this->displayNextSteps();
    }

    /**
     * Parse command line arguments
     */
    private function parseArguments($args) {
        $options = getopt('', [
            'gateway-ip:',
            'internal-network:',
            'api-key:',
            'admin-email:',
            'filegator-path:',
            'region::',
            'smtp-host::',
            'smtp-port::',
            'dry-run::',
        ]);

        // Required parameters
        $required = ['gateway-ip', 'internal-network', 'api-key', 'admin-email', 'filegator-path'];
        foreach ($required as $param) {
            if (!isset($options[$param]) || empty($options[$param])) {
                $this->error("Missing required parameter: --$param");
            }
        }

        $this->config = [
            'gateway_ip' => $options['gateway-ip'],
            'internal_network' => $options['internal-network'],
            'api_key' => $options['api-key'],
            'admin_email' => $options['admin-email'],
            'region' => $options['region'] ?? 'us-1',
            'smtp_host' => $options['smtp-host'] ?? 'localhost',
            'smtp_port' => $options['smtp-port'] ?? 587,
        ];

        $this->filegatorPath = rtrim($options['filegator-path'], '/');
        $this->dryRun = isset($options['dry-run']);
    }

    /**
     * Validate FileGator installation
     */
    private function validateEnvironment() {
        // Check FileGator path exists
        if (!is_dir($this->filegatorPath)) {
            $this->error("FileGator path not found: {$this->filegatorPath}");
        }

        // Check critical files
        $requiredFiles = [
            $this->filegatorPath . '/configuration.php',
            $this->filegatorPath . '/private',
            $this->filegatorPath . '/repository',
        ];

        foreach ($requiredFiles as $file) {
            if (!file_exists($file)) {
                $this->error("Required file/directory not found: $file");
            }
        }

        // Check write permissions
        if (!is_writable($this->filegatorPath . '/private')) {
            $this->error("Private directory is not writable");
        }

        // Validate IP address format
        if (!filter_var($this->config['gateway_ip'], FILTER_VALIDATE_IP)) {
            $this->error("Invalid gateway IP: {$this->config['gateway_ip']}");
        }

        // Validate CIDR notation
        if (!preg_match('/^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/', $this->config['internal_network'])) {
            $this->error("Invalid internal network CIDR: {$this->config['internal_network']}");
        }

        echo "[OK] Environment validation passed\n\n";
    }

    /**
     * Create required directories in repository
     */
    private function createDirectories() {
        echo "Creating directories...\n";

        $dirs = [
            $this->filegatorPath . '/repository/upload',
            $this->filegatorPath . '/repository/scanned',
            $this->filegatorPath . '/repository/download',
            $this->filegatorPath . '/private/hooks/onUpload',
            $this->filegatorPath . '/private/quarantine',
        ];

        foreach ($dirs as $dir) {
            if (!$this->dryRun) {
                if (!is_dir($dir)) {
                    mkdir($dir, 0755, true);
                    echo "  Created: $dir\n";
                } else {
                    echo "  Exists: $dir\n";
                }
            } else {
                echo "  [DRY RUN] Would create: $dir\n";
            }
        }

        echo "\n";
    }

    /**
     * Install hook scripts
     */
    private function installHookScripts() {
        echo "Installing hook scripts...\n";

        $examplePath = __DIR__;
        $hookDestPath = $this->filegatorPath . '/private/hooks/onUpload';

        $hooks = [
            '01_move_from_download.php',
            '02_scan_upload.php',
        ];

        foreach ($hooks as $hook) {
            $source = $examplePath . '/hooks/onUpload/' . $hook;
            $dest = $hookDestPath . '/' . $hook;

            if (!$this->dryRun) {
                copy($source, $dest);
                chmod($dest, 0644);
                echo "  Installed: $hook\n";
            } else {
                echo "  [DRY RUN] Would install: $hook\n";
            }
        }

        echo "\n";
    }

    /**
     * Install hooks configuration
     */
    private function installHooksConfig() {
        echo "Installing hooks configuration...\n";

        $template = file_get_contents(__DIR__ . '/config/hooks_config.php.template');

        // No replacements needed (uses getenv())
        $dest = $this->filegatorPath . '/private/hooks/config.php';

        if (!$this->dryRun) {
            file_put_contents($dest, $template);
            chmod($dest, 0644);
            echo "  Created: $dest\n";
        } else {
            echo "  [DRY RUN] Would create: $dest\n";
        }

        echo "\n";
    }

    /**
     * Install ACL configuration
     */
    private function installACLConfig() {
        echo "Installing ACL configuration...\n";

        $template = file_get_contents(__DIR__ . '/config/acl_config.php.template');

        // Replace placeholders
        $replacements = [
            '{{GATEWAY_IP}}' => $this->config['gateway_ip'],
            '{{INTERNAL_NETWORK}}' => $this->config['internal_network'],
        ];

        $aclConfig = str_replace(
            array_keys($replacements),
            array_values($replacements),
            $template
        );

        $dest = $this->filegatorPath . '/private/acl_config.php';

        if (!$this->dryRun) {
            file_put_contents($dest, $aclConfig);
            chmod($dest, 0644);
            echo "  Created: $dest\n";
        } else {
            echo "  [DRY RUN] Would create: $dest\n";
        }

        echo "\n";
    }

    /**
     * Create user 'john'
     */
    private function createUser() {
        echo "Creating user 'john'...\n";

        $usersFile = $this->filegatorPath . '/private/users.json';
        $users = json_decode(file_get_contents($usersFile), true);

        // Find next available ID
        $nextId = max(array_keys($users)) + 1;

        // Create user john (password: changeme)
        $users[$nextId] = [
            'username' => 'john',
            'name' => 'John Doe',
            'role' => 'user',
            'homedir' => '/',
            'permissions' => 'read|upload|download',
            'password' => password_hash('changeme', PASSWORD_BCRYPT),
        ];

        if (!$this->dryRun) {
            file_put_contents($usersFile, json_encode($users, JSON_PRETTY_PRINT));
            echo "  Created user 'john' (password: changeme)\n";
            echo "  ** IMPORTANT: Change password after first login **\n";
        } else {
            echo "  [DRY RUN] Would create user 'john'\n";
        }

        echo "\n";
    }

    /**
     * Update main configuration.php
     */
    private function updateMainConfig() {
        echo "Updating configuration.php...\n";

        $configFile = $this->filegatorPath . '/configuration.php';
        $config = include $configFile;

        // Enable PathACL
        if (isset($config['services']['Filegator\Services\PathACL\PathACLInterface'])) {
            $config['services']['Filegator\Services\PathACL\PathACLInterface']['config']['enabled'] = true;
            echo "  Enabled PathACL service\n";
        }

        // Enable Hooks
        if (isset($config['services']['Filegator\Services\Hooks\HooksInterface'])) {
            $config['services']['Filegator\Services\Hooks\HooksInterface']['config']['enabled'] = true;
            $config['services']['Filegator\Services\Hooks\HooksInterface']['config']['timeout'] = 60;
            echo "  Enabled Hooks service\n";
        }

        if (!$this->dryRun) {
            // Write back to file (note: this requires careful formatting)
            echo "  ** Manual verification required for configuration.php **\n";
            echo "  ** Ensure PathACL and Hooks services are enabled **\n";
        }

        echo "\n";
    }

    /**
     * Test Trend Micro API connectivity
     */
    private function testTrendMicroAPI() {
        echo "Testing Trend Micro API connectivity...\n";

        // Load SDK via Composer autoload
        require_once __DIR__ . '/vendor/autoload.php';

        $scanner = new \TrendMicroScanner(
            $this->config['region'],
            $this->config['api_key'],
            10  // timeout
        );

        // Create test file
        $testFile = sys_get_temp_dir() . '/filegator_test.txt';
        file_put_contents($testFile, "Test file for Trend Micro API validation");

        try {
            if (!$this->dryRun) {
                $result = $scanner->scanFile($testFile);
                if ($result['status'] === 'clean') {
                    echo "  [OK] API connection successful\n";
                } else {
                    echo "  [WARNING] API returned unexpected status: {$result['status']}\n";
                }
            } else {
                echo "  [DRY RUN] Would test API connectivity\n";
            }
        } catch (\Exception $e) {
            echo "  [ERROR] API test failed: " . $e->getMessage() . "\n";
        } finally {
            @unlink($testFile);
        }

        echo "\n";
    }

    /**
     * Display next steps
     */
    private function displayNextSteps() {
        echo "Installation completed!\n";
        echo "======================\n\n";
        echo "Next steps:\n";
        echo "1. Create .env file with environment variables:\n";
        echo "   - TREND_MICRO_API_KEY={$this->config['api_key']}\n";
        echo "   - TREND_MICRO_REGION={$this->config['region']}\n";
        echo "   - ADMIN_EMAIL={$this->config['admin_email']}\n";
        echo "\n";
        echo "2. Update SMTP settings in .env for email notifications\n";
        echo "\n";
        echo "3. Test the installation:\n";
        echo "   php scripts/test_installation.php\n";
        echo "\n";
        echo "4. Login as user 'john' (password: changeme)\n";
        echo "   ** Change password immediately! **\n";
        echo "\n";
        echo "5. Verify ACL rules:\n";
        echo "   - From gateway IP: Access /upload folder\n";
        echo "   - From internal IP: Access /scanned folder\n";
        echo "\n";
        echo "6. Test upload workflow:\n";
        echo "   Upload file to /download -> Auto-scan -> Appears in /scanned\n";
        echo "\n";
    }

    /**
     * Display current configuration
     */
    private function displayConfiguration() {
        echo "Configuration:\n";
        echo "--------------\n";
        echo "FileGator Path:    {$this->filegatorPath}\n";
        echo "Gateway IP:        {$this->config['gateway_ip']}\n";
        echo "Internal Network:  {$this->config['internal_network']}\n";
        echo "Admin Email:       {$this->config['admin_email']}\n";
        echo "TM Region:         {$this->config['region']}\n";
        echo "SMTP Host:         {$this->config['smtp_host']}\n";
        echo "SMTP Port:         {$this->config['smtp_port']}\n";
        echo "\n";
    }

    /**
     * Display error and exit
     */
    private function error($message) {
        echo "\n[ERROR] $message\n\n";
        echo "Usage:\n";
        echo "  php install.php --gateway-ip=IP --internal-network=CIDR --api-key=KEY --admin-email=EMAIL --filegator-path=PATH\n";
        echo "\n";
        echo "Example:\n";
        echo "  php install.php \\\n";
        echo "    --gateway-ip=192.168.1.100 \\\n";
        echo "    --internal-network=192.168.1.0/24 \\\n";
        echo "    --api-key=YOUR_TM_API_KEY \\\n";
        echo "    --admin-email=admin@example.com \\\n";
        echo "    --filegator-path=/var/www/filegator\n";
        echo "\n";
        exit(1);
    }
}

// Run installer
if (php_sapi_name() === 'cli') {
    $installer = new TrendMicroInstaller($argv);
    $installer->install();
} else {
    echo "This script must be run from the command line.\n";
    exit(1);
}
```

---

## 8. Security Considerations

### 8.1 Threat Model

**Threats Addressed**:
1. Malware upload from external sources
2. Unauthorized access to scanned files
3. Data exfiltration via gateway
4. API key exposure
5. Path traversal attacks
6. IP spoofing

**Mitigations**:

| Threat | Mitigation |
|--------|-----------|
| Malware upload | Trend Micro scanning before internal access |
| Unauthorized access | Path-based ACL with IP validation |
| Data exfiltration | Gateway cannot download from /scanned |
| API key exposure | Environment variables, no hardcoded keys |
| Path traversal | realpath() validation in hooks |
| IP spoofing | Trusted proxy configuration in PathACL |

### 8.2 API Key Storage

**Best Practices**:
```bash
# Store in environment variables (not in code)
export TREND_MICRO_API_KEY="your-api-key-here"

# Or use .env file (not committed to git)
echo "TREND_MICRO_API_KEY=your-key" >> .env

# Load in PHP
$apiKey = getenv('TREND_MICRO_API_KEY');
```

**File Permissions**:
```bash
chmod 600 /var/www/filegator/.env
chown www-data:www-data /var/www/filegator/.env
```

### 8.3 Network Security

**Reverse Proxy Configuration** (nginx example):

```nginx
# Gateway access only via reverse proxy
upstream filegator_backend {
    server 127.0.0.1:8080;
}

server {
    listen 443 ssl;
    server_name gateway.example.com;

    ssl_certificate /etc/ssl/certs/gateway.crt;
    ssl_certificate_key /etc/ssl/private/gateway.key;

    location / {
        proxy_pass http://filegator_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # FileGator will see this as gateway IP
        proxy_set_header X-Forwarded-For 192.168.1.100;
    }
}
```

### 8.4 Audit Logging

All security events are logged:
- File uploads
- Scan results (clean/malware)
- File movements
- Malware detections
- Scan errors
- ACL denials (via PathACL)

**Log Locations**:
```
/private/logs/hooks.log          - General hook activity
/private/logs/audit.log          - Audit trail
/private/logs/malware_detections.log - Malware alerts
/private/logs/scan_errors.log    - Scan failures
```

---

## 9. Deployment Flow

### 9.1 Pre-Installation Checklist

- [ ] FileGator installed and working
- [ ] Trend Micro Cloud One account created
- [ ] API key obtained from Trend Micro console
- [ ] Gateway IP address identified
- [ ] Internal network CIDR range determined
- [ ] SMTP server available for email alerts
- [ ] Admin email address confirmed

### 9.2 Installation Steps

```bash
# 1. Download example package
cd /tmp
git clone https://github.com/filegator/filegator.git
cd filegator/docs/examples/trend-micro-file-scanning

# 2. Run installer
php install.php \
  --gateway-ip=192.168.1.100 \
  --internal-network=192.168.1.0/24 \
  --api-key=YOUR_TM_API_KEY \
  --admin-email=admin@example.com \
  --filegator-path=/var/www/filegator \
  --region=us-1

# 3. Create .env file
cat > /var/www/filegator/.env << EOF
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_REGION=us-1
ADMIN_EMAIL=admin@example.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password
GATEWAY_IP=192.168.1.100
EOF

chmod 600 /var/www/filegator/.env

# 4. Test installation
cd /var/www/filegator/docs/examples/trend-micro-file-scanning
php scripts/test_installation.php

# 5. Test upload workflow
php scripts/simulate_upload.php --file=test.txt --zone=gateway

# 6. Verify logs
tail -f /var/www/filegator/private/logs/audit.log
```

### 9.3 Post-Installation Verification

1. **ACL Verification**:
   - Login as 'john' from gateway IP
   - Verify `/upload` folder is visible
   - Verify `/scanned` folder is NOT visible
   - Login from internal IP
   - Verify `/scanned` folder is visible
   - Verify `/upload` folder is NOT visible

2. **Upload Pipeline Test**:
   - Upload file to `/download` from gateway
   - Verify file disappears from `/download`
   - Check `/upload` folder (from gateway)
   - Wait for scan completion
   - Check `/scanned` folder (from internal IP)

3. **Malware Detection Test**:
   - Download EICAR test file
   - Upload to `/download`
   - Verify file is deleted
   - Check malware detection log
   - Verify email alert received

---

## 10. Testing Strategy

### 10.1 Test Scripts

**File**: `scripts/test_installation.php`

```php
<?php
/**
 * Installation Validation Script
 */

echo "FileGator Trend Micro Installation Test\n";
echo "========================================\n\n";

$tests = [
    'check_directories',
    'check_hooks',
    'check_acl_config',
    'check_hooks_config',
    'check_env_variables',
    'check_permissions',
];

foreach ($tests as $test) {
    echo "Running: $test... ";
    $result = call_user_func($test);
    echo $result ? "PASS\n" : "FAIL\n";
}

function check_directories() {
    $dirs = [
        '/var/www/filegator/repository/upload',
        '/var/www/filegator/repository/scanned',
        '/var/www/filegator/repository/download',
        '/var/www/filegator/private/hooks/onUpload',
    ];
    foreach ($dirs as $dir) {
        if (!is_dir($dir)) return false;
    }
    return true;
}

// ... other test functions
```

### 10.2 Test Cases

| Test Case | Expected Result |
|-----------|----------------|
| Upload from gateway to /download | File appears in /upload |
| Upload clean file | File appears in /scanned |
| Upload EICAR test file | File deleted, email sent |
| Gateway access /scanned | 403 Forbidden |
| Internal access /upload | 403 Forbidden |
| API key invalid | Scan error logged |
| Large file (>100MB) | Rejected with error |

---

## Appendices

### A. Folder Visibility Matrix

| Folder | Gateway (192.168.1.100) | Internal (192.168.1.0/24) |
|--------|-------------------------|---------------------------|
| `/` | Read | Read |
| `/upload` | Read, Upload, Download | Hidden |
| `/scanned` | Hidden | Read, Download |
| `/download` | Read, Upload | Read, Upload |

### B. Hook Execution Order

```
1. User uploads file to /download
   ↓
2. onUpload hook triggered (01_move_from_download.php)
   - Checks if file is in /download
   - Moves to /upload
   - Returns success
   ↓
3. File move triggers second onUpload (02_scan_upload.php)
   - Checks if file is in /upload
   - Calls Trend Micro API
   - Waits for scan result
   - If clean: moves to /scanned
   - If malware: deletes & emails admin
   - Returns result
```

### C. API Integration References

- **Trend Micro Cloud One**: https://cloudone.trendmicro.com/
- **File Security API Docs**: https://cloudone.trendmicro.com/docs/file-storage-security/
- **API Authentication**: https://cloudone.trendmicro.com/docs/file-storage-security/api-reference/

### D. Glossary

- **Gateway IP**: External-facing reverse proxy IP address
- **Internal Network**: Corporate LAN CIDR range
- **PathACL**: FileGator's path-based access control system
- **Hook**: PHP script triggered on FileGator events
- **Quarantine**: Isolated directory for suspicious files
- **CIDR**: Classless Inter-Domain Routing notation for IP ranges

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-09 | Initial design document |

---

**Document Status**: Draft
**Last Updated**: 2025-12-09
**Author**: System Architecture Designer
**Reviewers**: [To be assigned]
