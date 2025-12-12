<?php
/**
 * Trend Micro Vision One File Security SDK for PHP - Tests
 *
 * @package TrendAndrew\FileSecurity\Tests
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Tests;

use PHPUnit\Framework\TestCase;
use TrendAndrew\FileSecurity\AmaasClient;
use TrendAndrew\FileSecurity\LogLevel;
use TrendAndrew\FileSecurity\Exception\InvalidArgumentException;
use TrendAndrew\FileSecurity\Model\ScanOptions;

/**
 * Test cases for AmaasClient
 */
class AmaasClientTest extends TestCase
{
    /**
     * Valid regions for testing
     */
    private const VALID_REGIONS = [
        'us-east-1',
        'eu-central-1',
        'ap-northeast-1',
        'ap-southeast-1',
        'ap-southeast-2',
        'ap-south-1',
        'me-central-1',
        'us-1',
        'eu-1',
        'jp-1',
        'sg-1',
        'au-1',
        'in-1',
    ];

    /**
     * Test client creation with valid region
     */
    public function testCreateClientWithValidRegion(): void
    {
        foreach (self::VALID_REGIONS as $region) {
            $client = new AmaasClient($region, 'test-api-key');
            $this->assertEquals($region, $client->getRegion());
            $client->close();
        }
    }

    /**
     * Test client creation with invalid region throws exception
     */
    public function testCreateClientWithInvalidRegionThrowsException(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Invalid region');

        new AmaasClient('invalid-region', 'test-api-key');
    }

    /**
     * Test client creation with empty region throws exception
     */
    public function testCreateClientWithEmptyRegionThrowsException(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Region cannot be empty');

        new AmaasClient('', 'test-api-key');
    }

    /**
     * Test client creation with empty API key throws exception
     */
    public function testCreateClientWithEmptyApiKeyThrowsException(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('API key cannot be empty');

        new AmaasClient('us-east-1', '');
    }

    /**
     * Test endpoint generation for different regions
     */
    public function testEndpointGeneration(): void
    {
        $testCases = [
            'us-east-1' => 'https://antimalware.us-1.cloudone.trendmicro.com',
            'eu-central-1' => 'https://antimalware.de-1.cloudone.trendmicro.com',
            'ap-northeast-1' => 'https://antimalware.jp-1.cloudone.trendmicro.com',
            'ap-southeast-1' => 'https://antimalware.sg-1.cloudone.trendmicro.com',
            'ap-southeast-2' => 'https://antimalware.au-1.cloudone.trendmicro.com',
            'ap-south-1' => 'https://antimalware.in-1.cloudone.trendmicro.com',
        ];

        foreach ($testCases as $region => $expectedEndpoint) {
            $client = new AmaasClient($region, 'test-api-key');
            $this->assertEquals($expectedEndpoint, $client->getEndpoint());
            $client->close();
        }
    }

    /**
     * Test initByRegion static factory method
     */
    public function testInitByRegion(): void
    {
        $client = AmaasClient::initByRegion('us-east-1', 'test-api-key');
        $this->assertInstanceOf(AmaasClient::class, $client);
        $this->assertEquals('us-east-1', $client->getRegion());
        $client->close();
    }

    /**
     * Test initByRegion with custom timeout
     */
    public function testInitByRegionWithCustomTimeout(): void
    {
        $client = AmaasClient::initByRegion('us-east-1', 'test-api-key', 600);
        $this->assertInstanceOf(AmaasClient::class, $client);
        $client->close();
    }

    /**
     * Test scanFile with non-existent file throws exception
     */
    public function testScanFileWithNonExistentFileThrowsException(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('File not found');

        try {
            $client->scanFile('/non/existent/file.txt');
        } finally {
            $client->close();
        }
    }

    /**
     * Test scanBuffer with empty identifier throws exception
     */
    public function testScanBufferWithEmptyIdentifierThrowsException(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Identifier cannot be empty');

        try {
            $client->scanBuffer('', 'test content');
        } finally {
            $client->close();
        }
    }

    /**
     * Test logging level setting
     */
    public function testSetLoggingLevel(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');

        // Should not throw exception
        $client->setLoggingLevel(LogLevel::DEBUG);
        $client->setLoggingLevel(LogLevel::INFO);
        $client->setLoggingLevel(LogLevel::WARN);
        $client->setLoggingLevel(LogLevel::ERROR);
        $client->setLoggingLevel(LogLevel::OFF);

        $this->assertTrue(true); // If we get here, test passed
        $client->close();
    }

    /**
     * Test custom logging callback
     */
    public function testConfigLoggingCallback(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');
        $logMessages = [];

        $client->configLoggingCallback(function ($level, $message) use (&$logMessages) {
            $logMessages[] = ['level' => $level, 'message' => $message];
        });

        // Should not throw exception
        $this->assertTrue(true);
        $client->close();
    }

    /**
     * Test MAX_FILE_SIZE constant
     */
    public function testMaxFileSizeConstant(): void
    {
        $this->assertEquals(524288000, AmaasClient::MAX_FILE_SIZE);
    }

    /**
     * Test MAX_TAGS constant
     */
    public function testMaxTagsConstant(): void
    {
        $this->assertEquals(8, AmaasClient::MAX_TAGS);
    }

    /**
     * Test MAX_TAG_LENGTH constant
     */
    public function testMaxTagLengthConstant(): void
    {
        $this->assertEquals(63, AmaasClient::MAX_TAG_LENGTH);
    }

    /**
     * Test DEFAULT_TIMEOUT constant
     */
    public function testDefaultTimeoutConstant(): void
    {
        $this->assertEquals(300, AmaasClient::DEFAULT_TIMEOUT);
    }

    /**
     * Test VERSION constant
     */
    public function testVersionConstant(): void
    {
        $this->assertEquals('1.0.0', AmaasClient::VERSION);
    }

    /**
     * Test options validation with too many tags
     */
    public function testOptionsValidationWithTooManyTags(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');

        // Create options with more than MAX_TAGS
        $options = new ScanOptions([
            'tag1', 'tag2', 'tag3', 'tag4', 'tag5',
            'tag6', 'tag7', 'tag8', 'tag9', // 9 tags - exceeds limit
        ]);

        // Create a temporary file for testing
        $tempFile = tempnam(sys_get_temp_dir(), 'test_');
        file_put_contents($tempFile, 'test content');

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Maximum 8 tags allowed');

        try {
            $client->scanFile($tempFile, $options);
        } finally {
            $client->close();
            @unlink($tempFile);
        }
    }

    /**
     * Test options validation with tag too long
     */
    public function testOptionsValidationWithTagTooLong(): void
    {
        $client = new AmaasClient('us-east-1', 'test-api-key');

        // Create options with a tag exceeding MAX_TAG_LENGTH
        $longTag = str_repeat('a', 64); // 64 chars - exceeds 63 limit
        $options = new ScanOptions([$longTag]);

        // Create a temporary file for testing
        $tempFile = tempnam(sys_get_temp_dir(), 'test_');
        file_put_contents($tempFile, 'test content');

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Tag exceeds maximum length');

        try {
            $client->scanFile($tempFile, $options);
        } finally {
            $client->close();
            @unlink($tempFile);
        }
    }
}
