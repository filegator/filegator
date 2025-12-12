<?php
/**
 * TrendMicroScanner - Trend Micro Vision One File Security PHP Library
 *
 * A self-contained PHP library for scanning files using Trend Micro Vision One
 * File Security. Uses an embedded Node.js service for gRPC communication.
 *
 * This library is designed to be portable and can be extracted to a standalone
 * Composer package.
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

// Load models and exceptions
require_once __DIR__ . '/TrendAndrew/FileSecurity/autoload.php';

use TrendAndrew\FileSecurity\Model\ScanResult;
use TrendAndrew\FileSecurity\Exception\AmaasException;
use TrendAndrew\FileSecurity\Exception\AuthenticationException;
use TrendAndrew\FileSecurity\Exception\ConnectionException;
use TrendAndrew\FileSecurity\Exception\TimeoutException;

/**
 * TrendMicroScanner - Main Scanner Class
 *
 * Provides a simple interface for scanning files using Trend Micro Vision One
 * File Security API via an embedded Node.js gRPC service.
 *
 * Usage:
 * ```php
 * $scanner = new TrendMicroScanner('au', $apiKey);
 * $result = $scanner->scanFile('/path/to/file.pdf');
 * if ($result->hasMalware()) {
 *     // Handle infected file
 * }
 * ```
 */
class TrendMicroScanner
{
    /**
     * @var string Region identifier
     */
    private string $region;

    /**
     * @var string API key
     */
    private string $apiKey;

    /**
     * @var int Timeout in seconds
     */
    private int $timeout;

    /**
     * @var bool Enable debug logging
     */
    private bool $debug;

    /**
     * @var string|null Log file path
     */
    private ?string $logFile = null;

    /**
     * @var string Path to Node.js executable
     */
    private string $nodePath = 'node';

    /**
     * @var string Path to scanner service
     */
    private string $servicePath;

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
        'us-1' => 'us-east-1',
        'eu-1' => 'eu-central-1',
        'jp-1' => 'ap-northeast-1',
        'sg-1' => 'ap-southeast-1',
        'au-1' => 'ap-southeast-2',
        'in-1' => 'ap-south-1',
    ];

    /**
     * Create a new TrendMicroScanner instance
     *
     * @param string $region Region identifier (e.g., 'us', 'eu', 'au', 'ap-southeast-2')
     * @param string $apiKey API key for authentication
     * @param int $timeout Timeout in seconds (default: 300)
     * @param bool $debug Enable debug logging
     *
     * @throws \InvalidArgumentException If region is invalid
     * @throws \RuntimeException If Node.js service is not installed
     */
    public function __construct(
        string $region,
        string $apiKey,
        int $timeout = 300,
        bool $debug = false
    ) {
        // Validate region
        if (!isset(self::REGION_MAP[strtolower($region)])) {
            throw new \InvalidArgumentException(
                "Invalid region: $region. Valid regions: " . implode(', ', array_keys(self::REGION_MAP))
            );
        }

        $this->region = self::REGION_MAP[strtolower($region)];
        $this->apiKey = $apiKey;
        $this->timeout = $timeout;
        $this->debug = $debug;

        // Set path to Node.js scanner service
        // Service is at lib/service/scanner.js, src/ is at lib/src/
        $this->servicePath = dirname(__DIR__) . '/service/scanner.js';

        // Validate service exists
        if (!file_exists($this->servicePath)) {
            throw new \RuntimeException(
                "Scanner service not found at: {$this->servicePath}. " .
                "Run 'npm install' in the lib/service directory."
            );
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
                'Trend Micro API key not set. Set TREND_MICRO_API_KEY environment variable.'
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
     * Set path to Node.js executable
     *
     * @param string $path Path to node executable
     * @return self
     */
    public function setNodePath(string $path): self
    {
        $this->nodePath = $path;
        return $this;
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
        return $this;
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
        // Validate file exists
        if (!file_exists($filePath)) {
            throw new \InvalidArgumentException("File not found: $filePath");
        }

        $filePath = realpath($filePath);
        $this->log("Scanning file: $filePath");

        // Build request for Node.js service
        $request = [
            'action' => 'scan',
            'file' => $filePath,
            'apiKey' => $this->apiKey,
            'region' => $this->region,
            'timeout' => $this->timeout,
            'pml' => true,
            'tags' => $tags ?: ['filegator'],
        ];

        // Call Node.js service
        $response = $this->callService($request);

        // Build ScanResult from response
        return $this->buildScanResult($filePath, $response);
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
     * Test if the scanner service is working
     *
     * @return array Service status information
     * @throws \RuntimeException If service is not available
     */
    public function testService(): array
    {
        $request = ['action' => 'test'];
        return $this->callService($request);
    }

    /**
     * Close the scanner (no-op for IPC, kept for API compatibility)
     */
    public function close(): void
    {
        // No persistent connection to close with IPC approach
    }

    /**
     * Call the Node.js scanner service via stdin/stdout
     *
     * @param array $request Request data
     * @return array Response data
     * @throws AmaasException If service call fails
     */
    private function callService(array $request): array
    {
        $inputJson = json_encode($request);

        // Build command
        $cmd = sprintf(
            '%s %s',
            escapeshellcmd($this->nodePath),
            escapeshellarg($this->servicePath)
        );

        $this->log("Calling scanner service: $cmd");

        // Setup process pipes
        $descriptorSpec = [
            0 => ['pipe', 'r'],  // stdin
            1 => ['pipe', 'w'],  // stdout
            2 => ['pipe', 'w'],  // stderr
        ];

        // Start process
        $process = proc_open($cmd, $descriptorSpec, $pipes, dirname($this->servicePath));

        if (!is_resource($process)) {
            throw new ConnectionException("Failed to start scanner service");
        }

        // Write request to stdin
        fwrite($pipes[0], $inputJson);
        fclose($pipes[0]);

        // Read response from stdout
        $stdout = stream_get_contents($pipes[1]);
        fclose($pipes[1]);

        // Read any errors from stderr
        $stderr = stream_get_contents($pipes[2]);
        fclose($pipes[2]);

        // Get exit code
        $exitCode = proc_close($process);

        // Log debug info
        if ($this->debug) {
            $this->log("Service stdout: $stdout");
            if ($stderr) {
                $this->log("Service stderr: $stderr");
            }
            $this->log("Service exit code: $exitCode");
        }

        // Parse response
        $response = json_decode(trim($stdout), true);

        if (json_last_error() !== JSON_ERROR_NONE) {
            throw new AmaasException(
                "Invalid response from scanner service: " . json_last_error_msg() .
                "\nOutput: " . substr($stdout, 0, 500)
            );
        }

        // Handle errors from service
        if (!($response['success'] ?? false) && $exitCode !== 0) {
            $errorMessage = $response['message'] ?? 'Unknown error';
            $errorCode = $response['errorCode'] ?? 'UNKNOWN_ERROR';

            switch ($errorCode) {
                case 'AUTH_ERROR':
                    throw new AuthenticationException($errorMessage);
                case 'TIMEOUT':
                    throw new TimeoutException($errorMessage);
                case 'CONNECTION_ERROR':
                    throw new ConnectionException($errorMessage);
                default:
                    throw new AmaasException($errorMessage);
            }
        }

        return $response;
    }

    /**
     * Build ScanResult from service response
     *
     * @param string $filePath Original file path
     * @param array $response Service response
     * @return ScanResult
     */
    private function buildScanResult(string $filePath, array $response): ScanResult
    {
        // Transform Node.js service response to match ScanResult::fromResponse() format
        $scanResultData = [
            'scanResult' => $response['malwareFound'] ? count($response['threats'] ?? [1]) : 0,
            'scanId' => $response['scanId'] ?? null,
            'fileSHA256' => $response['fileSha256'] ?? null,
            'foundMalwares' => $response['threats'] ?? [],
        ];

        $result = ScanResult::fromResponse($scanResultData, basename($filePath), $filePath);

        $this->logResult($result);

        return $result;
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
            $logDir = dirname($this->logFile);
            if (!is_dir($logDir)) {
                mkdir($logDir, 0755, true);
            }
            file_put_contents($this->logFile, $logMessage . "\n", FILE_APPEND | LOCK_EX);
        }
    }

    /**
     * Log scan result
     */
    private function logResult(ScanResult $result): void
    {
        if ($result->isClean()) {
            $this->log("Scan CLEAN: {$result->getFileName()} (scanId: {$result->getScanId()})");
        } else {
            $malwares = array_map(
                fn($m) => $m->getMalwareName(),
                $result->getFoundMalwares()
            );
            $this->log("Scan MALWARE DETECTED: {$result->getFileName()} - " . implode(', ', $malwares));
        }
    }

    /**
     * Get available regions
     *
     * @return array List of valid region identifiers
     */
    public static function getAvailableRegions(): array
    {
        return array_keys(self::REGION_MAP);
    }
}
