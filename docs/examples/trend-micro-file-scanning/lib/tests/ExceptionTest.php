<?php
/**
 * Trend Micro Vision One File Security SDK for PHP - Tests
 *
 * @package TrendMicro\FileSecurity\Tests
 * @license MIT
 */

namespace TrendMicro\FileSecurity\Tests;

use PHPUnit\Framework\TestCase;
use TrendMicro\FileSecurity\Exception\AmaasException;
use TrendMicro\FileSecurity\Exception\InvalidArgumentException;
use TrendMicro\FileSecurity\Exception\AuthenticationException;
use TrendMicro\FileSecurity\Exception\ConnectionException;
use TrendMicro\FileSecurity\Exception\TimeoutException;

/**
 * Test cases for Exception classes
 */
class ExceptionTest extends TestCase
{
    /**
     * Test AmaasException creation
     */
    public function testAmaasExceptionCreation(): void
    {
        $exception = new AmaasException('Test error', 500, null, 'ERR001', ['key' => 'value']);

        $this->assertEquals('Test error', $exception->getMessage());
        $this->assertEquals(500, $exception->getCode());
        $this->assertEquals('ERR001', $exception->getErrorCode());
        $this->assertEquals(['key' => 'value'], $exception->getResponseData());
    }

    /**
     * Test AmaasException fromResponse
     */
    public function testAmaasExceptionFromResponse(): void
    {
        $response = [
            'message' => 'API error occurred',
            'code' => 'API_ERR',
        ];

        $exception = AmaasException::fromResponse($response);

        $this->assertEquals('API error occurred', $exception->getMessage());
        $this->assertEquals('API_ERR', $exception->getErrorCode());
        $this->assertEquals($response, $exception->getResponseData());
    }

    /**
     * Test InvalidArgumentException factory methods
     */
    public function testInvalidArgumentExceptionFactories(): void
    {
        // Invalid region
        $exception = InvalidArgumentException::invalidRegion('bad-region', ['us-east-1', 'eu-central-1']);
        $this->assertStringContainsString('bad-region', $exception->getMessage());
        $this->assertEquals('region', $exception->getArgumentName());
        $this->assertEquals('bad-region', $exception->getInvalidValue());

        // Empty API key
        $exception = InvalidArgumentException::emptyApiKey();
        $this->assertStringContainsString('API key', $exception->getMessage());
        $this->assertEquals('apiKey', $exception->getArgumentName());

        // File not found
        $exception = InvalidArgumentException::fileNotFound('/path/to/file.txt');
        $this->assertStringContainsString('/path/to/file.txt', $exception->getMessage());
        $this->assertEquals('filePath', $exception->getArgumentName());

        // File not readable
        $exception = InvalidArgumentException::fileNotReadable('/path/to/file.txt');
        $this->assertStringContainsString('not readable', $exception->getMessage());

        // File too large
        $exception = InvalidArgumentException::fileTooLarge(600000000, 500000000);
        $this->assertStringContainsString('600000000', $exception->getMessage());
        $this->assertStringContainsString('500000000', $exception->getMessage());

        // Too many tags
        $exception = InvalidArgumentException::tooManyTags(10, 8);
        $this->assertStringContainsString('Maximum 8 tags', $exception->getMessage());

        // Tag too long
        $exception = InvalidArgumentException::tagTooLong('verylongtag...', 63);
        $this->assertStringContainsString('maximum length', $exception->getMessage());
    }

    /**
     * Test AuthenticationException factory methods
     */
    public function testAuthenticationExceptionFactories(): void
    {
        // Invalid API key
        $exception = AuthenticationException::invalidApiKey();
        $this->assertEquals(401, $exception->getCode());
        $this->assertStringContainsString('Invalid API key', $exception->getMessage());

        // Expired API key
        $exception = AuthenticationException::expiredApiKey();
        $this->assertEquals(401, $exception->getCode());
        $this->assertStringContainsString('expired', $exception->getMessage());

        // Insufficient permissions
        $exception = AuthenticationException::insufficientPermissions('scan:write');
        $this->assertEquals(403, $exception->getCode());
        $this->assertStringContainsString('scan:write', $exception->getMessage());

        // From HTTP status
        $exception = AuthenticationException::fromHttpStatus(401);
        $this->assertEquals(401, $exception->getCode());

        $exception = AuthenticationException::fromHttpStatus(403, 'Custom message');
        $this->assertEquals(403, $exception->getCode());
        $this->assertEquals('Custom message', $exception->getMessage());
    }

    /**
     * Test ConnectionException factory methods
     */
    public function testConnectionExceptionFactories(): void
    {
        // DNS resolution failed
        $exception = ConnectionException::dnsResolutionFailed('api.example.com');
        $this->assertStringContainsString('api.example.com', $exception->getMessage());
        $this->assertEquals('api.example.com', $exception->getEndpoint());

        // Connection refused
        $exception = ConnectionException::connectionRefused('https://api.example.com');
        $this->assertStringContainsString('refused', $exception->getMessage());

        // TLS error
        $exception = ConnectionException::tlsError('https://api.example.com', 'Certificate expired');
        $this->assertStringContainsString('TLS', $exception->getMessage());
        $this->assertStringContainsString('Certificate expired', $exception->getMessage());

        // Network unreachable
        $exception = ConnectionException::networkUnreachable('https://api.example.com');
        $this->assertStringContainsString('unreachable', $exception->getMessage());

        // From cURL error
        $exception = ConnectionException::fromCurlError(7, 'Connection timed out', 'https://api.example.com');
        $this->assertEquals(7, $exception->getCode());
        $this->assertStringContainsString('cURL error 7', $exception->getMessage());
    }

    /**
     * Test TimeoutException factory methods
     */
    public function testTimeoutExceptionFactories(): void
    {
        // Connection timeout
        $exception = TimeoutException::connectionTimeout(30, 'https://api.example.com');
        $this->assertEquals(30, $exception->getTimeoutSeconds());
        $this->assertEquals('connection', $exception->getOperation());
        $this->assertStringContainsString('30 seconds', $exception->getMessage());

        // Request timeout
        $exception = TimeoutException::requestTimeout(300);
        $this->assertEquals(300, $exception->getTimeoutSeconds());
        $this->assertEquals('request', $exception->getOperation());

        // Scan timeout
        $exception = TimeoutException::scanTimeout(600, 'largefile.zip');
        $this->assertEquals(600, $exception->getTimeoutSeconds());
        $this->assertEquals('scan', $exception->getOperation());
        $this->assertStringContainsString('largefile.zip', $exception->getMessage());
    }

    /**
     * Test exception inheritance
     */
    public function testExceptionInheritance(): void
    {
        $this->assertInstanceOf(\Exception::class, new AmaasException());
        $this->assertInstanceOf(AmaasException::class, new InvalidArgumentException());
        $this->assertInstanceOf(AmaasException::class, new AuthenticationException());
        $this->assertInstanceOf(AmaasException::class, new ConnectionException());
        $this->assertInstanceOf(AmaasException::class, new TimeoutException());
    }
}
