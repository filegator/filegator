<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Storage;

use Filegator\Utils\Collection;

class DirectoryCollection implements \JsonSerializable
{
    use Collection;

    protected $location;

    public function __construct($location)
    {
        $this->location = $location;
    }

    public function addFile(string $type, string $path, string $name, int $size, int $timestamp, int $permissions)
    {
        if (! in_array($type, ['dir', 'file', 'back'])) {
            throw new \Exception('Invalid file type.');
        }

        $this->add([
            'type' => $type,
            'path' => $path,
            'name' => $name,
            'size' => $size,
            'time' => $timestamp,
            'permissions' => $permissions,
        ]);
    }

    public function resetTimestamps($timestamp = 0)
    {
        foreach ($this->items as &$item) {
            $item['time'] = $timestamp;
        }
    }

    public function jsonSerialize()
    {
        $this->sortByValue('type');

        return [
            'location' => $this->location,
            'files' => $this->items,
        ];
    }
}
