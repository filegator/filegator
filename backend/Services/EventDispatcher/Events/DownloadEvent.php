<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\EventDispatcher\Events;

use Filegator\Services\EventDispatcher\Event;
use Filegator\Services\Auth\User;

/**
 * Event for download operations
 */
class DownloadEvent extends Event
{
    // Event names for download operations
    public const BEFORE_DOWNLOAD = 'download.before';
    public const AFTER_DOWNLOAD = 'download.after';
    public const BEFORE_BATCH_DOWNLOAD = 'download.batch.before';
    public const AFTER_BATCH_DOWNLOAD = 'download.batch.after';

    /**
     * @var string|null The file path
     */
    protected $path;

    /**
     * @var string|null The file name
     */
    protected $fileName;

    /**
     * @var int|null The file size
     */
    protected $fileSize;

    /**
     * @var array Items being downloaded (for batch downloads)
     */
    protected $items = [];

    /**
     * @var User|null The user performing the download
     */
    protected $user;

    public function __construct(
        string $eventName,
        ?string $path = null,
        ?string $fileName = null,
        ?int $fileSize = null,
        array $items = [],
        ?User $user = null,
        array $additionalData = []
    ) {
        parent::__construct($eventName, array_merge([
            'path' => $path,
            'file_name' => $fileName,
            'file_size' => $fileSize,
            'items' => $items,
        ], $additionalData));

        $this->path = $path;
        $this->fileName = $fileName;
        $this->fileSize = $fileSize;
        $this->items = $items;
        $this->user = $user;
    }

    public function getPath(): ?string
    {
        return $this->path;
    }

    public function setPath(?string $path): self
    {
        $this->path = $path;
        $this->data['path'] = $path;
        return $this;
    }

    public function getFileName(): ?string
    {
        return $this->fileName;
    }

    public function setFileName(?string $fileName): self
    {
        $this->fileName = $fileName;
        $this->data['file_name'] = $fileName;
        return $this;
    }

    public function getFileSize(): ?int
    {
        return $this->fileSize;
    }

    public function setFileSize(?int $fileSize): self
    {
        $this->fileSize = $fileSize;
        $this->data['file_size'] = $fileSize;
        return $this;
    }

    public function getItems(): array
    {
        return $this->items;
    }

    public function setItems(array $items): self
    {
        $this->items = $items;
        $this->data['items'] = $items;
        return $this;
    }

    public function getUser(): ?User
    {
        return $this->user;
    }

    public function setUser(?User $user): self
    {
        $this->user = $user;
        return $this;
    }

    public function isBatchDownload(): bool
    {
        return !empty($this->items);
    }
}
