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
use League\Flysystem\Util;

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
        $config = $config['config'] ?? [];

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

        try {
            $metadata = $this->storage->getMetadata($path);
            if (isset($metadata['type'])) {
                return $metadata['type'] === 'dir';
            }
        } catch (\Exception $e) {
            // fallback
        }

        return $this->storage->getSize($path) === false;
    }

    public function copyFile(string $source, string $destination, bool $overwrite = false)
    {
        $source = $this->applyPathPrefix($source);
        $destination = $this->joinPaths($this->applyPathPrefix($destination), $this->getBaseName($source));

        if ($this->storage->has($destination)) {
            if ($overwrite) {
                $this->storage->delete($destination);
            } else {
                while ($this->storage->has($destination)) {
                    $destination = $this->upcountName($destination);
                }
            }
        }

        return $this->storage->copy($source, $destination);
    }

    public function copyDir(string $source, string $destination, bool $overwrite = false)
    {
        $source = $this->applyPathPrefix($this->addSeparators($source));
        $destination = $this->applyPathPrefix($this->addSeparators($destination));
        $source_dir = $this->getBaseName($source);
        $real_destination = $this->joinPaths($destination, $source_dir);

        if ($this->storage->has($real_destination)) {
            if ($overwrite) {
                return $this->mergeDir($source, $real_destination, true, false);
            }

            while ($this->storage->has($real_destination)) {
                $real_destination = $this->upcountName($real_destination);
            }
        }

        return $this->mergeDir($source, $real_destination, false, false);
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
            'filesize' => $this->storage->getSize($path),
        ];
    }

    public function move(string $from, string $to, bool $overwrite = false): bool
    {
        $from = $this->applyPathPrefix($from);
        $to = $this->applyPathPrefix($to);

        if (rtrim($from, $this->separator) === rtrim($to, $this->separator)) {
            return true;
        }

        // Prevent moving a directory into itself or its subdirectories
        $fromNormalized = $this->addSeparators($from);
        $toNormalized = $this->addSeparators($to);
        if ($this->isDir($this->stripPathPrefix($from)) && strpos($toNormalized, $fromNormalized) === 0) {
            throw new \Exception('Cannot move directory into itself or its subdirectory');
        }

        if ($this->storage->has($to)) {
            if ($overwrite) {
                $fromIsDir = $this->isDir($this->stripPathPrefix($from));
                $toIsDir = $this->isDir($this->stripPathPrefix($to));

                if ($fromIsDir && $toIsDir) {
                    return $this->mergeDir($from, $to, true, true);
                }

                if ($toIsDir) {
                    $this->storage->deleteDir($to);
                } else {
                    $this->storage->delete($to);
                }
            } else {
                while ($this->storage->has($to)) {
                    $to = $this->upcountName($to);
                }
            }
        }

        return $this->storage->rename($from, $to);
    }

    /**
     * Recursively merge a directory into a destination directory.
     *
     * @param string $source Source directory path (with prefix)
     * @param string $destination Target directory path (with prefix)
     * @param bool $overwrite Whether to overwrite existing files
     * @param bool $deleteSource Whether to delete source directory and files (move mode)
     * @return bool
     */
    protected function mergeDir(string $source, string $destination, bool $overwrite = true, bool $deleteSource = false): bool
    {
        $source = $this->addSeparators($source);
        $destination = $this->addSeparators($destination);

        if (!$this->storage->has($destination)) {
            $this->storage->createDir($destination);
        }

        $contents = $this->storage->listContents($source, true);

        if (empty($contents)) {
            if ($deleteSource) {
                $this->storage->deleteDir($source);
            }
            return true;
        }

        foreach ($contents as $file) {
            $source_path = $this->separator . ltrim($file['path'], $this->separator);
            $path = substr($source_path, strlen($source));
            $target_path = $this->joinPaths($destination, $path);

            if ($file['type'] == 'dir') {
                if (!$this->storage->has($target_path)) {
                    $this->storage->createDir($target_path);
                }
                continue;
            }

            if ($file['type'] == 'file') {
                if ($this->storage->has($target_path)) {
                    if ($overwrite) {
                        $this->storage->delete($target_path);
                        if ($deleteSource) {
                            $this->storage->rename($file['path'], $target_path);
                        } else {
                            $this->storage->copy($file['path'], $target_path);
                        }
                    } else {
                        while ($this->storage->has($target_path)) {
                            $target_path = $this->upcountName($target_path);
                        }
                        if ($deleteSource) {
                            $this->storage->rename($file['path'], $target_path);
                        } else {
                            $this->storage->copy($file['path'], $target_path);
                        }
                    }
                } else {
                    if ($deleteSource) {
                        $this->storage->rename($file['path'], $target_path);
                    } else {
                        $this->storage->copy($file['path'], $target_path);
                    }
                }
            }
        }

        if ($deleteSource) {
            $this->storage->deleteDir($source);
        }

        return true;
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
                $this->storage->delete($destination);
            } else {
                $destination = $this->upcountName($destination);
            }
        }

        return $this->storage->putStream($destination, $resource);
    }

    /**
     * Change file permissions one item, with optional recursion
     *
     * @param string $path
     * @param int $permissions
     * @param null|'all'|'folders'|'files' $recursive
     * @return bool
     * @throws \Exception
     */
    public function chmod(string $path, int $permissions, string $recursive = null)
    {
        // block special permission bits (setuid, setgid, sticky)
        if ($permissions < 0 || $permissions > 777) {
            throw new \Exception('Invalid permission, must be between 0 and 777');
        }

        // ensure every digit is 0-7
        if (!preg_match('/^[0-7]{1,3}$/', (string) $permissions)) {
            throw new \Exception('Invalid permission value. Must be between 0 and 777.');
        }

        $path = $this->applyPathPrefix($path);
        $path = Util::normalizePath($path);
        $adapter = $this->storage->getAdapter();

        $mainResult = $this->chmodItem($path, $permissions);
        if ($recursive !== null) {
            if (method_exists($adapter, 'setRecurseManually')) {
                $adapter->setRecurseManually(true); // this is needed for ftp driver
            }
            $contents = $this->storage->listContents($path, true);
            foreach ($contents as $item) {
                try {
                    if ($item['type'] == 'dir' && ($recursive == 'all' || $recursive == 'folders')) {
                        $this->chmodItem($item['path'], $permissions);
                    }
                    if ($item['type'] == 'file' && ($recursive == 'all' || $recursive == 'files')) {
                        $this->chmodItem($item['path'], $permissions);
                    }
                } catch (\Exception $e) {
                    continue;
                }
            }
        }

        return $mainResult;
    }
    /**
     * Change file permissions for a single item
     *
     * @param string $path
     * @param int $permissions
     * @return bool
     * @throws \Exception
     */
    public function chmodItem(string $path, int $permissions)
    {
        $adapter = $this->storage->getAdapter();

        switch (get_class($adapter)) {
            case 'League\Flysystem\Adapter\Local':
                $absolutePath = $adapter->applyPathPrefix($path);
                return chmod($absolutePath, octdec($permissions));
                break;
            case 'League\Flysystem\Sftp\SftpAdapter':
                return $adapter->getConnection()->chmod($path, octdec($permissions));
                break;
            case 'Filegator\Services\Storage\Adapters\FilegatorFtp':
                return ftp_chmod($adapter->getConnection(), octdec($permissions), $path) !== false;
                break;
            default:
                throw new \Exception('Selected adapter does not support unix permissions');
                break;
        }
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
            $dirname = $entry['dirname'] ?? $path;
            $size = $entry['size'] ?? 0;
            $timestamp = $entry['timestamp'] ?? 0;
            $permissions = $this->getPermissions($entry);

            $collection->addFile($entry['type'], $userpath, $name, $size, $timestamp, $permissions);
        }

        if (! $recursive && $this->addSeparators($path) !== $this->separator) {
            $collection->addFile('back', $this->getParent($path), '..', 0, 0, -1);
        }

        return $collection;
    }

    protected function getPermissions(array $entry): int
    {
        $adapter = $this->storage->getAdapter();
        $path = $entry['path'];

        switch (get_class($adapter)) {
            case 'League\Flysystem\Adapter\Local':
                $path = $adapter->applyPathPrefix($path); // get the full path
                $permissions = substr(sprintf('%o', fileperms($path)), -3);
                return $permissions;
                break;
            case 'League\Flysystem\Sftp\SftpAdapter':
                $stat = $adapter->getConnection()->stat($path);
                return $stat && isset($stat['permissions']) ? substr(decoct($stat['permissions']), -3) : -1;
                break;
            case 'Filegator\Services\Storage\Adapters\FilegatorFtp':
                return $entry['permissions'] ?? -1;
                break;
        }
        return -1;
    }

    protected function upcountCallback($matches)
    {
        $index = isset($matches[1]) ? intval($matches[1]) + 1 : 1;
        $ext = $matches[2] ?? '';

        return ' (' . $index . ')' . $ext;
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
        if ($path == '..'
            || strpos($path, '..' . $this->separator) !== false
            || strpos($path, $this->separator . '..') !== false
        ) {
            $path = $this->separator;
        }

        return $this->joinPaths($this->getPathPrefix(), $path);
    }

    private function stripPathPrefix(string $path): string
    {
        $path = $this->separator . ltrim($path, $this->separator);

        if (substr($path, 0, strlen($this->getPathPrefix())) == $this->getPathPrefix()) {
            $path = $this->separator . substr($path, strlen($this->getPathPrefix()));
        }

        return $path;
    }

    private function addSeparators(string $dir): string
    {
        if (! $dir || $dir == $this->separator || ! trim($dir, $this->separator)) {
            return $this->separator;
        }

        return $this->separator . trim($dir, $this->separator) . $this->separator;
    }

    private function joinPaths(string $path1, string $path2): string
    {
        $path1 = $this->escapeDots($path1);
        $path2 = $this->escapeDots($path2);

        if (! $path2 || ! trim($path2, $this->separator)) {
            return $this->addSeparators($path1);
        }

        return $this->addSeparators($path1) . ltrim($path2, $this->separator);
    }

    private function getParent(string $dir): string
    {
        if (! $dir || $dir == $this->separator || ! trim($dir, $this->separator)) {
            return $this->separator;
        }

        $tmp = explode($this->separator, trim($dir, $this->separator));
        array_pop($tmp);

        return $this->separator . trim(implode($this->separator, $tmp), $this->separator);
    }

    private function getBaseName(string $path): string
    {
        if (! $path || $path == $this->separator || ! trim($path, $this->separator)) {
            return $this->separator;
        }

        $tmp = explode($this->separator, trim($path, $this->separator));

        return  (string) array_pop($tmp);
    }

    private function escapeDots(string $path): string
    {
        $path = preg_replace('/\\\+\.{2,}/', '', $path);
        $path = preg_replace('/\.{2,}\\\+/', '', $path);
        $path = preg_replace('/\/+\.{2,}/', '', $path);
        $path = preg_replace('/\.{2,}\/+/', '', $path);
        $path = preg_replace('#(^|[\\\\/])\.\.($|[\\\\/])#', '$1$2', $path);

        return $path;
    }
}
