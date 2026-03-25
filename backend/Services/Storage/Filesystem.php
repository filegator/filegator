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
use League\Flysystem\StorageAttributes;

class Filesystem implements Service
{
    protected $separator;

    protected $storage;

    protected $path_prefix;

    protected $adapter;

    public function init(array $config = [])
    {
        $this->separator = $config['separator'];
        $this->path_prefix = $this->separator;

        $adapter = $config['adapter'];
        $config = isset($config['config']) ? $config['config'] : [];

        $this->adapter = $adapter();
        $this->storage = new Flysystem($this->adapter, $config);
    }

    public function createDir(string $path, string $name)
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while ($this->hasContents($destination)) {
            $destination = $this->upcountName($destination);
        }

        return $this->storage->createDirectory($destination);
    }

    public function createFile(string $path, string $name)
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while ($this->storage->fileExists($destination)) {
            $destination = $this->upcountName($destination);
        }

        $this->storage->write($destination, '');
    }

    public function fileExists(string $path)
    {
        $path = $this->applyPathPrefix($path);

        return $this->storage->fileExists($path);
    }

    public function isDir(string $path)
    {
        $path = $this->applyPathPrefix($path);

        return $this->storage->directoryExists($path);
    }

    public function copyFile(string $source, string $destination)
    {
        $source = $this->applyPathPrefix($source);
        $destination = $this->joinPaths($this->applyPathPrefix($destination), $this->getBaseName($source));

        while ($this->storage->fileExists($destination)) {
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

        while ($this->hasContents($real_destination)) {
            $real_destination = $this->upcountName($real_destination);
        }

        $contents = iterator_to_array($this->storage->listContents($source, true));

        if (empty($contents)) {
            $this->storage->createDirectory($real_destination);
        }

        foreach ($contents as $file) {
            $source_path = $this->separator.ltrim($file->path(), $this->separator);
            $path = substr($source_path, strlen($source), strlen($source_path));

            if ($file->isDir()) {
                $this->storage->createDirectory($this->joinPaths($real_destination, $path));

                continue;
            }

            if ($file->isFile()) {
                $this->storage->copy($file->path(), $this->joinPaths($real_destination, $path));
            }
        }
    }

    public function deleteDir(string $path)
    {
        $fullPath = $this->applyPathPrefix($path);
        if (!$this->storage->directoryExists($fullPath)) {
            throw new \Exception('Directory not found: ' . $path);
        }
        return $this->storage->deleteDirectory($fullPath);
    }

    public function deleteFile(string $path)
    {
        $fullPath = $this->applyPathPrefix($path);
        if (!$this->storage->fileExists($fullPath)) {
            throw new \Exception('File not found: ' . $path);
        }
        return $this->storage->delete($fullPath);
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
            'filesize' => $this->getFileSize($path),
        ];
    }

    public function move(string $from, string $to): bool
    {
        $from = $this->applyPathPrefix($from);
        $to = $this->applyPathPrefix($to);

        while ($this->storage->fileExists($to)) {
            $to = $this->upcountName($to);
        }

        return $this->storage->move($from, $to) ? true : false;
    }

    public function rename(string $destination, string $from, string $to): bool
    {
        $from = $this->joinPaths($this->applyPathPrefix($destination), $from);
        $to = $this->joinPaths($this->applyPathPrefix($destination), $to);

        while ($this->storage->fileExists($to)) {
            $to = $this->upcountName($to);
        }

        return $this->storage->move($from, $to) ? true : false;
    }

    public function store(string $path, string $name, $resource, bool $overwrite = false): bool
    {
        $destination = $this->joinPaths($this->applyPathPrefix($path), $name);

        while ($this->storage->fileExists($destination)) {
            if ($overwrite) {
                $this->storage->delete($destination);
            } else {
                $destination = $this->upcountName($destination);
            }
        }

        try {
            $this->storage->writeStream($destination, $resource);
            return true;
        } catch (\Exception $e) {
            return false;
        }
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
        $path = $this->applyPathPrefix($path);
        $path = $this->normalizePath($path);
        $adapter = $this->adapter;
        
        $mainResult = $this->chmodItem($path, $permissions);
        if ($recursive !== null) {
            if (method_exists($adapter, 'setRecurseManually')) {
                $adapter->setRecurseManually(true); // this is needed for ftp driver
            }
            $contents = iterator_to_array($this->storage->listContents($path, true));
            foreach ($contents as $item) {
                try {
                    if ($item->isDir() && ($recursive == 'all' || $recursive == 'folders')) {
                        $this->chmodItem($item->path(), $permissions);
                    }
                    if ($item->isFile() && ($recursive == 'all' || $recursive == 'files')) {
                        $this->chmodItem($item->path(), $permissions);
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
        $adapter = $this->adapter;
        $adapterClass = get_class($adapter);
        
        switch ($adapterClass) {
            case 'League\Flysystem\Local\LocalFilesystemAdapter':
                // Get the absolute path using reflection since applyPathPrefix is private in 3.x
                $absolutePath = $this->getLocalAdapterPath($adapter, $path);
                return chmod($absolutePath, octdec($permissions));
            case 'League\Flysystem\Sftp\SftpAdapter':
                return $adapter->getConnection()->chmod($path, octdec($permissions));
            case 'Filegator\Services\Storage\Adapters\FilegatorFtp':
                return ftp_chmod($adapter->getConnection(), octdec($permissions), $path) !== false;
            default:
                throw new \Exception('Selected adapter does not support unix permissions');
        }
    }
    
    /**
     * Get absolute path for Local adapter
     */
    private function getLocalAdapterPath($adapter, string $path): string
    {
        if (method_exists($adapter, 'getRootPath')) {
            $root = $adapter->getRootPath();
        } else {
            try {
                $reflection = new \ReflectionClass($adapter);
                $property = $reflection->getProperty('rootLocation');
                $property->setAccessible(true);
                $root = $property->getValue($adapter);
            } catch (\Exception $e) {
                throw new \Exception('Unable to determine absolute path for Local adapter');
            }
        }
        return rtrim($root, '/') . '/' . ltrim($path, '/');
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
            $name = $this->getBaseName($entry->path());
            $userpath = $this->stripPathPrefix($entry->path());
            $size = $entry->isDir() ? 0 : ($entry->fileSize() ?? 0);
            $lastModified = $entry->lastModified();
            $timestamp = $lastModified instanceof \DateTimeInterface ? $lastModified->getTimestamp() : (int) $lastModified ?? 0;
            $permissions = $this->getPermissions($entry);

            $type = $entry->isDir() ? 'dir' : 'file';
            $collection->addFile($type, $userpath, $name, $size, $timestamp, $permissions);
        }

        if (! $recursive && $this->addSeparators($path) !== $this->separator) {
            $collection->addFile('back', $this->getParent($path), '..', 0, 0, -1);
        }

        return $collection;
    }
    
    protected function getPermissions(StorageAttributes $entry): int
    {
        $adapter = $this->adapter;
        $path = $entry->path();
        $adapterClass = get_class($adapter);

        switch ($adapterClass) {
            case 'League\Flysystem\Local\LocalFilesystemAdapter':
                $absolutePath = $this->getLocalAdapterPath($adapter, $path);
                $permissions = substr(sprintf('%o', fileperms($absolutePath)), -3);
                return (int) $permissions;

            case 'League\Flysystem\Sftp\SftpAdapter':
            case 'League\Flysystem\PhpseclibV2\SftpAdapter':
            case 'League\Flysystem\PhpseclibV3\SftpAdapter':
                $connection = null;

                if (method_exists($adapter, 'getConnection')) {
                    $connection = $adapter->getConnection();
                } else {
                    try {
                        $reflection = new \ReflectionClass($adapter);
                        if ($reflection->hasProperty('connectionProvider')) {
                            $prop = $reflection->getProperty('connectionProvider');
                            $prop->setAccessible(true);
                            $provider = $prop->getValue($adapter);
                            if ($provider && method_exists($provider, 'provideConnection')) {
                                $connection = $provider->provideConnection();
                            }
                        }
                    } catch (\Throwable $e) {
                        $connection = null;
                    }
                }

                if ($connection && method_exists($connection, 'stat')) {
                    foreach ([$this->applyPathPrefix($path), $path, ltrim($path, '/'), '/'.ltrim($path, '/')] as $statPath) {
                        if (! $statPath) {
                            continue;
                        }

                        $stat = $connection->stat($statPath);
                        if ($stat && isset($stat['permissions'])) {
                            return (int) substr(decoct($stat['permissions']), -3);
                        }
                    }
                }

                if (method_exists($entry, 'visibility') && $entry->visibility() !== null) {
                    $converter = new \League\Flysystem\UnixVisibility\PortableVisibilityConverter();
                    $permissions = $entry->isDir()
                        ? $converter->forDirectory($entry->visibility())
                        : $converter->forFile($entry->visibility());

                    return (int) substr(sprintf('%o', $permissions), -3);
                }

                return -1;

            case 'Filegator\Services\Storage\Adapters\FilegatorFtp':
                return isset($entry->extraMetadata()['permissions']) ? (int) $entry->extraMetadata()['permissions'] : -1;

            default:
                return -1;
        }
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
        if ($path == '..'
            || strpos($path, '..'.$this->separator) !== false
            || strpos($path, $this->separator.'..') !== false
        ) {
            $path = $this->separator;
        }

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
        $path1 = $this->escapeDots($path1);
        $path2 = $this->escapeDots($path2);

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

    private function escapeDots(string $path): string
    {
        $path = preg_replace('/\\\+\.{2,}/', '', $path);
        $path = preg_replace('/\.{2,}\\\+/', '', $path);
        $path = preg_replace('/\/+\.{2,}/', '', $path);
        $path = preg_replace('/\.{2,}\/+/', '', $path);

        return $path;
    }

    private function normalizePath(string $path): string
    {
        $path = str_replace('\\', '/', $path);
        $path = preg_replace('|/+|', '/', $path);
        
        if (strpos($path, '/') === 0) {
            $parts = array_filter(explode('/', substr($path, 1)), 'strlen');
            $path = '/' . implode('/', $parts);
        } else {
            $parts = array_filter(explode('/', $path), 'strlen');
            $path = implode('/', $parts);
        }

        return $path;
    }

    private function hasContents(string $path): bool
    {
        try {
            $contents = iterator_to_array($this->storage->listContents($path, true), false);
            return ! empty($contents);
        } catch (\Exception $e) {
            return false;
        }
    }

    private function getFileSize(string $path): int
    {
        try {
            // Try to get file size directly via a meta query
            // In 3.x, we need to use an alternative approach
            $stream = $this->storage->readStream($path);
            if (is_resource($stream)) {
                $stat = fstat($stream);
                $size = $stat['size'] ?? 0;
                fclose($stream);
                return $size;
            }
            return 0;
        } catch (\Exception $e) {
            return 0;
        }
    }
}
