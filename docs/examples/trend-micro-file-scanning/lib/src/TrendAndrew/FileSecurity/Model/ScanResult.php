<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Model;

/**
 * Represents the result of a file/buffer scan operation
 *
 * Contains all information about the scan including:
 * - Scan result code (0 = clean, >0 = malware count)
 * - List of detected malware
 * - File hashes (SHA1, SHA256)
 * - Scan metadata
 */
class ScanResult
{
    /**
     * Scan result code indicating clean file
     */
    public const RESULT_CLEAN = 0;

    /**
     * @var int Scan result code (0 = clean, >0 = number of malware found)
     */
    private int $scanResult;

    /**
     * @var string|null Scan ID from the API
     */
    private ?string $scanId;

    /**
     * @var string File name that was scanned
     */
    private string $fileName;

    /**
     * @var string|null Original file path (if scanning from file)
     */
    private ?string $filePath;

    /**
     * @var string|null SHA1 hash of the file
     */
    private ?string $fileSha1;

    /**
     * @var string|null SHA256 hash of the file
     */
    private ?string $fileSha256;

    /**
     * @var Malware[] Array of detected malware
     */
    private array $foundMalwares = [];

    /**
     * @var int Scan duration in milliseconds
     */
    private int $scanDuration = 0;

    /**
     * @var string|null Version of the scan engine
     */
    private ?string $scannerVersion;

    /**
     * @var array Raw response data
     */
    private array $rawResponse = [];

    /**
     * Create a new ScanResult
     *
     * @param int $scanResult Scan result code
     * @param string $fileName File name
     * @param string|null $filePath Original file path
     */
    public function __construct(int $scanResult, string $fileName, ?string $filePath = null)
    {
        $this->scanResult = $scanResult;
        $this->fileName = $fileName;
        $this->filePath = $filePath;
    }

    /**
     * Check if the file is clean (no malware detected)
     *
     * @return bool
     */
    public function isClean(): bool
    {
        return $this->scanResult === self::RESULT_CLEAN;
    }

    /**
     * Check if malware was detected
     *
     * @return bool
     */
    public function hasMalware(): bool
    {
        return $this->scanResult > self::RESULT_CLEAN;
    }

    /**
     * Get the scan result code
     *
     * @return int
     */
    public function getScanResult(): int
    {
        return $this->scanResult;
    }

    /**
     * Get the scan ID
     *
     * @return string|null
     */
    public function getScanId(): ?string
    {
        return $this->scanId;
    }

    /**
     * Get the file name
     *
     * @return string
     */
    public function getFileName(): string
    {
        return $this->fileName;
    }

    /**
     * Get the file path
     *
     * @return string|null
     */
    public function getFilePath(): ?string
    {
        return $this->filePath;
    }

    /**
     * Get the SHA1 hash
     *
     * @return string|null
     */
    public function getFileSha1(): ?string
    {
        return $this->fileSha1;
    }

    /**
     * Get the SHA256 hash
     *
     * @return string|null
     */
    public function getFileSha256(): ?string
    {
        return $this->fileSha256;
    }

    /**
     * Get the list of found malware
     *
     * @return Malware[]
     */
    public function getFoundMalwares(): array
    {
        return $this->foundMalwares;
    }

    /**
     * Get the number of malware found
     *
     * @return int
     */
    public function getMalwareCount(): int
    {
        return count($this->foundMalwares);
    }

    /**
     * Get the scan duration in milliseconds
     *
     * @return int
     */
    public function getScanDuration(): int
    {
        return $this->scanDuration;
    }

    /**
     * Get the scanner version
     *
     * @return string|null
     */
    public function getScannerVersion(): ?string
    {
        return $this->scannerVersion;
    }

    /**
     * Get the raw API response
     *
     * @return array
     */
    public function getRawResponse(): array
    {
        return $this->rawResponse;
    }

    /**
     * Convert to array
     *
     * @return array
     */
    public function toArray(): array
    {
        return [
            'scanResult' => $this->scanResult,
            'scanId' => $this->scanId,
            'fileName' => $this->fileName,
            'filePath' => $this->filePath,
            'fileSha1' => $this->fileSha1,
            'fileSha256' => $this->fileSha256,
            'foundMalwares' => array_map(fn(Malware $m) => $m->toArray(), $this->foundMalwares),
            'scanDuration' => $this->scanDuration,
            'scannerVersion' => $this->scannerVersion,
            'isClean' => $this->isClean(),
        ];
    }

    /**
     * Create ScanResult from API response
     *
     * @param array $response API response data
     * @param string $fileName File name
     * @param string|null $filePath Original file path
     * @return self
     */
    public static function fromResponse(array $response, string $fileName, ?string $filePath = null): self
    {
        $scanResult = $response['scanResult'] ?? $response['scan_result'] ?? 0;

        $result = new self((int) $scanResult, $fileName, $filePath);

        $result->scanId = $response['scanId'] ?? $response['scan_id'] ?? null;
        $result->fileSha1 = $response['fileSHA1'] ?? $response['file_sha1'] ?? null;
        $result->fileSha256 = $response['fileSHA256'] ?? $response['file_sha256'] ?? null;
        $result->scannerVersion = $response['scannerVersion'] ?? $response['scanner_version'] ?? null;
        $result->rawResponse = $response;

        // Parse found malwares
        $malwares = $response['foundMalwares'] ?? $response['found_malwares'] ?? [];
        foreach ($malwares as $malwareData) {
            $result->foundMalwares[] = Malware::fromResponse($malwareData);
        }

        // Calculate duration if timestamps provided
        if (isset($response['scanStartTimestamp']) && isset($response['scanEndTimestamp'])) {
            $result->scanDuration = (int) (
                ($response['scanEndTimestamp'] - $response['scanStartTimestamp']) * 1000
            );
        } elseif (isset($response['scanDuration'])) {
            $result->scanDuration = (int) $response['scanDuration'];
        }

        return $result;
    }

    /**
     * String representation
     *
     * @return string
     */
    public function __toString(): string
    {
        if ($this->isClean()) {
            return sprintf("Scan Result: CLEAN - %s", $this->fileName);
        }

        return sprintf(
            "Scan Result: MALWARE DETECTED (%d) - %s - Threats: %s",
            $this->scanResult,
            $this->fileName,
            implode(', ', array_map(fn(Malware $m) => $m->getMalwareName(), $this->foundMalwares))
        );
    }
}
