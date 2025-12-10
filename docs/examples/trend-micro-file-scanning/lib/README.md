# Trend Micro Vision One File Security SDK for PHP

A PHP SDK for the Trend Micro Vision One File Security scanning service (AMaaS - Anti-Malware as a Service).

## Features

- File and buffer scanning for malware detection
- Multiple region support (US, EU, Japan, Singapore, Australia, India, Middle East)
- Comprehensive error handling with specific exception types
- PSR-4 autoloading compatible
- No external dependencies (uses cURL)
- PHP 8.0+ support

## Installation

### Using Composer (Recommended)

```bash
composer require trendmicro/filesecurity-sdk
```

### Manual Installation

1. Copy the `TrendMicro` directory to your project
2. Include the autoloader:

```php
require_once 'path/to/TrendMicro/FileSecurity/autoload.php';
```

## Quick Start

```php
<?php
require_once 'TrendMicro/FileSecurity/autoload.php';

use TrendMicro\FileSecurity\AmaasClient;

// Create client
$client = new AmaasClient('us-east-1', 'your-api-key');

// Scan a file
$result = $client->scanFile('/path/to/file.pdf');

if ($result->isClean()) {
    echo "File is clean!\n";
} else {
    echo "Malware detected!\n";
    foreach ($result->getFoundMalwares() as $malware) {
        echo "- " . $malware->getMalwareName() . "\n";
    }
}

// Always close when done
$client->close();
```

## Supported Regions

| Region Code | Location | Endpoint |
|-------------|----------|----------|
| `us-east-1` | United States | antimalware.us-1.cloudone.trendmicro.com |
| `eu-central-1` | Europe (Germany) | antimalware.de-1.cloudone.trendmicro.com |
| `ap-northeast-1` | Japan | antimalware.jp-1.cloudone.trendmicro.com |
| `ap-southeast-1` | Singapore | antimalware.sg-1.cloudone.trendmicro.com |
| `ap-southeast-2` | Australia | antimalware.au-1.cloudone.trendmicro.com |
| `ap-south-1` | India | antimalware.in-1.cloudone.trendmicro.com |
| `me-central-1` | Middle East | antimalware.trend-us-1.cloudone.trendmicro.com |

Legacy region names (`us-1`, `eu-1`, `jp-1`, `sg-1`, `au-1`, `in-1`) are also supported.

## API Reference

### AmaasClient

The main client class for interacting with the File Security API.

#### Constructor

```php
$client = new AmaasClient(
    string $region,      // Region identifier
    string $apiKey,      // API key for authentication
    int $timeout = 300,  // Timeout in seconds
    bool $enableTls = true,  // Use TLS (recommended)
    ?string $caCert = null   // Custom CA certificate path
);
```

#### Methods

##### scanFile()

Scan a file for malware.

```php
$result = $client->scanFile(
    string $filePath,           // Path to file
    ?ScanOptions $options = null // Optional scan options
): ScanResult;
```

##### scanBuffer()

Scan a buffer (file content) for malware.

```php
$result = $client->scanBuffer(
    string $identifier,         // Buffer identifier/filename
    string $buffer,             // File content
    ?ScanOptions $options = null
): ScanResult;
```

##### close()

Close the client and release resources.

```php
$client->close();
```

### ScanOptions

Options for customizing scan behavior.

```php
$options = new ScanOptions(['tag1', 'tag2']);
$options->setPml(true);      // Enable Predictive Machine Learning
$options->setFeedback(true); // Enable feedback to Trend Micro
```

### ScanResult

Contains the scan results.

```php
$result->isClean();         // bool - true if no malware
$result->hasMalware();      // bool - true if malware found
$result->getScanResult();   // int - 0 = clean, >0 = malware count
$result->getFoundMalwares(); // Malware[] - array of detected malware
$result->getFileSha1();     // string - SHA1 hash
$result->getFileSha256();   // string - SHA256 hash
$result->getScanId();       // string - unique scan ID
```

### Malware

Represents a detected malware.

```php
$malware->getMalwareName();      // string - malware name
$malware->getFileName();         // string - file name
$malware->getType();             // string - malware type
$malware->getFilter();           // string - detection filter
$malware->getFilterDescription(); // string - filter description
```

## Exception Handling

The SDK throws specific exceptions for different error types:

```php
use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\InvalidArgumentException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;
use TrendMicro\FileSecurity\Exception\ConnectionException;
use TrendMicro\FileSecurity\Exception\TimeoutException;

try {
    $result = $client->scanFile('/path/to/file');
} catch (InvalidArgumentException $e) {
    // Invalid file path, region, etc.
    echo "Invalid argument: " . $e->getMessage();
} catch (AuthenticationException $e) {
    // Invalid or expired API key
    echo "Authentication failed: " . $e->getMessage();
} catch (ConnectionException $e) {
    // Network or connection error
    echo "Connection failed: " . $e->getMessage();
    echo "Endpoint: " . $e->getEndpoint();
} catch (TimeoutException $e) {
    // Operation timed out
    echo "Timeout after " . $e->getTimeoutSeconds() . " seconds";
} catch (AmaasException $e) {
    // Generic API error
    echo "Scan failed: " . $e->getMessage();
    echo "Error code: " . $e->getErrorCode();
}
```

## Logging

Enable logging for debugging:

```php
use TrendMicro\FileSecurity\LogLevel;

$client->setLoggingLevel(LogLevel::DEBUG);

// Custom logging callback
$client->configLoggingCallback(function ($level, $message) {
    error_log($message);
});
```

Log levels: `OFF`, `FATAL`, `ERROR`, `WARN`, `INFO`, `DEBUG`

## FileGator Integration

For easy FileGator integration, use the `TrendMicroScanner` wrapper:

```php
require_once 'TrendMicroScanner.php';

// From environment variables
$scanner = TrendMicroScanner::fromEnvironment();

// Or from .env file
$scanner = TrendMicroScanner::fromEnvFile('/path/to/.env');

// Scan file
$result = $scanner->scanFile('/path/to/file');

if (!$result->isClean()) {
    // Quarantine or delete the file
}

$scanner->close();
```

Environment variables:
- `TREND_MICRO_API_KEY` - API key (required)
- `TREND_MICRO_REGION` - Region (default: 'us')
- `TREND_MICRO_TIMEOUT` - Timeout in seconds (default: 300)
- `TREND_MICRO_DEBUG` - Enable debug logging (default: false)

## Limits

- Maximum file size: 500MB
- Maximum tags: 8
- Maximum tag length: 63 characters
- Default timeout: 300 seconds

## Requirements

- PHP 8.0 or higher
- cURL extension
- JSON extension

## Testing

```bash
cd lib
composer install
./vendor/bin/phpunit
```

## License

MIT License - see LICENSE file for details.

## Links

- [Trend Micro Vision One File Security Documentation](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security)
- [API Reference](https://automation.trendmicro.com/xdr/api-v3#tag/File-Security)
- [Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)
- [Java SDK](https://github.com/trendmicro/tm-v1-fs-java-sdk)
