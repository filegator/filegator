<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Storage;

use Filegator\Services\Service;
use League\Flysystem\Filesystem as Flysystem;

class Filesystem implements Service
{
    protected $separator;

    protected $storage;

    protected $path_prefix;

    public function init(array $config = [])
    {
        $this->separator = $config['separator'];
        $this->path_prefix = $this->separator;

        $adapter = $config['adapter'];
        $config = isset($config['config']) ? $config['config'] : [];

        $this->storage = new Flysystem($adapter(), $config);
    }

    public function createDir(string $path, string $name)
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while (! empty($this->storage->listContents($destination, true))) {
            $destination = $this->upcountName($destination);
        }

        return $this->storage->createDir($destination);
    }

    public function createFile(string $path, string $name)
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while ($this->storage->has($destination)) {
            $destination = $this->upcountName($destination);
        }

        $this->storage->put($destination, '');
    }

    public function fileExists(string $path)
    {
        $path = $this->applyPathPrefix($path);

        return $this->storage->has($path);
    }

    public function isDir(string $path)
    {
        $path = $this->applyPathPrefix($path);

        return $this->storage->getSize($path) === false;
    }

    public function copyFile(string $source, string $destination)
    {
        $source = $this->applyPathPrefix($source);
        $destination = $this->joinPaths($this->applyPathPrefix($destination), $this->getBaseName($source));

        while ($this->storage->has($destination)) {
            $destination = $this->upcountName($destination);
        }

        return $this->storage->copy($source, $destination);
    }

    public function copyDir(string $source, string $destination)
    {
        $source = $this->applyPathPrefix($this->addSeparators($source));
        $destination = $this->applyPathPrefix($this->addSeparators($destination));
        $source_dir = $this->getBaseName($source);
        $real_destination = $this->joinPaths($destination, $source_dir);

        while (! empty($this->storage->listContents($real_destination, true))) {
            $real_destination = $this->upcountName($real_destination);
        }

        $contents = $this->storage->listContents($source, true);

        if (empty($contents)) {
            $this->storage->createDir($real_destination);
        }

        foreach ($contents as $file) {
            $source_path = $this->separator.ltrim($file['path'], $this->separator);
            $path = substr($source_path, strlen($source), strlen($source_path));

            if ($file['type'] == 'dir') {
                $this->storage->createDir($this->joinPaths($real_destination, $path));

                continue;
            }

            if ($file['type'] == 'file') {
                $this->storage->copy($file['path'], $this->joinPaths($real_destination, $path));
            }
        }
    }

    public function deleteDir(string $path)
    {
        return $this->storage->deleteDir($this->applyPathPrefix($path));
    }

    public function deleteFile(string $path)
    {
        return $this->storage->delete($this->applyPathPrefix($path));
    }

    public function readStream(string $path): array
    {
        if ($this->isDir($path)) {
            throw new \Exception('Cannot stream directory');
        }

        $path = $this->applyPathPrefix($path);

        return [
            'filename' => $this->getBaseName($path),
            'stream' => $this->storage->readStream($path),
        ];
    }

    public function move(string $from, string $to): bool
    {
        $from = $this->applyPathPrefix($from);
        $to = $this->applyPathPrefix($to);

        while ($this->storage->has($to)) {
            $to = $this->upcountName($to);
        }

        return $this->storage->rename($from, $to);
    }

    public function rename(string $destination, string $from, string $to): bool
    {
        $from = $this->joinPaths($this->applyPathPrefix($destination), $from);
        $to = $this->joinPaths($this->applyPathPrefix($destination), $to);

        while ($this->storage->has($to)) {
            $to = $this->upcountName($to);
        }

        return $this->storage->rename($from, $to);
    }

    public function store(string $path, string $name, $resource, bool $overwrite = false): bool
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while ($this->storage->has($destination)) {
            if ($overwrite) {
                $this->deleteFile($destination);
            } else {
                $destination = $this->upcountName($destination);
            }
        }

        return $this->storage->putStream($destination, $resource);
    }

    public function setPathPrefix(string $path_prefix)
    {
        $this->path_prefix = $this->addSeparators($path_prefix);
    }

    public function getSeparator()
    {
        return $this->separator;
    }

    public function getPathPrefix(): string
    {
        return $this->path_prefix;
    }

    public function getDirectoryCollection(string $path, bool $recursive = false): DirectoryCollection
    {
        $collection = new DirectoryCollection($path);

        foreach ($this->storage->listContents($this->applyPathPrefix($path), $recursive) as $entry) {
            // By default only 'path' and 'type' is present

            $name = $this->getBaseName($entry['path']);
            $userpath = $this->stripPathPrefix($entry['path']);
            $dirname = isset($entry['dirname']) ? $entry['dirname'] : $path;
            $size = isset($entry['size']) ? $entry['size'] : 0;
            $timestamp = isset($entry['timestamp']) ? $entry['timestamp'] : 0;

            $collection->addFile($entry['type'], $userpath, $name, $size, $timestamp);
        }

        if (! $recursive && $this->addSeparators($path) !== $this->separator) {
            $collection->addFile('back', $this->getParent($path), '..', 0, 0);
        }

        return $collection;
    }

    protected function upcountCallback($matches)
    {
        $index = isset($matches[1]) ? intval($matches[1]) + 1 : 1;
        $ext = isset($matches[2]) ? $matches[2] : '';

        return ' ('.$index.')'.$ext;
    }

    protected function upcountName($name)
    {
        return preg_replace_callback(
            '/(?:(?: \(([\d]+)\))?(\.[^.]+))?$/',
            [$this, 'upcountCallback'],
            $name,
            1
        );
    }

    private function applyPathPrefix(string $path): string
    {
        return $this->joinPaths($this->getPathPrefix(), $path);
    }

    private function stripPathPrefix(string $path): string
    {
        $path = $this->separator.ltrim($path, $this->separator);

        if (substr($path, 0, strlen($this->getPathPrefix())) == $this->getPathPrefix()) {
            $path = $this->separator.substr($path, strlen($this->getPathPrefix()));
        }

        return $path;
    }

    private function addSeparators(string $dir): string
    {
        if (! $dir || $dir == $this->separator || ! trim($dir, $this->separator)) {
            return $this->separator;
        }

        return $this->separator.trim($dir, $this->separator).$this->separator;
    }

    private function joinPaths(string $path1, string $path2): string
    {
        if (! $path2 || ! trim($path2, $this->separator)) {
            return $this->addSeparators($path1);
        }

        return $this->addSeparators($path1).ltrim($path2, $this->separator);
    }

    private function getParent(string $dir): string
    {
        if (! $dir || $dir == $this->separator || ! trim($dir, $this->separator)) {
            return $this->separator;
        }

        $tmp = explode($this->separator, trim($dir, $this->separator));
        array_pop($tmp);

        return $this->separator.trim(implode($this->separator, $tmp), $this->separator);
    }

    private function getBaseName(string $path): string
    {
        if (! $path || $path == $this->separator || ! trim($path, $this->separator)) {
            return $this->separator;
        }

        $tmp = explode($this->separator, trim($path, $this->separator));

        return  (string) array_pop($tmp);
    }
}
