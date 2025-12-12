<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Exception;

/**
 * Exception thrown when connection to the API fails
 *
 * This exception is thrown when:
 * - Network connectivity issues occur
 * - DNS resolution fails
 * - TLS/SSL handshake fails
 * - Server is unreachable
 */
class ConnectionException extends AmaasException
{
    /**
     * @var string|null The endpoint that failed to connect
     */
    protected ?string $endpoint = null;

    /**
     * Create a new connection exception
     *
     * @param string $message Error message
     * @param string|null $endpoint The endpoint that failed
     * @param int $code Error code
     * @param \Throwable|null $previous Previous exception
     */
    public function __construct(
        string $message = '',
        ?string $endpoint = null,
        int $code = 0,
        ?\Throwable $previous = null
    ) {
        parent::__construct($message, $code, $previous);
        $this->endpoint = $endpoint;
    }

    /**
     * Get the endpoint that failed to connect
     *
     * @return string|null
     */
    public function getEndpoint(): ?string
    {
        return $this->endpoint;
    }

    /**
     * Create exception for DNS resolution failure
     *
     * @param string $host The hostname that failed to resolve
     * @return static
     */
    public static function dnsResolutionFailed(string $host): static
    {
        return new static(
            "Failed to resolve hostname: {$host}",
            $host
        );
    }

    /**
     * Create exception for connection refused
     *
     * @param string $endpoint The endpoint that refused connection
     * @return static
     */
    public static function connectionRefused(string $endpoint): static
    {
        return new static(
            "Connection refused by server: {$endpoint}",
            $endpoint
        );
    }

    /**
     * Create exception for TLS/SSL error
     *
     * @param string $endpoint The endpoint
     * @param string|null $sslError The SSL error message
     * @return static
     */
    public static function tlsError(string $endpoint, ?string $sslError = null): static
    {
        $message = "TLS/SSL handshake failed for: {$endpoint}";
        if ($sslError) {
            $message .= " - {$sslError}";
        }
        return new static($message, $endpoint);
    }

    /**
     * Create exception for network unreachable
     *
     * @param string $endpoint The endpoint
     * @return static
     */
    public static function networkUnreachable(string $endpoint): static
    {
        return new static(
            "Network unreachable: {$endpoint}",
            $endpoint
        );
    }

    /**
     * Create exception from cURL error
     *
     * @param int $errno cURL error number
     * @param string $error cURL error message
     * @param string|null $endpoint The endpoint
     * @return static
     */
    public static function fromCurlError(int $errno, string $error, ?string $endpoint = null): static
    {
        $message = "Connection failed";
        if ($endpoint) {
            $message .= " to {$endpoint}";
        }
        $message .= ": {$error} (cURL error {$errno})";

        return new static($message, $endpoint, $errno);
    }
}
