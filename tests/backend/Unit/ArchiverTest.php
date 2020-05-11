<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Services\Archiver\Adapters\ZipArchiver;
use Filegator\Services\Storage\Filesystem;
use Filegator\Services\Tmpfs\Adapters\Tmpfs;
use League\Flysystem\Memory\MemoryAdapter;
use League\Flysystem\Adapter\NullAdapter;
use Tests\TestCase;

/**
 * @internal
 */
class ArchiverTest extends TestCase
{
    protected $archiver;

    protected function setUp(): void
    {
        $tmpfs = new Tmpfs();
        $tmpfs->init([
            'path' => TEST_TMP_PATH,
            'gc_probability_perc' => 10,
            'gc_older_than' => 60 * 60 * 24 * 2, // 2 days
        ]);

        $this->archiver = new ZipArchiver($tmpfs);

        parent::setUp();
    }

    public function testCreatingEmptyArchive()
    {
        $storage = new Filesystem();
        $storage->init([
            'separator' => '/',
            'adapter' => function () {
                return new NullAdapter();
            },
        ]);

        $uniqid = $this->archiver->createArchive($storage);

        $this->assertNotNull($uniqid);
        $this->assertFileNotExists(TEST_TMP_PATH.$uniqid);
    }

    public function testAddingDirectoryWithFilesAndSubdir()
    {
        $storage = new Filesystem();
        $storage->init([
            'separator' => '/',
            'adapter' => function () {
                return new MemoryAdapter();
            },
        ]);

        $storage->createDir('/', 'test');
        $storage->createDir('/test', 'sub');
        $storage->createFile('/test', 'file1.txt');
        $storage->createFile('/test', 'file2.txt');

        $name = $this->archiver->createArchive($storage);
        $this->archiver->addDirectoryFromStorage('/test');
        $this->archiver->closeArchive();

        $this->assertGreaterThan(0, filesize(TEST_TMP_PATH.$name));
    }

    public function testUploadingArchiveToStorage()
    {
        $storage = new Filesystem();
        $storage->init([
            'separator' => '/',
            'adapter' => function () {
                return new MemoryAdapter();
            },
        ]);

        $storage->createDir('/', 'test');
        $storage->createDir('/test', 'sub');
        $storage->createFile('/test', 'file1.txt');
        $storage->createFile('/test', 'file2.txt');

        $name = $this->archiver->createArchive($storage);
        $this->archiver->addDirectoryFromStorage('/test');
        $this->archiver->storeArchive('/destination', 'myarchive.zip');

        $this->assertFileNotExists(TEST_TMP_PATH.$name);
    }

    public function testUncompressingArchiveFromStorage()
    {
        $storage = new Filesystem();
        $storage->init([
            'separator' => '/',
            'adapter' => function () {
                return new MemoryAdapter();
            },
        ]);

        $stream = fopen(TEST_ARCHIVE, 'r');
        $storage->store('/', 'testarchive.zip', $stream);
        fclose($stream);

        $storage->createDir('/', 'result');

        $this->archiver->uncompress('/testarchive.zip', '/result', $storage);

        $this->assertStringContainsString('testarchive', json_encode($storage->getDirectoryCollection('/')));
        $this->assertStringContainsString('onetwo', json_encode($storage->getDirectoryCollection('/result')));
    }
}
