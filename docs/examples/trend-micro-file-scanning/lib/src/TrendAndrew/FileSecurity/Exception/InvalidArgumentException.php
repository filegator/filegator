<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Exception;

/**
 * Exception thrown when invalid arguments are provided to SDK methods
 *
 * This exception is thrown for validation errors such as:
 * - Invalid region
 * - Empty API key
 * - Invalid file path
 * - File not found
 * - File too large
 * - Invalid options
 */
class InvalidArgumentException extends AmaasException
{
    /**
     * @var string|null The name of the invalid argument
     */
    protected ?string $argumentName = null;

    /**
     * @var mixed The invalid value that was provided
     */
    protected mixed $invalidValue = null;

    /**
     * Create a new invalid argument exception
     *
     * @param string $message Error message
     * @param string|null $argumentName Name of the invalid argument
     * @param mixed $invalidValue The invalid value
     * @param \Throwable|null $previous Previous exception
     */
    public function __construct(
        string $message = '',
        ?string $argumentName = null,
        mixed $invalidValue = null,
        ?\Throwable $previous = null
    ) {
        parent::__construct($message, 0, $previous);
        $this->argumentName = $argumentName;
        $this->invalidValue = $invalidValue;
    }

    /**
     * Get the name of the invalid argument
     *
     * @return string|null
     */
    public function getArgumentName(): ?string
    {
        return $this->argumentName;
    }

    /**
     * Get the invalid value
     *
     * @return mixed
     */
    public function getInvalidValue(): mixed
    {
        return $this->invalidValue;
    }

    /**
     * Create exception for invalid region
     *
     * @param string $region The invalid region
     * @param array $validRegions List of valid regions
     * @return static
     */
    public static function invalidRegion(string $region, array $validRegions): static
    {
        $message = sprintf(
            "Invalid region: %s. Valid regions are: %s",
            $region,
            implode(', ', $validRegions)
        );
        return new static($message, 'region', $region);
    }

    /**
     * Create exception for empty API key
     *
     * @return static
     */
    public static function emptyApiKey(): static
    {
        return new static("API key cannot be empty", 'apiKey', '');
    }

    /**
     * Create exception for file not found
     *
     * @param string $filePath The file path that was not found
     * @return static
     */
    public static function fileNotFound(string $filePath): static
    {
        return new static("File not found: {$filePath}", 'filePath', $filePath);
    }

    /**
     * Create exception for file not readable
     *
     * @param string $filePath The file path that was not readable
     * @return static
     */
    public static function fileNotReadable(string $filePath): static
    {
        return new static("File not readable: {$filePath}", 'filePath', $filePath);
    }

    /**
     * Create exception for file too large
     *
     * @param int $fileSize The actual file size
     * @param int $maxSize The maximum allowed size
     * @return static
     */
    public static function fileTooLarge(int $fileSize, int $maxSize): static
    {
        $message = sprintf(
            "File size (%d bytes) exceeds maximum allowed size (%d bytes)",
            $fileSize,
            $maxSize
        );
        return new static($message, 'fileSize', $fileSize);
    }

    /**
     * Create exception for too many tags
     *
     * @param int $tagCount The actual tag count
     * @param int $maxTags The maximum allowed tags
     * @return static
     */
    public static function tooManyTags(int $tagCount, int $maxTags): static
    {
        $message = sprintf(
            "Maximum %d tags allowed, got %d",
            $maxTags,
            $tagCount
        );
        return new static($message, 'tags', $tagCount);
    }

    /**
     * Create exception for tag too long
     *
     * @param string $tag The tag that was too long
     * @param int $maxLength The maximum allowed length
     * @return static
     */
    public static function tagTooLong(string $tag, int $maxLength): static
    {
        $message = sprintf(
            "Tag exceeds maximum length of %d characters: %s",
            $maxLength,
            $tag
        );
        return new static($message, 'tag', $tag);
    }
}
