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
 * Event for file-related operations
 */
class FileEvent extends Event
{
    // Event names for file operations
    public const BEFORE_CREATE_FILE = 'file.create.before';
    public const AFTER_CREATE_FILE = 'file.create.after';
    public const BEFORE_CREATE_DIR = 'dir.create.before';
    public const AFTER_CREATE_DIR = 'dir.create.after';
    public const BEFORE_DELETE_FILE = 'file.delete.before';
    public const AFTER_DELETE_FILE = 'file.delete.after';
    public const BEFORE_DELETE_DIR = 'dir.delete.before';
    public const AFTER_DELETE_DIR = 'dir.delete.after';
    public const BEFORE_COPY_FILE = 'file.copy.before';
    public const AFTER_COPY_FILE = 'file.copy.after';
    public const BEFORE_COPY_DIR = 'dir.copy.before';
    public const AFTER_COPY_DIR = 'dir.copy.after';
    public const BEFORE_MOVE = 'file.move.before';
    public const AFTER_MOVE = 'file.move.after';
    public const BEFORE_RENAME = 'file.rename.before';
    public const AFTER_RENAME = 'file.rename.after';
    public const BEFORE_CHMOD = 'file.chmod.before';
    public const AFTER_CHMOD = 'file.chmod.after';
    public const BEFORE_SAVE_CONTENT = 'file.save_content.before';
    public const AFTER_SAVE_CONTENT = 'file.save_content.after';
    public const DIRECTORY_LIST = 'dir.list';
    public const CHANGE_DIRECTORY = 'dir.change';
    public const BEFORE_ZIP = 'file.zip.before';
    public const AFTER_ZIP = 'file.zip.after';
    public const BEFORE_UNZIP = 'file.unzip.before';
    public const AFTER_UNZIP = 'file.unzip.after';

    /**
     * @var string|null The file/directory path
     */
    protected $path;

    /**
     * @var string|null The file/directory name
     */
    protected $name;

    /**
     * @var string|null The type (file or dir)
     */
    protected $type;

    /**
     * @var User|null The user performing the action
     */
    protected $user;

    public function __construct(
        string $eventName,
        ?string $path = null,
        ?string $name = null,
        ?string $type = null,
        ?User $user = null,
        array $additionalData = []
    ) {
        parent::__construct($eventName, array_merge([
            'path' => $path,
            'name' => $name,
            'type' => $type,
        ], $additionalData));

        $this->path = $path;
        $this->name = $name;
        $this->type = $type;
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

    public function getName(): ?string
    {
        return $this->name;
    }

    public function setName(?string $name): self
    {
        $this->name = $name;
        $this->data['name'] = $name;
        return $this;
    }

    public function getType(): ?string
    {
        return $this->type;
    }

    public function setType(?string $type): self
    {
        $this->type = $type;
        $this->data['type'] = $type;
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
}
