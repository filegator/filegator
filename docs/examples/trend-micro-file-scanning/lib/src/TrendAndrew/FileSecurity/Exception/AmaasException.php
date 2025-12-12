<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Exception;

/**
 * Base exception for AMaaS SDK errors
 *
 * This is the parent class for all SDK-specific exceptions.
 * It provides additional context about API errors including
 * error codes and raw response data.
 */
class AmaasException extends \Exception
{
    /**
     * @var string|null Error code from the API
     */
    protected ?string $errorCode = null;

    /**
     * @var array|null Raw response data from the API
     */
    protected ?array $responseData = null;

    /**
     * Create a new AMaaS exception
     *
     * @param string $message Error message
     * @param int $code Error code
     * @param \Throwable|null $previous Previous exception
     * @param string|null $errorCode API error code
     * @param array|null $responseData Raw API response
     */
    public function __construct(
        string $message = '',
        int $code = 0,
        ?\Throwable $previous = null,
        ?string $errorCode = null,
        ?array $responseData = null
    ) {
        parent::__construct($message, $code, $previous);
        $this->errorCode = $errorCode;
        $this->responseData = $responseData;
    }

    /**
     * Get the API error code
     *
     * @return string|null
     */
    public function getErrorCode(): ?string
    {
        return $this->errorCode;
    }

    /**
     * Get the raw response data
     *
     * @return array|null
     */
    public function getResponseData(): ?array
    {
        return $this->responseData;
    }

    /**
     * Create exception from API response
     *
     * @param array $response API response data
     * @return static
     */
    public static function fromResponse(array $response): static
    {
        $message = $response['message'] ?? $response['error'] ?? 'Unknown API error';
        $errorCode = $response['code'] ?? $response['error_code'] ?? null;

        return new static($message, 0, null, $errorCode, $response);
    }
}
