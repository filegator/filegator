<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Exception;

/**
 * Exception thrown when an operation times out
 *
 * This exception is thrown when:
 * - Connection timeout occurs
 * - Request timeout occurs
 * - Scan operation exceeds the configured timeout
 */
class TimeoutException extends AmaasException
{
    /**
     * @var int|null The timeout value in seconds
     */
    protected ?int $timeoutSeconds = null;

    /**
     * @var string|null The operation that timed out
     */
    protected ?string $operation = null;

    /**
     * Create a new timeout exception
     *
     * @param string $message Error message
     * @param int|null $timeoutSeconds The timeout value
     * @param string|null $operation The operation that timed out
     * @param \Throwable|null $previous Previous exception
     */
    public function __construct(
        string $message = '',
        ?int $timeoutSeconds = null,
        ?string $operation = null,
        ?\Throwable $previous = null
    ) {
        parent::__construct($message, 0, $previous);
        $this->timeoutSeconds = $timeoutSeconds;
        $this->operation = $operation;
    }

    /**
     * Get the timeout value in seconds
     *
     * @return int|null
     */
    public function getTimeoutSeconds(): ?int
    {
        return $this->timeoutSeconds;
    }

    /**
     * Get the operation that timed out
     *
     * @return string|null
     */
    public function getOperation(): ?string
    {
        return $this->operation;
    }

    /**
     * Create exception for connection timeout
     *
     * @param int $timeout The timeout value in seconds
     * @param string|null $endpoint The endpoint
     * @return static
     */
    public static function connectionTimeout(int $timeout, ?string $endpoint = null): static
    {
        $message = "Connection timed out after {$timeout} seconds";
        if ($endpoint) {
            $message .= " to {$endpoint}";
        }
        return new static($message, $timeout, 'connection');
    }

    /**
     * Create exception for request timeout
     *
     * @param int $timeout The timeout value in seconds
     * @return static
     */
    public static function requestTimeout(int $timeout): static
    {
        return new static(
            "Request timed out after {$timeout} seconds",
            $timeout,
            'request'
        );
    }

    /**
     * Create exception for scan timeout
     *
     * @param int $timeout The timeout value in seconds
     * @param string|null $fileName The file being scanned
     * @return static
     */
    public static function scanTimeout(int $timeout, ?string $fileName = null): static
    {
        $message = "Scan operation timed out after {$timeout} seconds";
        if ($fileName) {
            $message .= " for file: {$fileName}";
        }
        return new static($message, $timeout, 'scan');
    }
}
