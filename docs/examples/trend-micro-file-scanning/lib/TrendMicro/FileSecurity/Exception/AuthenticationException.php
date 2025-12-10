<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendMicro\FileSecurity
 * @license MIT
 */

namespace TrendMicro\FileSecurity\Exception;

/**
 * Exception thrown when authentication fails
 *
 * This exception is thrown when:
 * - API key is invalid or expired
 * - API key lacks required permissions
 * - Authentication token is rejected by the server
 */
class AuthenticationException extends AmaasException
{
    /**
     * HTTP status code for authentication errors
     */
    public const HTTP_UNAUTHORIZED = 401;
    public const HTTP_FORBIDDEN = 403;

    /**
     * Create exception for invalid API key
     *
     * @return static
     */
    public static function invalidApiKey(): static
    {
        return new static(
            "Invalid API key. Please check your API key and try again.",
            self::HTTP_UNAUTHORIZED
        );
    }

    /**
     * Create exception for expired API key
     *
     * @return static
     */
    public static function expiredApiKey(): static
    {
        return new static(
            "API key has expired. Please generate a new API key.",
            self::HTTP_UNAUTHORIZED
        );
    }

    /**
     * Create exception for insufficient permissions
     *
     * @param string|null $permission The required permission
     * @return static
     */
    public static function insufficientPermissions(?string $permission = null): static
    {
        $message = "Insufficient permissions to perform this operation";
        if ($permission) {
            $message .= ": {$permission}";
        }
        return new static($message, self::HTTP_FORBIDDEN);
    }

    /**
     * Create exception from HTTP status code
     *
     * @param int $statusCode HTTP status code
     * @param string|null $message Optional message
     * @return static
     */
    public static function fromHttpStatus(int $statusCode, ?string $message = null): static
    {
        $defaultMessage = match ($statusCode) {
            self::HTTP_UNAUTHORIZED => 'Authentication failed. Please check your API key.',
            self::HTTP_FORBIDDEN => 'Access denied. Insufficient permissions.',
            default => 'Authentication error occurred.',
        };

        return new static($message ?? $defaultMessage, $statusCode);
    }
}
