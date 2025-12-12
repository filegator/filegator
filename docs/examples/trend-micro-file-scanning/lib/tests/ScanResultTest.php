<?php
/**
 * Trend Micro Vision One File Security SDK for PHP - Tests
 *
 * @package TrendAndrew\FileSecurity\Tests
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Tests;

use PHPUnit\Framework\TestCase;
use TrendAndrew\FileSecurity\Model\ScanResult;
use TrendAndrew\FileSecurity\Model\Malware;

/**
 * Test cases for ScanResult
 */
class ScanResultTest extends TestCase
{
    /**
     * Test clean scan result
     */
    public function testCleanScanResult(): void
    {
        $result = new ScanResult(0, 'test.txt');

        $this->assertTrue($result->isClean());
        $this->assertFalse($result->hasMalware());
        $this->assertEquals(0, $result->getScanResult());
        $this->assertEquals('test.txt', $result->getFileName());
        $this->assertEmpty($result->getFoundMalwares());
        $this->assertEquals(0, $result->getMalwareCount());
    }

    /**
     * Test scan result with malware detected
     */
    public function testScanResultWithMalware(): void
    {
        $response = [
            'scanResult' => 1,
            'scanId' => 'scan-12345',
            'fileSHA1' => 'da39a3ee5e6b4b0d3255bfef95601890afd80709',
            'fileSHA256' => 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
            'foundMalwares' => [
                [
                    'malwareName' => 'EICAR-Test-File',
                    'fileName' => 'eicar.txt',
                    'type' => 'virus',
                    'filter' => 'pattern',
                ],
            ],
        ];

        $result = ScanResult::fromResponse($response, 'eicar.txt', '/tmp/eicar.txt');

        $this->assertFalse($result->isClean());
        $this->assertTrue($result->hasMalware());
        $this->assertEquals(1, $result->getScanResult());
        $this->assertEquals('scan-12345', $result->getScanId());
        $this->assertEquals('eicar.txt', $result->getFileName());
        $this->assertEquals('/tmp/eicar.txt', $result->getFilePath());
        $this->assertEquals('da39a3ee5e6b4b0d3255bfef95601890afd80709', $result->getFileSha1());
        $this->assertEquals('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', $result->getFileSha256());
        $this->assertCount(1, $result->getFoundMalwares());
        $this->assertEquals(1, $result->getMalwareCount());

        $malware = $result->getFoundMalwares()[0];
        $this->assertInstanceOf(Malware::class, $malware);
        $this->assertEquals('EICAR-Test-File', $malware->getMalwareName());
    }

    /**
     * Test scan result with multiple malwares
     */
    public function testScanResultWithMultipleMalwares(): void
    {
        $response = [
            'scanResult' => 3,
            'foundMalwares' => [
                ['malwareName' => 'Malware1', 'fileName' => 'test.exe'],
                ['malwareName' => 'Malware2', 'fileName' => 'test.exe'],
                ['malwareName' => 'Malware3', 'fileName' => 'test.exe'],
            ],
        ];

        $result = ScanResult::fromResponse($response, 'test.exe');

        $this->assertTrue($result->hasMalware());
        $this->assertEquals(3, $result->getScanResult());
        $this->assertEquals(3, $result->getMalwareCount());
        $this->assertCount(3, $result->getFoundMalwares());
    }

    /**
     * Test toArray method
     */
    public function testToArray(): void
    {
        $response = [
            'scanResult' => 1,
            'scanId' => 'scan-123',
            'fileSHA1' => 'abc123',
            'foundMalwares' => [
                ['malwareName' => 'TestMalware', 'fileName' => 'test.exe'],
            ],
        ];

        $result = ScanResult::fromResponse($response, 'test.exe', '/path/to/test.exe');
        $array = $result->toArray();

        $this->assertEquals(1, $array['scanResult']);
        $this->assertEquals('scan-123', $array['scanId']);
        $this->assertEquals('test.exe', $array['fileName']);
        $this->assertEquals('/path/to/test.exe', $array['filePath']);
        $this->assertEquals('abc123', $array['fileSha1']);
        $this->assertFalse($array['isClean']);
        $this->assertCount(1, $array['foundMalwares']);
    }

    /**
     * Test string representation for clean result
     */
    public function testToStringClean(): void
    {
        $result = new ScanResult(0, 'clean_file.txt');
        $string = (string) $result;

        $this->assertStringContainsString('CLEAN', $string);
        $this->assertStringContainsString('clean_file.txt', $string);
    }

    /**
     * Test string representation for malware result
     */
    public function testToStringMalware(): void
    {
        $response = [
            'scanResult' => 1,
            'foundMalwares' => [
                ['malwareName' => 'TestVirus', 'fileName' => 'infected.exe'],
            ],
        ];

        $result = ScanResult::fromResponse($response, 'infected.exe');
        $string = (string) $result;

        $this->assertStringContainsString('MALWARE DETECTED', $string);
        $this->assertStringContainsString('infected.exe', $string);
        $this->assertStringContainsString('TestVirus', $string);
    }

    /**
     * Test scan duration calculation
     */
    public function testScanDuration(): void
    {
        $response = [
            'scanResult' => 0,
            'scanDuration' => 150,
        ];

        $result = ScanResult::fromResponse($response, 'test.txt');

        $this->assertEquals(150, $result->getScanDuration());
    }

    /**
     * Test raw response access
     */
    public function testRawResponse(): void
    {
        $response = [
            'scanResult' => 0,
            'customField' => 'custom value',
            'nested' => ['key' => 'value'],
        ];

        $result = ScanResult::fromResponse($response, 'test.txt');
        $raw = $result->getRawResponse();

        $this->assertEquals('custom value', $raw['customField']);
        $this->assertEquals(['key' => 'value'], $raw['nested']);
    }

    /**
     * Test RESULT_CLEAN constant
     */
    public function testResultCleanConstant(): void
    {
        $this->assertEquals(0, ScanResult::RESULT_CLEAN);
    }
}
