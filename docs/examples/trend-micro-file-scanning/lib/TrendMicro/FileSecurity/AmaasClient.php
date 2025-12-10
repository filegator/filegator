<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * A PHP client library for the Trend Micro Vision One File Security scanning service.
 * This SDK provides file and buffer scanning capabilities using the AMaaS (Anti-Malware as a Service) API.
 *
 * @package TrendMicro\FileSecurity
 * @version 1.0.0
 * @license MIT
 * @link https://github.com/trendmicro/tm-v1-fs-php-sdk
 */

namespace TrendMicro\FileSecurity;

use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\InvalidArgumentException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;
use TrendMicro\FileSecurity\Exception\ConnectionException;
use TrendMicro\FileSecurity\Exception\TimeoutException;
use TrendMicro\FileSecurity\Http\HttpClient;
use TrendMicro\FileSecurity\Model\ScanResult;
use TrendMicro\FileSecurity\Model\ScanOptions;

/**
 * AMaaS (Anti-Malware as a Service) Client
 *
 * Main entry point for the Trend Micro Vision One File Security SDK.
 * Provides methods for scanning files and buffers for malware.
 *
 * Usage:
 * ```php
 * $client = new AmaasClient('us-east-1', 'your-api-key');
 * $result = $client->scanFile('/path/to/file.exe');
 * if ($result->hasMalware()) {
 *     foreach ($result->getFoundMalwares() as $malware) {
 *         echo "Detected: " . $malware->getMalwareName() . "\n";
 *     }
 * }
 * $client->close();
 * ```
 */
class AmaasClient
{
    /**
     * SDK Version
     */
    public const VERSION = '1.0.0';

    /**
     * Default timeout in seconds
     */
    public const DEFAULT_TIMEOUT = 300;

    /**
     * Maximum file size (500MB)
     */
    public const MAX_FILE_SIZE = 524288000;

    /**
     * Maximum number of tags
     */
    public const MAX_TAGS = 8;

    /**
     * Maximum tag length
     */
    public const MAX_TAG_LENGTH = 63;

    /**
     * Supported regions and their endpoints
     */
    private const REGION_ENDPOINTS = [
        'us-east-1'      => 'antimalware.us-1.cloudone.trendmicro.com',
        'eu-central-1'   => 'antimalware.de-1.cloudone.trendmicro.com',
        'ap-northeast-1' => 'antimalware.jp-1.cloudone.trendmicro.com',
        'ap-southeast-1' => 'antimalware.sg-1.cloudone.trendmicro.com',
        'ap-southeast-2' => 'antimalware.au-1.cloudone.trendmicro.com',
        'ap-south-1'     => 'antimalware.in-1.cloudone.trendmicro.com',
        'me-central-1'   => 'antimalware.trend-us-1.cloudone.trendmicro.com',
        // Legacy region names
        'us-1' => 'antimalware.us-1.cloudone.trendmicro.com',
        'eu-1' => 'antimalware.de-1.cloudone.trendmicro.com',
        'jp-1' => 'antimalware.jp-1.cloudone.trendmicro.com',
        'sg-1' => 'antimalware.sg-1.cloudone.trendmicro.com',
        'au-1' => 'antimalware.au-1.cloudone.trendmicro.com',
        'in-1' => 'antimalware.in-1.cloudone.trendmicro.com',
    ];

    /**
     * @var string Region identifier
     */
    private string $region;

    /**
     * @var string API key for authentication
     */
    private string $apiKey;

    /**
     * @var int Timeout in seconds
     */
    private int $timeout;

    /**
     * @var bool Whether TLS is enabled
     */
    private bool $enableTls;

    /**
     * @var string|null Custom CA certificate path
     */
    private ?string $caCert;

    /**
     * @var HttpClient HTTP client instance
     */
    private HttpClient $httpClient;

    /**
     * @var string API endpoint URL
     */
    private string $endpoint;

    /**
     * @var callable|null Logging callback
     */
    private $loggingCallback = null;

    /**
     * @var int Logging level
     */
    private int $loggingLevel = LogLevel::OFF;

    /**
     * Create a new AMaaS client
     *
     * @param string $region Region identifier (e.g., 'us-east-1', 'eu-central-1')
     * @param string $apiKey API key for authentication
     * @param int $timeout Timeout in seconds (default: 300)
     * @param bool $enableTls Whether to use TLS (default: true)
     * @param string|null $caCert Path to custom CA certificate
     *
     * @throws InvalidArgumentException If region or API key is invalid
     */
    public function __construct(
        string $region,
        string $apiKey,
        int $timeout = self::DEFAULT_TIMEOUT,
        bool $enableTls = true,
        ?string $caCert = null
    ) {
        $this->validateRegion($region);
        $this->validateApiKey($apiKey);

        $this->region = $region;
        $this->apiKey = $apiKey;
        $this->timeout = $timeout;
        $this->enableTls = $enableTls;
        $this->caCert = $caCert;

        $this->endpoint = $this->buildEndpoint($region);
        $this->httpClient = new HttpClient($this->endpoint, $apiKey, $timeout, $enableTls, $caCert);
    }

    /**
     * Create client from region identifier
     *
     * @param string $region Region identifier
     * @param string $apiKey API key
     * @param int $timeout Timeout in seconds
     * @return self
     */
    public static function initByRegion(
        string $region,
        string $apiKey,
        int $timeout = self::DEFAULT_TIMEOUT
    ): self {
        return new self($region, $apiKey, $timeout);
    }

    /**
     * Scan a file for malware
     *
     * @param string $filePath Path to the file to scan
     * @param ScanOptions|null $options Scan options
     * @return ScanResult Scan result
     *
     * @throws InvalidArgumentException If file path is invalid
     * @throws AmaasException If scan fails
     */
    public function scanFile(string $filePath, ?ScanOptions $options = null): ScanResult
    {
        $this->log(LogLevel::INFO, "Scanning file: {$filePath}");

        if (!file_exists($filePath)) {
            throw new InvalidArgumentException("File not found: {$filePath}");
        }

        if (!is_readable($filePath)) {
            throw new InvalidArgumentException("File not readable: {$filePath}");
        }

        $fileSize = filesize($filePath);
        if ($fileSize > self::MAX_FILE_SIZE) {
            throw new InvalidArgumentException(
                "File size ({$fileSize} bytes) exceeds maximum allowed size (" . self::MAX_FILE_SIZE . " bytes)"
            );
        }

        $options = $options ?? new ScanOptions();
        $this->validateOptions($options);

        $fileName = basename($filePath);
        $fileContent = file_get_contents($filePath);

        if ($fileContent === false) {
            throw new AmaasException("Failed to read file: {$filePath}");
        }

        return $this->performScan($fileName, $fileContent, $filePath, $options);
    }

    /**
     * Scan a buffer for malware
     *
     * @param string $identifier Identifier/filename for the buffer
     * @param string $buffer Buffer content to scan
     * @param ScanOptions|null $options Scan options
     * @return ScanResult Scan result
     *
     * @throws InvalidArgumentException If parameters are invalid
     * @throws AmaasException If scan fails
     */
    public function scanBuffer(string $identifier, string $buffer, ?ScanOptions $options = null): ScanResult
    {
        $this->log(LogLevel::INFO, "Scanning buffer: {$identifier}");

        if (empty($identifier)) {
            throw new InvalidArgumentException("Identifier cannot be empty");
        }

        $bufferSize = strlen($buffer);
        if ($bufferSize > self::MAX_FILE_SIZE) {
            throw new InvalidArgumentException(
                "Buffer size ({$bufferSize} bytes) exceeds maximum allowed size (" . self::MAX_FILE_SIZE . " bytes)"
            );
        }

        $options = $options ?? new ScanOptions();
        $this->validateOptions($options);

        return $this->performScan($identifier, $buffer, null, $options);
    }

    /**
     * Perform the actual scan operation
     *
     * @param string $fileName File/buffer name
     * @param string $content Content to scan
     * @param string|null $filePath Original file path (if scanning file)
     * @param ScanOptions $options Scan options
     * @return ScanResult
     *
     * @throws AmaasException If scan fails
     */
    private function performScan(
        string $fileName,
        string $content,
        ?string $filePath,
        ScanOptions $options
    ): ScanResult {
        $startTime = microtime(true);

        try {
            $this->log(LogLevel::DEBUG, "Starting scan for: {$fileName}");

            $response = $this->httpClient->scan($fileName, $content, $options);

            $duration = round((microtime(true) - $startTime) * 1000);
            $this->log(LogLevel::DEBUG, "Scan completed in {$duration}ms");

            return ScanResult::fromResponse($response, $fileName, $filePath);
        } catch (AuthenticationException $e) {
            $this->log(LogLevel::ERROR, "Authentication failed: " . $e->getMessage());
            throw $e;
        } catch (ConnectionException $e) {
            $this->log(LogLevel::ERROR, "Connection failed: " . $e->getMessage());
            throw $e;
        } catch (TimeoutException $e) {
            $this->log(LogLevel::ERROR, "Scan timed out: " . $e->getMessage());
            throw $e;
        } catch (\Exception $e) {
            $this->log(LogLevel::ERROR, "Scan failed: " . $e->getMessage());
            throw new AmaasException("Scan failed: " . $e->getMessage(), 0, $e);
        }
    }

    /**
     * Close the client and release resources
     */
    public function close(): void
    {
        $this->log(LogLevel::DEBUG, "Closing AMaaS client");
        $this->httpClient->close();
    }

    /**
     * Set the logging level
     *
     * @param int $level Log level (use LogLevel constants)
     */
    public function setLoggingLevel(int $level): void
    {
        $this->loggingLevel = $level;
    }

    /**
     * Configure a custom logging callback
     *
     * @param callable $callback Callback function (level, message)
     */
    public function configLoggingCallback(callable $callback): void
    {
        $this->loggingCallback = $callback;
    }

    /**
     * Get the current region
     *
     * @return string
     */
    public function getRegion(): string
    {
        return $this->region;
    }

    /**
     * Get the API endpoint URL
     *
     * @return string
     */
    public function getEndpoint(): string
    {
        return $this->endpoint;
    }

    /**
     * Validate region parameter
     *
     * @param string $region Region to validate
     * @throws InvalidArgumentException If region is invalid
     */
    private function validateRegion(string $region): void
    {
        if (empty($region)) {
            throw new InvalidArgumentException("Region cannot be empty");
        }

        if (!isset(self::REGION_ENDPOINTS[$region])) {
            $validRegions = implode(', ', array_keys(self::REGION_ENDPOINTS));
            throw new InvalidArgumentException(
                "Invalid region: {$region}. Valid regions are: {$validRegions}"
            );
        }
    }

    /**
     * Validate API key parameter
     *
     * @param string $apiKey API key to validate
     * @throws InvalidArgumentException If API key is invalid
     */
    private function validateApiKey(string $apiKey): void
    {
        if (empty($apiKey)) {
            throw new InvalidArgumentException("API key cannot be empty");
        }
    }

    /**
     * Validate scan options
     *
     * @param ScanOptions $options Options to validate
     * @throws InvalidArgumentException If options are invalid
     */
    private function validateOptions(ScanOptions $options): void
    {
        $tags = $options->getTags();

        if (count($tags) > self::MAX_TAGS) {
            throw new InvalidArgumentException(
                "Maximum " . self::MAX_TAGS . " tags allowed, got " . count($tags)
            );
        }

        foreach ($tags as $tag) {
            if (strlen($tag) > self::MAX_TAG_LENGTH) {
                throw new InvalidArgumentException(
                    "Tag exceeds maximum length of " . self::MAX_TAG_LENGTH . " characters: {$tag}"
                );
            }
        }
    }

    /**
     * Build the API endpoint URL for a region
     *
     * @param string $region Region identifier
     * @return string Endpoint URL
     */
    private function buildEndpoint(string $region): string
    {
        $host = self::REGION_ENDPOINTS[$region];
        $protocol = $this->enableTls ? 'https' : 'http';
        return "{$protocol}://{$host}";
    }

    /**
     * Log a message
     *
     * @param int $level Log level
     * @param string $message Message to log
     */
    private function log(int $level, string $message): void
    {
        if ($level > $this->loggingLevel) {
            return;
        }

        $timestamp = date('Y-m-d H:i:s.v');
        $levelName = LogLevel::getName($level);
        $formattedMessage = "[{$timestamp}] [{$levelName}] {$message}";

        if ($this->loggingCallback !== null) {
            ($this->loggingCallback)($level, $formattedMessage);
        } else {
            error_log($formattedMessage);
        }
    }
}

/**
 * Log level constants
 */
class LogLevel
{
    public const OFF = 0;
    public const FATAL = 1;
    public const ERROR = 2;
    public const WARN = 3;
    public const INFO = 4;
    public const DEBUG = 5;

    /**
     * Get the name of a log level
     *
     * @param int $level Log level
     * @return string Level name
     */
    public static function getName(int $level): string
    {
        return match ($level) {
            self::OFF => 'OFF',
            self::FATAL => 'FATAL',
            self::ERROR => 'ERROR',
            self::WARN => 'WARN',
            self::INFO => 'INFO',
            self::DEBUG => 'DEBUG',
            default => 'UNKNOWN',
        };
    }
}
