# Trend Vision One File Security PHP SDK

[![PHP Version](https://img.shields.io/badge/php-%3E%3D8.0-blue.svg)](https://www.php.net/)
[![Node.js Version](https://img.shields.io/badge/node-%3E%3D16.0-green.svg)](https://nodejs.org/)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

Trend Vision One File Security PHP SDK provides malware scanning capabilities for files using the Trend Micro Vision One File Security service (AMaaS - Anti-Malware as a Service). This SDK enables PHP applications to detect malware, viruses, and other threats in uploaded files before they are processed or stored.

This SDK uses a hybrid PHP/Node.js architecture that wraps the official [file-security-sdk](https://www.npmjs.com/package/file-security-sdk) npm package, providing PHP applications with access to Trend Micro's gRPC-based scanning service without requiring the PHP gRPC extension.

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Trend Vision One Setup](#trend-vision-one-setup)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Code Examples](#code-examples)
- [Error Handling](#error-handling)
- [Logging](#logging)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [License](#license)

## Features

- **File scanning** for malware, viruses, trojans, and other threats
- **Multiple region support** - US, EU, Japan, Singapore, Australia, India, Middle East
- **Comprehensive error handling** with specific exception types
- **PSR-4 autoloading** compatible
- **No PHP gRPC extension required** - uses embedded Node.js service
- **No external ports** - IPC via stdin/stdout (no firewall configuration needed)
- **PHP 8.0+ support**
- **Detailed scan results** including SHA256 hashes and threat information

## Prerequisites

### System Requirements

| Requirement | Version | Notes |
|-------------|---------|-------|
| PHP | 8.0 or later | With `proc_open` enabled |
| Node.js | 16.0 or later | For gRPC communication |
| npm | 8.0 or later | For package management |

### Trend Vision One Requirements

Before using this SDK, you need:

1. An active **Trend Vision One** account
2. API key with **Run file scan via SDK** permission
3. Account associated with the correct regional deployment

See [Trend Vision One Setup](#trend-vision-one-setup) for detailed instructions.

## Installation

### Using Composer (Recommended)

```bash
composer require trendmicro/file-security-sdk
```

After installing the Composer package, install the Node.js dependencies:

```bash
cd vendor/trendmicro/file-security-sdk/service
npm install
```

### Manual Installation

1. **Download the SDK** to your project:

```bash
git clone https://github.com/trendmicro/tm-v1-fs-php-sdk.git
cd tm-v1-fs-php-sdk
```

2. **Install Node.js dependencies:**

```bash
cd service
npm install
```

3. **Include the SDK in your PHP code:**

```php
require_once '/path/to/tm-v1-fs-php-sdk/TrendMicroScanner.php';
```

### Verify Installation

Test that the scanner service is working:

```bash
node service/scanner.js --test
```

Expected output:
```json
{
  "success": true,
  "status": "test",
  "message": "Scanner service is working",
  "version": "1.0.0",
  "nodeVersion": "v20.0.0",
  "regions": ["us", "eu", "jp", "sg", "au", "in", "me"]
}
```

## Quick Start

```php
<?php

require_once 'TrendMicroScanner.php';

// Initialize the scanner with your region and API key
$scanner = new TrendMicroScanner('us', 'your-api-key-here');

try {
    // Scan a file
    $result = $scanner->scanFile('/path/to/file.pdf');

    if ($result->isClean()) {
        echo "File is clean!\n";
        echo "Scan ID: " . $result->getScanId() . "\n";
        echo "SHA256: " . $result->getFileSha256() . "\n";
    } else {
        echo "Malware detected!\n";
        foreach ($result->getFoundMalwares() as $malware) {
            echo "  - " . $malware->getMalwareName() . "\n";
        }
    }
} catch (Exception $e) {
    echo "Scan failed: " . $e->getMessage() . "\n";
} finally {
    $scanner->close();
}
```

## Trend Vision One Setup

### Creating an API Key

To use this SDK, you need a Trend Vision One API key with the appropriate permissions.

1. **Log in** to the [Trend Vision One console](https://portal.xdr.trendmicro.com/)

2. Navigate to **Administration** → **API Keys**

3. Click **Add API Key**

4. Configure the API key:
   - **Name**: Enter a descriptive name (e.g., "File Security PHP SDK")
   - **Role**: Select or create a role with **Run file scan via SDK** permission
   - **Status**: Set to **Enabled**
   - **Expiration**: Set an appropriate expiration date

5. Click **Add** to create the key

6. **Copy and securely store** the API key - it will only be displayed once

> **Important**:
> - Verify your account is associated with the correct regional deployment
> - The API key region must match the region you specify in the SDK
> - Keep your API key secure and never commit it to version control

### Required Permissions

Your API key must have a role with the following permission:

| Permission | Description |
|------------|-------------|
| **Run file scan via SDK** | Allows scanning files using the File Security SDK |

### Regional Endpoints

Ensure your API key is created in the correct regional console:

| Region | Console URL |
|--------|-------------|
| United States | https://portal.xdr.trendmicro.com/ |
| Europe | https://portal.eu.xdr.trendmicro.com/ |
| Japan | https://portal.xdr.trendmicro.co.jp/ |
| Singapore | https://portal.sg.xdr.trendmicro.com/ |
| Australia | https://portal.au.xdr.trendmicro.com/ |
| India | https://portal.in.xdr.trendmicro.com/ |

## Configuration

### Supported Regions

The SDK supports all Trend Vision One regional deployments:

| Short Code | AWS Region | Location | gRPC Endpoint |
|------------|------------|----------|---------------|
| `us` | us-east-1 | United States | antimalware.us-east-1.cloudone.trendmicro.com:443 |
| `eu` | eu-central-1 | Europe (Germany) | antimalware.eu-central-1.cloudone.trendmicro.com:443 |
| `jp` | ap-northeast-1 | Japan | antimalware.ap-northeast-1.cloudone.trendmicro.com:443 |
| `sg` | ap-southeast-1 | Singapore | antimalware.ap-southeast-1.cloudone.trendmicro.com:443 |
| `au` | ap-southeast-2 | Australia | antimalware.ap-southeast-2.cloudone.trendmicro.com:443 |
| `in` | ap-south-1 | India | antimalware.ap-south-1.cloudone.trendmicro.com:443 |
| `me` | me-central-1 | Middle East | antimalware.me-central-1.cloudone.trendmicro.com:443 |

You can use either short codes (e.g., `us`, `au`) or full AWS region identifiers (e.g., `us-east-1`, `ap-southeast-2`).

Legacy region codes (`us-1`, `eu-1`, `jp-1`, `sg-1`, `au-1`, `in-1`) are also supported for backward compatibility.

### Constructor Parameters

```php
$scanner = new TrendMicroScanner(
    string $region,      // Required: Region identifier
    string $apiKey,      // Required: Vision One API key
    int $timeout = 300,  // Optional: Timeout in seconds (default: 300)
    bool $debug = false  // Optional: Enable debug logging (default: false)
);
```

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `$region` | string | Yes | - | Vision One region code or AWS region identifier |
| `$apiKey` | string | Yes | - | API key with file scan permissions |
| `$timeout` | int | No | 300 | Connection and scan timeout in seconds |
| `$debug` | bool | No | false | Enable debug logging to error_log |

### Environment Variables

The SDK can be configured using environment variables:

| Variable | Alternative | Description |
|----------|-------------|-------------|
| `TREND_MICRO_REGION` | `TM_REGION` | Default region |
| `TREND_MICRO_API_KEY` | `TM_API_KEY` | API key |
| `TREND_MICRO_TIMEOUT` | `TM_TIMEOUT` | Timeout in seconds |
| `TREND_MICRO_DEBUG` | `TM_DEBUG` | Enable debug mode (true/false) |

```php
// Create scanner from environment variables
$scanner = TrendMicroScanner::fromEnvironment();
```

### Configuration from .env File

```php
// Load configuration from .env file
$scanner = TrendMicroScanner::fromEnvFile('/path/to/.env');
```

Example `.env` file:

```env
# Trend Micro Vision One Configuration
TREND_MICRO_REGION=au
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_TIMEOUT=300
TREND_MICRO_DEBUG=false
```

## API Reference

### TrendMicroScanner Class

The main scanner class providing a simple interface for file scanning.

#### Constructor

```php
public function __construct(
    string $region,
    string $apiKey,
    int $timeout = 300,
    bool $debug = false
)
```

Creates a new scanner instance.

**Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `$region` | string | Yes | - | Region identifier (short code or AWS region) |
| `$apiKey` | string | Yes | - | Vision One API key |
| `$timeout` | int | No | 300 | Timeout in seconds |
| `$debug` | bool | No | false | Enable debug logging |

**Throws:**
- `InvalidArgumentException` - If region is invalid
- `RuntimeException` - If Node.js service is not installed

---

#### scanFile()

```php
public function scanFile(string $filePath, array $tags = []): ScanResult
```

Scans a file for malware.

**Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `$filePath` | string | Yes | Absolute or relative path to the file |
| `$tags` | array | No | Tags for categorization (max 8 tags, each max 63 chars) |

**Returns:** `ScanResult` object containing scan results

**Throws:**
- `InvalidArgumentException` - If file does not exist
- `AuthenticationException` - If API key is invalid
- `ConnectionException` - If connection to service fails
- `TimeoutException` - If scan times out
- `AmaasException` - For other scan errors

**Example:**
```php
$result = $scanner->scanFile('/uploads/document.pdf', ['upload', 'user-content']);
```

---

#### isFileClean()

```php
public function isFileClean(string $filePath): bool
```

Quick check if a file is clean (shorthand method).

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `$filePath` | string | Path to the file |

**Returns:** `true` if file is clean, `false` if malware detected

**Throws:** Same as `scanFile()`

**Example:**
```php
if ($scanner->isFileClean('/uploads/file.zip')) {
    // Process the file
}
```

---

#### testService()

```php
public function testService(): array
```

Tests if the scanner service is working properly.

**Returns:** Array with service status information including:
- `success` - Whether the test passed
- `status` - Status string ('test')
- `message` - Status message
- `version` - Service version
- `nodeVersion` - Node.js version
- `regions` - Available regions

**Throws:** `RuntimeException` if service is unavailable

---

#### setLogFile()

```php
public function setLogFile(string $logFile): self
```

Sets the log file path for scan results.

**Returns:** `$this` for method chaining

---

#### setDebug()

```php
public function setDebug(bool $enabled): self
```

Enables or disables debug mode.

**Returns:** `$this` for method chaining

---

#### setNodePath()

```php
public function setNodePath(string $path): self
```

Sets a custom path to the Node.js executable.

**Returns:** `$this` for method chaining

---

#### close()

```php
public function close(): void
```

Closes the scanner (cleanup). Kept for API compatibility with other SDKs.

---

#### Static Methods

##### fromEnvironment()

```php
public static function fromEnvironment(): self
```

Creates a scanner from environment variables.

**Throws:** `RuntimeException` if API key is not set

##### fromEnvFile()

```php
public static function fromEnvFile(string $envFile): self
```

Creates a scanner from a .env file.

**Throws:** `RuntimeException` if file not found or API key not set

##### getAvailableRegions()

```php
public static function getAvailableRegions(): array
```

Returns a list of all valid region identifiers.

---

### ScanResult Class

Contains the results of a file scan.

#### Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `isClean()` | bool | Returns `true` if no malware detected |
| `hasMalware()` | bool | Returns `true` if malware detected |
| `getScanResult()` | int | Scan result code (0 = clean, >0 = malware count) |
| `getScanId()` | ?string | Unique scan identifier |
| `getFileName()` | string | Scanned file name |
| `getFilePath()` | ?string | Original file path |
| `getFileSha1()` | ?string | SHA1 hash of the file |
| `getFileSha256()` | ?string | SHA256 hash of the file |
| `getFoundMalwares()` | Malware[] | Array of detected malware |
| `getMalwareCount()` | int | Number of malware instances found |
| `getScanDuration()` | int | Scan duration in milliseconds |
| `getScannerVersion()` | ?string | Version of the scan engine |
| `getRawResponse()` | array | Raw API response data |
| `toArray()` | array | Convert to associative array |

---

### Malware Class

Represents a detected malware threat.

#### Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `getMalwareName()` | string | Malware identifier/name |
| `getFileName()` | string | File where malware was detected |
| `getType()` | ?string | Malware type/category |
| `getFilter()` | ?string | Detection filter name |
| `getFilterDescription()` | ?string | Filter description |
| `toArray()` | array | Convert to associative array |

---

## Code Examples

### Basic File Scanning

```php
<?php

require_once 'TrendMicroScanner.php';

$scanner = new TrendMicroScanner('au', getenv('TM_API_KEY'));

try {
    $result = $scanner->scanFile('/uploads/document.pdf');

    if ($result->isClean()) {
        echo "File is safe to process.\n";
        echo "SHA256: " . $result->getFileSha256() . "\n";
        echo "Scan ID: " . $result->getScanId() . "\n";
    } else {
        echo "WARNING: Malware detected!\n";
        foreach ($result->getFoundMalwares() as $malware) {
            echo "  Threat: " . $malware->getMalwareName() . "\n";
            echo "  Type: " . ($malware->getType() ?? 'Unknown') . "\n";
        }
    }
} catch (Exception $e) {
    echo "Error: " . $e->getMessage() . "\n";
} finally {
    $scanner->close();
}
```

### File Upload Handler

```php
<?php

require_once 'TrendMicroScanner.php';

use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;

function handleFileUpload(array $uploadedFile): array
{
    // Validate upload
    if ($uploadedFile['error'] !== UPLOAD_ERR_OK) {
        return ['success' => false, 'error' => 'Upload failed'];
    }

    $tempPath = $uploadedFile['tmp_name'];
    $originalName = $uploadedFile['name'];

    // Initialize scanner
    try {
        $scanner = TrendMicroScanner::fromEnvironment();
    } catch (RuntimeException $e) {
        // Scanner not configured - log warning
        error_log("Malware scanner not configured: " . $e->getMessage());
        return processUpload($tempPath, $originalName);
    }

    // Scan the file
    try {
        $result = $scanner->scanFile($tempPath, ['upload', 'user-content']);

        if ($result->hasMalware()) {
            // Delete infected file
            unlink($tempPath);

            // Log the detection
            error_log(sprintf(
                "Malware detected in upload '%s': %s (Scan ID: %s)",
                $originalName,
                implode(', ', array_map(
                    fn($m) => $m->getMalwareName(),
                    $result->getFoundMalwares()
                )),
                $result->getScanId()
            ));

            return [
                'success' => false,
                'error' => 'File contains malware and was rejected',
                'scan_id' => $result->getScanId(),
            ];
        }

        // File is clean - proceed with upload
        return processUpload($tempPath, $originalName);

    } catch (AuthenticationException $e) {
        error_log("Scanner auth failed: " . $e->getMessage());
        return ['success' => false, 'error' => 'Security scan unavailable'];

    } catch (AmaasException $e) {
        error_log("Scan error: " . $e->getMessage());
        return ['success' => false, 'error' => 'Security scan failed'];
    } finally {
        $scanner->close();
    }
}

function processUpload(string $tempPath, string $name): array
{
    $destination = '/var/www/uploads/' . uniqid() . '_' . basename($name);
    move_uploaded_file($tempPath, $destination);
    return ['success' => true, 'path' => $destination];
}
```

### Batch Scanning with Logging

```php
<?php

require_once 'TrendMicroScanner.php';

$scanner = new TrendMicroScanner('eu', getenv('TM_API_KEY'), 600, true);
$scanner->setLogFile('/var/log/malware-scans.log');

$files = glob('/incoming/*.{pdf,doc,docx,xls,xlsx,zip}', GLOB_BRACE);
$results = ['clean' => [], 'infected' => [], 'errors' => []];

foreach ($files as $file) {
    echo "Scanning: " . basename($file) . "... ";

    try {
        $result = $scanner->scanFile($file);

        if ($result->isClean()) {
            $results['clean'][] = [
                'file' => basename($file),
                'sha256' => $result->getFileSha256(),
                'scan_id' => $result->getScanId(),
            ];
            rename($file, '/processed/' . basename($file));
            echo "CLEAN\n";
        } else {
            $threats = array_map(fn($m) => $m->getMalwareName(), $result->getFoundMalwares());
            $results['infected'][] = [
                'file' => basename($file),
                'threats' => $threats,
                'scan_id' => $result->getScanId(),
            ];
            rename($file, '/quarantine/' . basename($file));
            echo "INFECTED: " . implode(', ', $threats) . "\n";
        }
    } catch (Exception $e) {
        $results['errors'][] = [
            'file' => basename($file),
            'error' => $e->getMessage(),
        ];
        echo "ERROR: " . $e->getMessage() . "\n";
    }
}

$scanner->close();

// Output summary
echo "\n=== Scan Summary ===\n";
printf("Total scanned: %d\n", count($files));
printf("Clean: %d\n", count($results['clean']));
printf("Infected: %d\n", count($results['infected']));
printf("Errors: %d\n", count($results['errors']));
```

### Using with Laravel

**Service Class:**

```php
<?php
// app/Services/MalwareScannerService.php

namespace App\Services;

use TrendMicroScanner;
use TrendMicro\FileSecurity\Model\ScanResult;
use TrendMicro\FileSecurity\Exception\AmaasException;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Log;

class MalwareScannerService
{
    private TrendMicroScanner $scanner;

    public function __construct()
    {
        $this->scanner = new TrendMicroScanner(
            config('services.trendmicro.region', 'us'),
            config('services.trendmicro.api_key'),
            config('services.trendmicro.timeout', 300)
        );

        if ($logFile = config('services.trendmicro.log_file')) {
            $this->scanner->setLogFile($logFile);
        }
    }

    public function scan(UploadedFile $file): ScanResult
    {
        return $this->scanner->scanFile(
            $file->getRealPath(),
            ['laravel', 'upload']
        );
    }

    public function isSafe(UploadedFile $file): bool
    {
        try {
            return $this->scanner->isFileClean($file->getRealPath());
        } catch (AmaasException $e) {
            Log::error('Malware scan failed', [
                'file' => $file->getClientOriginalName(),
                'error' => $e->getMessage(),
            ]);
            return false; // Fail-safe: reject on scan error
        }
    }

    public function __destruct()
    {
        $this->scanner->close();
    }
}
```

**Configuration:**

```php
// config/services.php
return [
    // ... other services

    'trendmicro' => [
        'region' => env('TREND_MICRO_REGION', 'us'),
        'api_key' => env('TREND_MICRO_API_KEY'),
        'timeout' => env('TREND_MICRO_TIMEOUT', 300),
        'log_file' => storage_path('logs/malware-scans.log'),
    ],
];
```

**Service Provider:**

```php
// app/Providers/AppServiceProvider.php

use App\Services\MalwareScannerService;

public function register()
{
    $this->app->singleton(MalwareScannerService::class, function ($app) {
        return new MalwareScannerService();
    });
}
```

**Controller Usage:**

```php
use App\Services\MalwareScannerService;

public function upload(Request $request, MalwareScannerService $scanner)
{
    $file = $request->file('document');

    if (!$scanner->isSafe($file)) {
        return response()->json(['error' => 'File rejected: potential threat detected'], 422);
    }

    $path = $file->store('documents');
    return response()->json(['path' => $path]);
}
```

---

## Error Handling

### Exception Hierarchy

```
Exception
└── TrendMicro\FileSecurity\Exception\AmaasException
    ├── TrendMicro\FileSecurity\Exception\AuthenticationException
    ├── TrendMicro\FileSecurity\Exception\ConnectionException
    └── TrendMicro\FileSecurity\Exception\TimeoutException
```

### Common Errors

| Exception | Error Code | Cause | Resolution |
|-----------|------------|-------|------------|
| `InvalidArgumentException` | - | Invalid region code | Use a valid region: `us`, `eu`, `jp`, `sg`, `au`, `in`, `me` |
| `InvalidArgumentException` | - | File not found | Verify file path exists and is readable |
| `RuntimeException` | - | Node.js service not installed | Run `npm install` in the `service/` directory |
| `AuthenticationException` | AUTH_ERROR | Invalid API key | Verify API key has **Run file scan via SDK** permission |
| `AuthenticationException` | AUTH_ERROR | Wrong region | Ensure API key matches the configured region |
| `ConnectionException` | CONNECTION_ERROR | Network error | Check internet connectivity and firewall rules |
| `ConnectionException` | CONNECTION_ERROR | DNS resolution failed | Verify the region endpoint is accessible |
| `TimeoutException` | TIMEOUT | Scan took too long | Increase timeout or check network latency |

### Error Handling Example

```php
<?php

use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;
use TrendMicro\FileSecurity\Exception\ConnectionException;
use TrendMicro\FileSecurity\Exception\TimeoutException;

try {
    $scanner = new TrendMicroScanner('au', $apiKey);
    $result = $scanner->scanFile($filePath);

    // Process result...

} catch (AuthenticationException $e) {
    // Invalid API key or wrong region
    error_log("Authentication failed: " . $e->getMessage());
    http_response_code(500);
    echo json_encode(['error' => 'Security service configuration error']);

} catch (ConnectionException $e) {
    // Network or DNS issues
    error_log("Connection failed: " . $e->getMessage());
    http_response_code(503);
    echo json_encode(['error' => 'Security service temporarily unavailable']);

} catch (TimeoutException $e) {
    // Scan took too long
    error_log("Scan timeout: " . $e->getMessage());
    http_response_code(504);
    echo json_encode(['error' => 'File scan timed out, please try again']);

} catch (AmaasException $e) {
    // Other scan errors
    error_log("Scan error: " . $e->getMessage());
    http_response_code(500);
    echo json_encode(['error' => 'File scan failed']);

} catch (InvalidArgumentException $e) {
    // Invalid input (file not found, invalid region, etc.)
    http_response_code(400);
    echo json_encode(['error' => $e->getMessage()]);
}
```

---

## Logging

### Enable Debug Logging

```php
// Via constructor
$scanner = new TrendMicroScanner('au', $apiKey, 300, true);

// Or via setter
$scanner->setDebug(true);
```

Debug messages are sent to PHP's `error_log()`.

### File Logging

```php
$scanner->setLogFile('/var/log/trend-micro-scans.log');
```

### Log Format

```
[2024-01-15 10:30:45] Scanning file: /uploads/document.pdf
[2024-01-15 10:30:47] Scan CLEAN: document.pdf (scanId: abc123-def456)
[2024-01-15 10:31:02] Scan MALWARE DETECTED: malicious.exe - EICAR-Test-File, Trojan.Generic
```

---

## Security

### TLS/SSL

All communication with Trend Vision One uses TLS encryption. The Node.js gRPC client verifies server certificates using Trend Micro's publicly-signed certificates.

### API Key Security

- **Never commit** API keys to version control
- Store API keys in environment variables or secrets managers
- Use `.env` files for local development (add to `.gitignore`)
- Rotate API keys periodically
- Use the minimum required permissions

### File Handling Best Practices

- Scan files **before** moving to permanent storage
- Use quarantine directories for infected files
- Log all scan results for audit purposes
- Implement file size limits before scanning
- Delete infected files immediately after detection

---

## Troubleshooting

### Scanner service not found

**Error:** `RuntimeException: Scanner service not found at: /path/to/service/scanner.js`

**Solution:** Run `npm install` in the `service/` directory:

```bash
cd /path/to/sdk/service
npm install
```

### Node.js not found

**Error:** `ConnectionException: Failed to start scanner service`

**Solution:** Ensure Node.js 16+ is installed and in the system PATH:

```bash
node --version  # Should be v16.0.0 or later
```

### Authentication failed

**Error:** `AuthenticationException: Authentication failed - check API key and region`

**Solutions:**
1. Verify API key is correct and not expired
2. Ensure API key has **Run file scan via SDK** permission
3. Verify region matches where your Vision One account is hosted

### Connection timeout

**Error:** `TimeoutException: Scan timed out`

**Solutions:**
1. Increase timeout: `new TrendMicroScanner($region, $apiKey, 600)`
2. Check network connectivity to Trend Micro endpoints
3. Verify firewall allows outbound HTTPS/gRPC traffic (port 443)

### Testing the Service

Run the service test to verify installation:

```bash
node /path/to/sdk/service/scanner.js --test
```

Expected output:
```json
{
  "success": true,
  "status": "test",
  "message": "Scanner service is working",
  "version": "1.0.0",
  "nodeVersion": "v20.0.0",
  "regions": ["us", "eu", "jp", "sg", "au", "in", "me"]
}
```

### PHP Test

```php
<?php

require_once 'TrendMicroScanner.php';

try {
    $scanner = new TrendMicroScanner('au', 'test-key');
    $status = $scanner->testService();
    print_r($status);
} catch (Exception $e) {
    echo "Error: " . $e->getMessage() . "\n";
}
```

---

## Architecture

This SDK uses a hybrid PHP/Node.js architecture:

```
┌─────────────────────┐
│    PHP Application  │
│    (Your Code)      │
└──────────┬──────────┘
           │
           │ require_once
           ▼
┌─────────────────────┐
│  TrendMicroScanner  │
│  (PHP Library)      │
└──────────┬──────────┘
           │
           │ proc_open() - stdin/stdout JSON IPC
           ▼
┌─────────────────────┐
│  scanner.js         │
│  (Node.js Service)  │
└──────────┬──────────┘
           │
           │ gRPC (TLS)
           ▼
┌─────────────────────┐
│  Trend Vision One   │
│  File Security API  │
└─────────────────────┘
```

### Why This Architecture?

| Benefit | Description |
|---------|-------------|
| **No PHP gRPC extension** | Avoids complex gRPC-PHP installation and compilation |
| **Official SDK compatibility** | Uses the official `file-security-sdk` npm package |
| **No network ports** | IPC via stdin/stdout - no firewall configuration needed |
| **Process isolation** | Each scan runs in an isolated Node.js process |
| **Cross-platform** | Works on Linux, macOS, and Windows |
| **Portable** | Self-contained library can be moved between projects |

### File Structure

```
lib/
├── TrendMicroScanner.php      # Main PHP scanner class
├── TrendMicro/
│   └── FileSecurity/
│       ├── autoload.php       # PSR-4 autoloader
│       ├── AmaasClient.php    # Low-level client (optional)
│       ├── Model/
│       │   ├── ScanResult.php # Scan result model
│       │   ├── Malware.php    # Malware model
│       │   └── ScanOptions.php
│       ├── Exception/
│       │   ├── AmaasException.php
│       │   ├── AuthenticationException.php
│       │   ├── ConnectionException.php
│       │   └── TimeoutException.php
│       └── Http/
│           └── HttpClient.php
└── service/
    ├── package.json           # Node.js dependencies
    ├── scanner.js             # Node.js gRPC service
    └── node_modules/          # Installed dependencies
```

---

## Service Limits

| Limit | Value |
|-------|-------|
| Maximum file size | 500 MB |
| Maximum tags per scan | 8 |
| Maximum tag length | 63 characters |
| Default timeout | 300 seconds |
| Maximum timeout | 1800 seconds |

---

## License

This SDK is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

## Related Resources

### Official Documentation

- [Trend Vision One File Security Overview](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security)
- [Deploying the SDK](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-deploying-node-js-sdk)
- [Vision One API Reference](https://automation.trendmicro.com/xdr/api-v3#tag/File-Security)

### Other SDKs

- [Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)
- [Python SDK](https://github.com/trendmicro/tm-v1-fs-python-sdk)
- [Java SDK](https://github.com/trendmicro/tm-v1-fs-java-sdk)
- [Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)

### Support

For SDK issues, please open an issue on the GitHub repository.

For Trend Vision One service issues, contact [Trend Micro Support](https://success.trendmicro.com/).
