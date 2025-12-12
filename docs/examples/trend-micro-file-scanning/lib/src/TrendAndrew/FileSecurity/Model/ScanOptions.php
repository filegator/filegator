<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Model;

/**
 * Scan options for file/buffer scanning operations
 *
 * Allows customization of scan behavior including:
 * - Tags for categorizing/tracking scans
 * - PML (Predictive Machine Learning) settings
 * - Feedback settings
 */
class ScanOptions
{
    /**
     * @var array Tags for categorizing/tracking scans
     */
    private array $tags = [];

    /**
     * @var bool Enable Predictive Machine Learning (default: true)
     */
    private bool $pml = true;

    /**
     * @var bool Enable feedback to Trend Micro (default: true)
     */
    private bool $feedback = true;

    /**
     * Create new scan options
     *
     * @param array $tags Initial tags
     */
    public function __construct(array $tags = [])
    {
        $this->tags = $tags;
    }

    /**
     * Get the tags
     *
     * @return array
     */
    public function getTags(): array
    {
        return $this->tags;
    }

    /**
     * Set the tags
     *
     * @param array $tags Array of tag strings
     * @return self
     */
    public function setTags(array $tags): self
    {
        $this->tags = $tags;
        return $this;
    }

    /**
     * Add a tag
     *
     * @param string $tag Tag to add
     * @return self
     */
    public function addTag(string $tag): self
    {
        if (!in_array($tag, $this->tags, true)) {
            $this->tags[] = $tag;
        }
        return $this;
    }

    /**
     * Check if PML is enabled
     *
     * @return bool
     */
    public function isPmlEnabled(): bool
    {
        return $this->pml;
    }

    /**
     * Enable or disable PML
     *
     * @param bool $enabled Whether to enable PML
     * @return self
     */
    public function setPml(bool $enabled): self
    {
        $this->pml = $enabled;
        return $this;
    }

    /**
     * Check if feedback is enabled
     *
     * @return bool
     */
    public function isFeedbackEnabled(): bool
    {
        return $this->feedback;
    }

    /**
     * Enable or disable feedback
     *
     * @param bool $enabled Whether to enable feedback
     * @return self
     */
    public function setFeedback(bool $enabled): self
    {
        $this->feedback = $enabled;
        return $this;
    }

    /**
     * Convert to array for API request
     *
     * @return array
     */
    public function toArray(): array
    {
        return [
            'tags' => $this->tags,
            'pml' => $this->pml,
            'feedback' => $this->feedback,
        ];
    }

    /**
     * Create ScanOptions from array
     *
     * @param array $data Options data
     * @return self
     */
    public static function fromArray(array $data): self
    {
        $options = new self($data['tags'] ?? []);

        if (isset($data['pml'])) {
            $options->setPml((bool) $data['pml']);
        }

        if (isset($data['feedback'])) {
            $options->setFeedback((bool) $data['feedback']);
        }

        return $options;
    }
}
