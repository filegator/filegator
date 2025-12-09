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
 * Event for upload operations
 */
class UploadEvent extends Event
{
    // Event names for upload operations
    public const BEFORE_UPLOAD = 'upload.before';
    public const AFTER_UPLOAD = 'upload.after';
    public const CHUNK_UPLOADED = 'upload.chunk';
    public const UPLOAD_COMPLETE = 'upload.complete';
    public const UPLOAD_FAILED = 'upload.failed';

    /**
     * @var string|null The file name
     */
    protected $fileName;

    /**
     * @var string|null The destination path
     */
    protected $destination;

    /**
     * @var int|null The file size
     */
    protected $fileSize;

    /**
     * @var int|null Current chunk number
     */
    protected $chunkNumber;

    /**
     * @var int|null Total chunks
     */
    protected $totalChunks;

    /**
     * @var User|null The user performing the upload
     */
    protected $user;

    public function __construct(
        string $eventName,
        ?string $fileName = null,
        ?string $destination = null,
        ?int $fileSize = null,
        ?int $chunkNumber = null,
        ?int $totalChunks = null,
        ?User $user = null,
        array $additionalData = []
    ) {
        parent::__construct($eventName, array_merge([
            'file_name' => $fileName,
            'destination' => $destination,
            'file_size' => $fileSize,
            'chunk_number' => $chunkNumber,
            'total_chunks' => $totalChunks,
        ], $additionalData));

        $this->fileName = $fileName;
        $this->destination = $destination;
        $this->fileSize = $fileSize;
        $this->chunkNumber = $chunkNumber;
        $this->totalChunks = $totalChunks;
        $this->user = $user;
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

    public function getDestination(): ?string
    {
        return $this->destination;
    }

    public function setDestination(?string $destination): self
    {
        $this->destination = $destination;
        $this->data['destination'] = $destination;
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

    public function getChunkNumber(): ?int
    {
        return $this->chunkNumber;
    }

    public function getTotalChunks(): ?int
    {
        return $this->totalChunks;
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

    public function isLastChunk(): bool
    {
        return $this->chunkNumber !== null
            && $this->totalChunks !== null
            && $this->chunkNumber >= $this->totalChunks;
    }
}
