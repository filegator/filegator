<?php
/**
 * Trend Micro Vision One File Security SDK for PHP - Tests
 *
 * @package TrendMicro\FileSecurity\Tests
 * @license MIT
 */

namespace TrendMicro\FileSecurity\Tests;

use PHPUnit\Framework\TestCase;
use TrendMicro\FileSecurity\Model\ScanOptions;

/**
 * Test cases for ScanOptions
 */
class ScanOptionsTest extends TestCase
{
    /**
     * Test default options
     */
    public function testDefaultOptions(): void
    {
        $options = new ScanOptions();

        $this->assertEmpty($options->getTags());
        $this->assertTrue($options->isPmlEnabled());
        $this->assertTrue($options->isFeedbackEnabled());
    }

    /**
     * Test options with initial tags
     */
    public function testOptionsWithInitialTags(): void
    {
        $tags = ['tag1', 'tag2', 'tag3'];
        $options = new ScanOptions($tags);

        $this->assertEquals($tags, $options->getTags());
    }

    /**
     * Test setTags method
     */
    public function testSetTags(): void
    {
        $options = new ScanOptions(['initial']);
        $options->setTags(['new1', 'new2']);

        $this->assertEquals(['new1', 'new2'], $options->getTags());
    }

    /**
     * Test addTag method
     */
    public function testAddTag(): void
    {
        $options = new ScanOptions(['tag1']);
        $options->addTag('tag2');
        $options->addTag('tag3');

        $this->assertEquals(['tag1', 'tag2', 'tag3'], $options->getTags());
    }

    /**
     * Test addTag doesn't add duplicates
     */
    public function testAddTagNoDuplicates(): void
    {
        $options = new ScanOptions(['tag1']);
        $options->addTag('tag1'); // Should not add duplicate
        $options->addTag('tag2');

        $this->assertEquals(['tag1', 'tag2'], $options->getTags());
    }

    /**
     * Test setPml method
     */
    public function testSetPml(): void
    {
        $options = new ScanOptions();

        $options->setPml(false);
        $this->assertFalse($options->isPmlEnabled());

        $options->setPml(true);
        $this->assertTrue($options->isPmlEnabled());
    }

    /**
     * Test setFeedback method
     */
    public function testSetFeedback(): void
    {
        $options = new ScanOptions();

        $options->setFeedback(false);
        $this->assertFalse($options->isFeedbackEnabled());

        $options->setFeedback(true);
        $this->assertTrue($options->isFeedbackEnabled());
    }

    /**
     * Test fluent interface
     */
    public function testFluentInterface(): void
    {
        $options = new ScanOptions();

        $result = $options
            ->setTags(['tag1'])
            ->addTag('tag2')
            ->setPml(false)
            ->setFeedback(false);

        $this->assertSame($options, $result);
        $this->assertEquals(['tag1', 'tag2'], $options->getTags());
        $this->assertFalse($options->isPmlEnabled());
        $this->assertFalse($options->isFeedbackEnabled());
    }

    /**
     * Test toArray method
     */
    public function testToArray(): void
    {
        $options = new ScanOptions(['tag1', 'tag2']);
        $options->setPml(false);
        $options->setFeedback(true);

        $array = $options->toArray();

        $this->assertEquals(['tag1', 'tag2'], $array['tags']);
        $this->assertFalse($array['pml']);
        $this->assertTrue($array['feedback']);
    }

    /**
     * Test fromArray factory method
     */
    public function testFromArray(): void
    {
        $data = [
            'tags' => ['tag1', 'tag2'],
            'pml' => false,
            'feedback' => true,
        ];

        $options = ScanOptions::fromArray($data);

        $this->assertEquals(['tag1', 'tag2'], $options->getTags());
        $this->assertFalse($options->isPmlEnabled());
        $this->assertTrue($options->isFeedbackEnabled());
    }

    /**
     * Test fromArray with missing values uses defaults
     */
    public function testFromArrayWithDefaults(): void
    {
        $options = ScanOptions::fromArray([]);

        $this->assertEmpty($options->getTags());
        $this->assertTrue($options->isPmlEnabled());
        $this->assertTrue($options->isFeedbackEnabled());
    }

    /**
     * Test fromArray with partial data
     */
    public function testFromArrayPartial(): void
    {
        $data = [
            'tags' => ['only-tags'],
        ];

        $options = ScanOptions::fromArray($data);

        $this->assertEquals(['only-tags'], $options->getTags());
        $this->assertTrue($options->isPmlEnabled());
        $this->assertTrue($options->isFeedbackEnabled());
    }
}
