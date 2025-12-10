<?php
/**
 * TrendMicroScanner - FileGator Integration Wrapper
 *
 * A simplified wrapper around the Trend Micro Vision One File Security SDK
 * for easy integration with FileGator hooks.
 *
 * @package FileGator\TrendMicro
 * @license MIT
 */

// Load SDK autoloader
require_once __DIR__ . '/TrendMicro/FileSecurity/autoload.php';

use TrendMicro\FileSecurity\AmaasClient;
use TrendMicro\FileSecurity\LogLevel;
use TrendMicro\FileSecurity\Model\ScanResult;
use TrendMicro\FileSecurity\Model\ScanOptions;
use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;
use TrendMicro\FileSecurity\Exception\ConnectionException;
use TrendMicro\FileSecurity\Exception\TimeoutException;

/**
 * TrendMicroScanner - FileGator Integration Class
 *
 * Provides a simple interface for scanning files using Trend Micro Vision One
 * File Security API. Designed for use in FileGator upload hooks.
 *
 * Usage:
 * ```php
 * $scanner = new TrendMicroScanner($region, $apiKey);
 * $result = $scanner->scanFile('/path/to/file.pdf');
 * if ($result->hasMalware()) {
 *     // Handle infected file
 * }
 * $scanner->close();
 * ```
 */
class TrendMicroScanner
{
    /**
     * @var AmaasClient SDK client instance
     */
    private AmaasClient $client;

    /**
     * @var string|null Log file path
     */
    private ?string $logFile = null;

    /**
     * @var bool Enable debug logging
     */
    private bool $debug = false;

    /**
     * Region mapping from simple names to SDK region codes
     */
    private const REGION_MAP = [
        'us' => 'us-east-1',
        'eu' => 'eu-central-1',
        'jp' => 'ap-northeast-1',
        'sg' => 'ap-southeast-1',
        'au' => 'ap-southeast-2',
        'in' => 'ap-south-1',
        'me' => 'me-central-1',
        // Also accept SDK region codes directly
        'us-east-1' => 'us-east-1',
        'eu-central-1' => 'eu-central-1',
        'ap-northeast-1' => 'ap-northeast-1',
        'ap-southeast-1' => 'ap-southeast-1',
        'ap-southeast-2' => 'ap-southeast-2',
        'ap-south-1' => 'ap-south-1',
        'me-central-1' => 'me-central-1',
        // Legacy region names
        'us-1' => 'us-1',
        'eu-1' => 'eu-1',
        'jp-1' => 'jp-1',
        'sg-1' => 'sg-1',
        'au-1' => 'au-1',
        'in-1' => 'in-1',
    ];

    /**
     * Create a new TrendMicroScanner instance
     *
     * @param string $region Region identifier (e.g., 'us', 'eu', 'us-east-1')
     * @param string $apiKey API key for authentication
     * @param int $timeout Timeout in seconds (default: 300)
     * @param bool $debug Enable debug logging
     *
     * @throws \InvalidArgumentException If region is invalid
     * @throws AmaasException If client creation fails
     */
    public function __construct(
        string $region,
        string $apiKey,
        int $timeout = 300,
        bool $debug = false
    ) {
        $this->debug = $debug;

        // Map simple region name to SDK region code
        $sdkRegion = self::REGION_MAP[$region] ?? null;
        if ($sdkRegion === null) {
            throw new \InvalidArgumentException(
                "Invalid region: $region. Valid regions: " . implode(', ', array_keys(self::REGION_MAP))
            );
        }

        $this->client = new AmaasClient($sdkRegion, $apiKey, $timeout);

        if ($debug) {
            $this->client->setLoggingLevel(LogLevel::DEBUG);
        }
    }

    /**
     * Create scanner from environment variables
     *
     * Reads configuration from:
     * - TREND_MICRO_REGION or TM_REGION (default: 'us')
     * - TREND_MICRO_API_KEY or TM_API_KEY (required)
     * - TREND_MICRO_TIMEOUT or TM_TIMEOUT (default: 300)
     * - TREND_MICRO_DEBUG or TM_DEBUG (default: false)
     *
     * @return self
     * @throws \RuntimeException If API key is not set
     */
    public static function fromEnvironment(): self
    {
        $region = getenv('TREND_MICRO_REGION') ?: getenv('TM_REGION') ?: 'us';
        $apiKey = getenv('TREND_MICRO_API_KEY') ?: getenv('TM_API_KEY') ?: '';
        $timeout = (int) (getenv('TREND_MICRO_TIMEOUT') ?: getenv('TM_TIMEOUT') ?: 300);
        $debug = filter_var(getenv('TREND_MICRO_DEBUG') ?: getenv('TM_DEBUG'), FILTER_VALIDATE_BOOLEAN);

        if (empty($apiKey)) {
            throw new \RuntimeException(
                'Trend Micro API key not set. Set TREND_MICRO_API_KEY or TM_API_KEY environment variable.'
            );
        }

        return new self($region, $apiKey, $timeout, $debug);
    }

    /**
     * Create scanner from .env file
     *
     * @param string $envFile Path to .env file
     * @return self
     * @throws \RuntimeException If file not found or API key not set
     */
    public static function fromEnvFile(string $envFile): self
    {
        if (!file_exists($envFile)) {
            throw new \RuntimeException("Environment file not found: $envFile");
        }

        $lines = file($envFile, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
        foreach ($lines as $line) {
            $line = trim($line);
            if (empty($line) || $line[0] === '#') {
                continue;
            }

            if (strpos($line, '=') !== false) {
                list($key, $value) = explode('=', $line, 2);
                $key = trim($key);
                $value = trim($value, " \t\n\r\0\x0B\"'");
                putenv("$key=$value");
                $_ENV[$key] = $value;
            }
        }

        return self::fromEnvironment();
    }

    /**
     * Scan a file for malware
     *
     * @param string $filePath Path to the file to scan
     * @param array $tags Optional tags for categorization (max 8, each max 63 chars)
     * @return ScanResult Scan result object
     *
     * @throws \InvalidArgumentException If file is invalid
     * @throws AmaasException If scan fails
     * @throws AuthenticationException If API key is invalid
     * @throws ConnectionException If connection fails
     * @throws TimeoutException If scan times out
     */
    public function scanFile(string $filePath, array $tags = []): ScanResult
    {
        $this->log("Scanning file: $filePath");

        $options = new ScanOptions($tags);
        $result = $this->client->scanFile($filePath, $options);

        $this->logResult($result);

        return $result;
    }

    /**
     * Scan a buffer (file content) for malware
     *
     * @param string $identifier Identifier/filename for the buffer
     * @param string $buffer Buffer content to scan
     * @param array $tags Optional tags for categorization
     * @return ScanResult Scan result object
     *
     * @throws \InvalidArgumentException If parameters are invalid
     * @throws AmaasException If scan fails
     */
    public function scanBuffer(string $identifier, string $buffer, array $tags = []): ScanResult
    {
        $this->log("Scanning buffer: $identifier (size: " . strlen($buffer) . " bytes)");

        $options = new ScanOptions($tags);
        $result = $this->client->scanBuffer($identifier, $buffer, $options);

        $this->logResult($result);

        return $result;
    }

    /**
     * Quick check if a file is clean (shorthand method)
     *
     * @param string $filePath Path to the file to scan
     * @return bool True if file is clean, false if malware detected
     *
     * @throws AmaasException If scan fails
     */
    public function isFileClean(string $filePath): bool
    {
        return $this->scanFile($filePath)->isClean();
    }

    /**
     * Set log file for scan results
     *
     * @param string $logFile Path to log file
     * @return self
     */
    public function setLogFile(string $logFile): self
    {
        $this->logFile = $logFile;
        return $this;
    }

    /**
     * Enable or disable debug mode
     *
     * @param bool $enabled Whether to enable debug mode
     * @return self
     */
    public function setDebug(bool $enabled): self
    {
        $this->debug = $enabled;
        $this->client->setLoggingLevel($enabled ? LogLevel::DEBUG : LogLevel::OFF);
        return $this;
    }

    /**
     * Get the underlying SDK client
     *
     * @return AmaasClient
     */
    public function getClient(): AmaasClient
    {
        return $this->client;
    }

    /**
     * Close the scanner and release resources
     */
    public function close(): void
    {
        $this->client->close();
    }

    /**
     * Log a message
     */
    private function log(string $message): void
    {
        if (!$this->logFile && !$this->debug) {
            return;
        }

        $timestamp = date('Y-m-d H:i:s');
        $logMessage = "[$timestamp] $message";

        if ($this->debug) {
            error_log("[TrendMicroScanner] $message");
        }

        if ($this->logFile) {
            file_put_contents($this->logFile, $logMessage . "\n", FILE_APPEND | LOCK_EX);
        }
    }

    /**
     * Log scan result
     */
    private function logResult(ScanResult $result): void
    {
        if ($result->isClean()) {
            $this->log("Scan CLEAN: {$result->getFileName()}");
        } else {
            $malwares = array_map(
                fn($m) => $m->getMalwareName(),
                $result->getFoundMalwares()
            );
            $this->log("Scan MALWARE DETECTED: {$result->getFileName()} - " . implode(', ', $malwares));
        }
    }
}
