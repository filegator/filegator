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

/**
 * Custom FTP adapter for Filegator
 * 
 * Note: The FTP adapter was removed from Flysystem 3.x core library
 * For FTP support in 3.x, you would need a separate FTP adapter package
 * or implement this class to work with a custom FTP implementation.
 * 
 * This is a placeholder for now - functionality requires a separate FTP adapter.
 */
class FilegatorFtp implements FilesystemAdapter
{
    // This adapter requires a separate FTP implementation
    // The official Flysystem only provides SFTP in 3.x

    public function fileExists(string $path): bool
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function directoryExists(string $path): bool
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function write(string $path, string $contents, array $options = []): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function writeStream(string $path, mixed $contents, array $options = []): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function read(string $path): string
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function readStream(string $path): mixed
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function delete(string $path): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function deleteDirectory(string $path): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function createDirectory(string $path, array $options = []): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function setVisibility(string $path, string $visibility): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function visibility(string $path): array
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function mimeType(string $path): array
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function lastModified(string $path): array
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function fileSize(string $path): array
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function listContents(string $path, bool $deep = false): iterable
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function move(string $source, string $destination, array $options = []): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function copy(string $source, string $destination, array $options = []): void
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }

    public function getConnection(): mixed
    {
        throw new \Exception('FTP adapter is not available in Flysystem 3.x. Please use SFTP or provide a custom FTP adapter implementation.');
    }
}
