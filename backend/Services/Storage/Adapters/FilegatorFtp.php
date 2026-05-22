<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Storage\Adapters;

use League\Flysystem\FilesystemAdapter;
use League\Flysystem\StorageAttributes;
use League\Flysystem\Config;
use League\Flysystem\FileAttributes;

/**
 * Delegating wrapper for Flysystem v3 FTP adapter.
 * 
 * This class wraps the official \League\Flysystem\Ftp\FtpAdapter,
 * providing a simplified interface for backward compatibility and convenience.
 */
class FilegatorFtp implements FilesystemAdapter
{
    private FilesystemAdapter $adapter;

    /**
     * Constructor accepts either an instance of a FilesystemAdapter (for testing or direct injection)
     * or an array of FTP adapter configuration.
     *
     * Configuration array keys:
     * - host (required): FTP hostname
     * - username (required): FTP username
     * - password (required): FTP password
     * - root (optional): FTP root directory path (default: '')
     * - port (optional): FTP port (default: 21)
     * - ssl (optional): Use SSL (default: false)
     * - timeout (optional): Connection timeout (default: 90)
     * - utf8 (optional): Enable UTF-8 (default: false)
     * - passive (optional): Passive mode (default: true)
     * - recurseManually (optional): Manual recursion (default: false, true in fromArray)
     * - systemType (optional): 'windows' or 'unix' (default: null for auto-detection)
     * - useRawListOptions (optional): Use raw LIST options (default: null)
     * - Others: see League\Flysystem\Ftp\FtpConnectionOptions
     *
     * @param array|FilesystemAdapter $config
     * @throws \Exception When league/flysystem-ftp is not installed
     * @throws \InvalidArgumentException When config type is invalid
     */
    public function __construct($config = [])
    {
        if ($config instanceof FilesystemAdapter) {
            $this->adapter = $config;
            return;
        }

        if (!is_array($config)) {
            throw new \InvalidArgumentException('FilegatorFtp expects an array config or a FilesystemAdapter instance.');
        }

        // Check that the FTP adapter package is installed
        $ftpAdapterClass = '\\League\\Flysystem\\Ftp\\FtpAdapter';
        $ftpConnectionOptionsClass = '\\League\\Flysystem\\Ftp\\FtpConnectionOptions';

        if (!class_exists($ftpAdapterClass) || !class_exists($ftpConnectionOptionsClass)) {
            throw new \Exception(
                'Please require "league/flysystem-ftp" to use FTP adapter: '
                . 'composer require league/flysystem-ftp:^3.0'
            );
        }

        // Convert config array to FtpConnectionOptions and instantiate the adapter
        $connectionOptions = $ftpConnectionOptionsClass::fromArray($config);
        $this->adapter = new $ftpAdapterClass($connectionOptions);
    }

    public function fileExists(string $path): bool
    {
        return $this->adapter->fileExists($path);
    }

    public function directoryExists(string $path): bool
    {
        return $this->adapter->directoryExists($path);
    }

    public function write(string $path, string $contents, Config $config): void
    {
        $this->adapter->write($path, $contents, $config);
    }

    /**
     * @param resource $contents
     */
    public function writeStream(string $path, $contents, Config $config): void
    {
        $this->adapter->writeStream($path, $contents, $config);
    }

    public function read(string $path): string
    {
        return $this->adapter->read($path);
    }

    /**
     * @return resource
     */
    public function readStream(string $path)
    {
        return $this->adapter->readStream($path);
    }

    public function delete(string $path): void
    {
        $this->adapter->delete($path);
    }

    public function deleteDirectory(string $path): void
    {
        $this->adapter->deleteDirectory($path);
    }

    public function createDirectory(string $path, Config $config): void
    {
        $this->adapter->createDirectory($path, $config);
    }

    public function setVisibility(string $path, string $visibility): void
    {
        $this->adapter->setVisibility($path, $visibility);
    }

    public function visibility(string $path): FileAttributes
    {
        return $this->adapter->visibility($path);
    }

    public function mimeType(string $path): FileAttributes
    {
        return $this->adapter->mimeType($path);
    }

    public function lastModified(string $path): FileAttributes
    {
        return $this->adapter->lastModified($path);
    }

    public function fileSize(string $path): FileAttributes
    {
        return $this->adapter->fileSize($path);
    }

    public function listContents(string $path, bool $deep): iterable
    {
        return $this->adapter->listContents($path, $deep);
    }

    public function move(string $source, string $destination, Config $config): void
    {
        $this->adapter->move($source, $destination, $config);
    }

    public function copy(string $source, string $destination, Config $config): void
    {
        $this->adapter->copy($source, $destination, $config);
    }

    /**
     * Return underlying connection/resource if available from the underlying adapter.
     */
    public function getConnection(): mixed
    {
        if (method_exists($this->adapter, 'getConnection')) {
            return $this->adapter->getConnection();
        }

        return null;
    }
}
