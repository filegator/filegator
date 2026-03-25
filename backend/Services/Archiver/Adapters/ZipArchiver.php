<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Archiver\Adapters;

use Filegator\Services\Archiver\ArchiverInterface;
use Filegator\Services\Service;
use Filegator\Services\Storage\Filesystem as Storage;
use Filegator\Services\Tmpfs\TmpfsInterface;
use League\Flysystem\Filesystem as Flysystem;
use League\Flysystem\ZipArchive\FilesystemZipArchiveProvider;
use League\Flysystem\ZipArchive\ZipArchiveAdapter;

class ZipArchiver implements Service, ArchiverInterface
{
    protected $archive;

    protected $archiveAdapter;

    protected $storage;

    protected $tmpfs;

    protected $uniqid;

    protected $tmp_files = [];

    public function __construct(TmpfsInterface $tmpfs)
    {
        $this->tmpfs = $tmpfs;
    }

    public function init(array $config = [])
    {
    }

    public function createArchive(Storage $storage): string
    {
        $this->uniqid = uniqid();

        $provider = new FilesystemZipArchiveProvider($this->tmpfs->getFileLocation($this->uniqid));
        $this->archiveAdapter = new ZipArchiveAdapter($provider);
        $this->archive = new Flysystem($this->archiveAdapter);

        $this->storage = $storage;

        return $this->uniqid;
    }

    public function addDirectoryFromStorage(string $path)
    {
        $content = $this->storage->getDirectoryCollection($path, true);
        $this->archive->createDirectory($path);

        foreach ($content->all() as $item) {
            if ($item['type'] == 'dir') {
                $this->archive->createDirectory($item['path']);
            }
            if ($item['type'] == 'file') {
                $this->addFileFromStorage($item['path']);
            }
        }
    }

    public function addFileFromStorage(string $path)
    {
        $file = $this->storage->readStream($path);

        $this->archive->writeStream($path, $file['stream']);

        if (is_resource($file['stream'])) {
            fclose($file['stream']);
        }
    }

    public function uncompress(string $source, string $destination, Storage $storage)
    {
        $name = uniqid().'.zip';

        $remote_archive = $storage->readStream($source);
        $this->tmpfs->write($name, $remote_archive['stream']);

        $provider = new FilesystemZipArchiveProvider($this->tmpfs->getFileLocation($name));
        $archive = new Flysystem(new ZipArchiveAdapter($provider));

        $contents = iterator_to_array($archive->listContents('/', true));

        foreach ($contents as $item) {
            $stream = null;
            if ($item->isDir()) {
                $storage->createDir($destination, $item->path());
            }
            if ($item->isFile()) {
                $stream = $archive->readStream($item->path());
                $dirname = dirname($item->path());
                if ($dirname === '.') {
                    $dirname = '';
                }
                $basename = basename($item->path());
                $storage->store($destination.'/'.$dirname, $basename, $stream);
            }
            if (is_resource($stream)) {
                fclose($stream);
            }
        }

        $this->tmpfs->remove($name);
    }

    public function closeArchive()
    {
        // In Flysystem v2 ZipArchiveAdapter, each operation opens and closes the zip file internally.
        // No explicit archive handle is required here.

        foreach ($this->tmp_files as $file) {
            $this->tmpfs->remove($file);
        }
    }

    public function storeArchive($destination, $name)
    {
        $this->closeArchive();

        $file = $this->tmpfs->readStream($this->uniqid);
        $this->storage->store($destination, $name, $file['stream']);
        if (is_resource($file['stream'])) {
            fclose($file['stream']);
        }

        $this->tmpfs->remove($this->uniqid);
    }
}
