<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Exception;
use Filegator\Services\Storage\Filesystem;
use League\Flysystem\Adapter\Local;
use Tests\TestCase;

/**
 * @internal
 */
class FilesystemTest extends TestCase
{
    protected $storage;

    protected $timestamp;

    protected $separator = '/';

    protected function setUp(): void
    {
        $this->resetTempDir();

        $this->timestamp = time();

        $this->storage = new Filesystem();
        $this->storage->init([
            'separator' => '/',
            'adapter' => function () {
                return new Local(
                    TEST_REPOSITORY
                );
            },
        ]);
    }

    protected function tearDown(): void
    {
        $this->resetTempDir();
    }

    public function testGetDirectoryFileCount()
    {
        $this->storage->createFile('/', '1.txt');
        $this->storage->createFile('/', '2.txt');
        $this->storage->createFile('/', '3.txt');
        $this->storage->createFile('/', '4.txt');
        $this->storage->createFile('/', '5.txt');
        $this->storage->createDir('/', 'tmp');

        $ret = $this->storage->getDirectoryCollection('/');

        $ret_array = json_decode(json_encode($ret), true);

        $this->assertCount(6, $ret_array['files']);
    }

    public function testGetSubDirectyoryFileCount()
    {
        $this->storage->createDir('/', 'sub');
        $this->storage->createFile('/sub', '1.txt');
        $this->storage->createFile('/sub', '2.txt');
        $this->storage->createFile('/sub', '3.txt');
        $this->storage->createFile('/sub', '4.txt');
        $this->storage->createDir('/sub', 'deep');
        $this->storage->createFile('/sub/deep', '1.txt');

        $ret = $this->storage->getDirectoryCollection('/sub');

        $ret_array = json_decode(json_encode($ret), true);

        // back + 4 files + 1 deep dir
        $this->assertCount(6, $ret_array['files']);

        $ret = $this->storage->getDirectoryCollection('/sub/deep');

        $ret_array = json_decode(json_encode($ret), true);

        // back + 1 file
        $this->assertCount(2, $ret_array['files']);
    }

    public function testInvalidDirReturnsBackLinkOnly()
    {
        $ret = $this->storage->getDirectoryCollection('/etc');

        $this->assertJsonStringEqualsJsonString(json_encode($ret), json_encode([
            'location' => '/etc',
            'files' => [
                0 => [
                    'type' => 'back',
                    'path' => '/',
                    'name' => '..',
                    'size' => 0,
                    'time' => 0,
                    'permissions' => -1
                ],
            ],
        ]));
    }

    public function testListSubDirContents()
    {
        $this->storage->createDir('/', 'john');
        $this->storage->createDir('/john', 'johnsub');
        $this->storage->createFile('/john/johnsub', 'john2.txt');

        $ret = $this->storage->getDirectoryCollection('/john/johnsub');
        $ret->resetTimestamps();

        $this->assertJsonStringEqualsJsonString(json_encode([
            'location' => '/john/johnsub',
            'files' => [
                0 => [
                    'type' => 'back',
                    'path' => '/john',
                    'name' => '..',
                    'permissions' => -1,
                    'size' => 0,
                    'time' => 0,
                ],
                1 => [
                    'type' => 'file',
                    'path' => '/john/johnsub/john2.txt',
                    'name' => 'john2.txt',
                    'permissions' => 644,
                    'size' => 0,
                    'time' => 0,
                ],
            ],
        ]), json_encode($ret));
    }

    public function testHomeDirContentsUsingPathPrefix()
    {
        $this->storage->setPathPrefix('/john');
        $this->storage->createDir('/', 'johnsub');
        $this->storage->createFile('/', 'john.txt');

        $ret = $this->storage->getDirectoryCollection('/');
        $ret->resetTimestamps(-1);

        $this->assertJsonStringEqualsJsonString(json_encode([
            'location' => '/',
            'files' => [
                0 => [
                    'type' => 'dir',
                    'path' => '/johnsub',
                    'name' => 'johnsub',
                    'permissions' => 755,
                    'size' => 0,
                    'time' => -1,
                ],
                1 => [
                    'type' => 'file',
                    'path' => '/john.txt',
                    'name' => 'john.txt',
                    'permissions' => 644,
                    'size' => 0,
                    'time' => -1,
                ],
            ],
        ]), json_encode($ret));
    }

    public function testSubDirContentsUsingPathPrefix()
    {
        $this->storage->setPathPrefix('/john');
        $this->storage->createDir('/', 'johnsub');
        $this->storage->createFile('/johnsub', 'john2.txt');

        $ret = $this->storage->getDirectoryCollection('/johnsub');

        $ret->resetTimestamps();

        $this->assertJsonStringEqualsJsonString(json_encode([
            'location' => '/johnsub',
            'files' => [
                0 => [
                    'type' => 'back',
                    'path' => '/',
                    'name' => '..',
                    'permissions' => -1,
                    'size' => 0,
                    'time' => 0,
                ],
                1 => [
                    'type' => 'file',
                    'path' => '/johnsub/john2.txt',
                    'name' => 'john2.txt',
                    'permissions' => 644,
                    'size' => 0,
                    'time' => 0,
                ],
            ],
        ]), json_encode($ret));
    }

    public function testStoringFileToRoot()
    {
        // create dummy file
        file_put_contents(TEST_FILE, 'lorem ipsum');

        $resource = fopen(TEST_FILE, 'r');
        $ret = $this->storage->store('/', 'loremfile.txt', $resource);
        fclose($resource);

        $this->assertTrue($ret);

        $this->assertFileExists(TEST_REPOSITORY.'/loremfile.txt');
    }

    public function testStoringFileToRootSubFolder()
    {
        // create dummy file
        file_put_contents(TEST_FILE, 'lorem ipsum');

        $resource = fopen(TEST_FILE, 'r');
        $ret = $this->storage->store('/sub/sub1', 'loremfile.txt', $resource);
        fclose($resource);

        $this->assertTrue($ret);

        $this->assertFileExists(TEST_REPOSITORY.'/sub/sub1/loremfile.txt');
        $this->assertFileNotExists(TEST_REPOSITORY.'/loremfile.txt');
    }

    public function testUpcountingFilenameOrDirname()
    {
        $this->assertEquals('test (1).txt', $this->invokeMethod($this->storage, 'upcountName', ['test.txt']));
        $this->assertEquals('test (2).txt', $this->invokeMethod($this->storage, 'upcountName', ['test (1).txt']));
        $this->assertEquals('test (100).txt', $this->invokeMethod($this->storage, 'upcountName', ['test (99).txt']));
        $this->assertEquals('test (1)', $this->invokeMethod($this->storage, 'upcountName', ['test']));
        $this->assertEquals('test (9) (2) (1)', $this->invokeMethod($this->storage, 'upcountName', ['test (9) (2)']));
        $this->assertEquals('test (2) (3) (4).txt', $this->invokeMethod($this->storage, 'upcountName', ['test (2) (3) (3).txt']));
        $this->assertEquals('1 (1)', $this->invokeMethod($this->storage, 'upcountName', ['1']));
        $this->assertEquals('test (1).txt (1).zip', $this->invokeMethod($this->storage, 'upcountName', ['test (1).txt.zip']));
        $this->assertEquals('test(1) (1)', $this->invokeMethod($this->storage, 'upcountName', ['test(1)']));
    }

    public function testStoringFileWithTheSameNameUpcountsSecondFilename()
    {
        // create dummy file
        file_put_contents(TEST_FILE, 'lorem ipsum');

        $resource = fopen(TEST_FILE, 'r');
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // create another dummy file witht the same name but different content
        file_put_contents(TEST_FILE, 'croissant');

        $resource = fopen(TEST_FILE, 'r');
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // first file is not overwritten
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('lorem ipsum', stream_get_contents($ret['stream']));

        // second file is also here but with upcounted name
        $ret = $this->storage->readStream('singletone (1).txt');
        $this->assertEquals('croissant', stream_get_contents($ret['stream']));
    }

    public function testStoringFileWithTheSameNameOverwritesOriginalFile()
    {
        // create dummy file
        $string = 'lorem ipsum';
        $resource = fopen('data://text/plain;base64,'.base64_encode($string), 'r');

        // and store it
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // first file contains lorem ipsum
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('lorem ipsum', stream_get_contents($ret['stream']));

        // create another dummy file
        $string = 'croissant';
        $resource = fopen('data://text/plain;base64,'.base64_encode($string), 'r');

        // and store it with the same name
        $this->storage->store('/', 'singletone.txt', $resource, true);
        fclose($resource);

        // first file is overwritten
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('croissant', stream_get_contents($ret['stream']));
    }

    public function testStoringFileWithTheSameNameUpcountsSecondFilenameUsingPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');

        // create dummy file
        file_put_contents(TEST_FILE, 'lorem ipsum');

        $resource = fopen(TEST_FILE, 'r');
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // create another dummy file witht the same name but different content
        file_put_contents(TEST_FILE, 'croissant');

        $resource = fopen(TEST_FILE, 'r');
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // first file is not overwritten
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('lorem ipsum', stream_get_contents($ret['stream']));

        // second file is also here but with upcounted name
        $ret = $this->storage->readStream('singletone (1).txt');
        $this->assertEquals('croissant', stream_get_contents($ret['stream']));
    }

    public function testStoringFileWithTheSameNameOverwritesOriginalFileUsingPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');

        // create dummy file
        $string = 'lorem ipsum';
        $resource = fopen('data://text/plain;base64,'.base64_encode($string), 'r');

        // and store it
        $this->storage->store('/', 'singletone.txt', $resource);
        fclose($resource);

        // first file contains lorem ipsum
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('lorem ipsum', stream_get_contents($ret['stream']));

        // create another dummy file
        $string = 'croissant';
        $resource = fopen('data://text/plain;base64,'.base64_encode($string), 'r');

        // and store it with the same name
        $this->storage->store('/', 'singletone.txt', $resource, true);
        fclose($resource);

        // first file is overwritten
        $ret = $this->storage->readStream('singletone.txt');
        $this->assertEquals('croissant', stream_get_contents($ret['stream']));
    }
    public function testCreatingFileWithTheSameNameUpcountsFilenameRecursively()
    {
        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/', 'test (1).txt');

        $resource = fopen(TEST_FILE, 'r');
        $this->storage->store('/', 'test.txt', $resource);
        fclose($resource);

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
        $this->assertTrue($this->storage->fileExists('/test (2).txt')); // created with (2)
    }

    public function testCreatingDirectoryWithTheSameNameAsNonEmptyDirUpcountsDestinationDir()
    {
        $this->storage->createDir('/', 'test');
        $this->storage->createFile('/test', 'a.txt');
        // this dir
        $this->storage->createDir('/', 'test');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/test');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/test (1)'); // goes here
    }

    public function testCreatingDirectoryWithTheSameNameAsNonEmptyDirUpcountsDestinationDirRecursively()
    {
        $this->storage->createDir('/', 'test');
        $this->storage->createFile('/test', 'a.txt');
        $this->storage->createDir('/', 'test (1)');
        $this->storage->createFile('/test (1)', 'b.txt');

        // this dir
        $this->storage->createDir('/', 'test');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/test');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/test (1)');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/test (1) (1)'); // goes here
    }

    public function testMovingFileWithTheSameNameUpcountsSecondFilename()
    {
        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/sub', 'test.txt');

        // move second file over the first one
        $this->storage->move('/sub/test.txt', '/test.txt');

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
    }

    public function testMovingFileWithTheSameNameUpcountsSecondFilenameUntilTheNameIsUnique()
    {
        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/', 'test (1).txt');
        $this->storage->createFile('/', 'test (2).txt');
        $this->storage->createFile('/sub', 'test.txt');

        // move second file over the first one
        $this->storage->move('/sub/test.txt', '/test.txt');

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
        $this->assertTrue($this->storage->fileExists('/test (2).txt'));
        $this->assertTrue($this->storage->fileExists('/test (3).txt')); // file is moved here
    }

    public function testCopyingFileWithTheSameNameUpcountsSecondFilename()
    {
        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/', 'test (1).txt');
        $this->storage->createFile('/', 'test (2).txt');
        $this->storage->createFile('/sub', 'test.txt');

        // move second file over the first one
        $this->storage->copyFile('/sub/test.txt', '/');

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
        $this->assertTrue($this->storage->fileExists('/test (2).txt'));
        $this->assertTrue($this->storage->fileExists('/test (3).txt')); // file is copied here
    }

    public function testGetPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');
        $this->assertEquals('/john/', $this->storage->getPathPrefix());

        $this->storage->setPathPrefix('/john');
        $this->assertEquals('/john/', $this->storage->getPathPrefix());

        $this->storage->setPathPrefix('john/');
        $this->assertEquals('/john/', $this->storage->getPathPrefix());

        $this->storage->setPathPrefix('john');
        $this->assertEquals('/john/', $this->storage->getPathPrefix());
    }

    public function testApplyPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');

        $this->assertEquals('/john/test', $this->invokeMethod($this->storage, 'applyPathPrefix', ['test']));
        $this->assertEquals('/john/test/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['test/']));
        $this->assertEquals('/john/test/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['/test/']));
        $this->assertEquals('/john/test', $this->invokeMethod($this->storage, 'applyPathPrefix', ['/test']));
        $this->assertEquals('/john/test.txt', $this->invokeMethod($this->storage, 'applyPathPrefix', ['test.txt']));
        $this->assertEquals('/john/test.txt/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['test.txt/']));
        // no escaping path to upper dir
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['/..']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['../']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['/sub/../../']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\\\']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\..\\']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['\\\\..']));
        $this->assertEquals('/john/', $this->invokeMethod($this->storage, 'applyPathPrefix', ['\\..\\..']));
        $this->assertEquals('/john/\\.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['\\.\\...']));
        $this->assertEquals('/john/\\.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['\\.\\....']));
        $this->assertEquals('/john/.\\.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['.\\.\\...']));
        $this->assertEquals('/john/.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\.\\...']));
        $this->assertEquals('/john/.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\.\\...']));
        $this->assertEquals('/john/.', $this->invokeMethod($this->storage, 'applyPathPrefix', ['..\\.\\......']));
        $this->assertEquals('/john/.\\', $this->invokeMethod($this->storage, 'applyPathPrefix', ['...\\.\\......\\']));
    }

    public function testStripPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');

        $this->assertEquals('/', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/john/']));
        $this->assertEquals('/test/', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/john/test/']));
        $this->assertEquals('/test', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/john/test']));
        $this->assertEquals('/doe/test', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/john/doe/test']));
        $this->assertEquals('/doe/test.txt', $this->invokeMethod($this->storage, 'stripPathPrefix', ['john/doe/test.txt']));
        $this->assertEquals('/john', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/john']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'stripPathPrefix', ['/']));
    }

    public function testAddSeparators()
    {
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'addSeparators', ['']));
        $this->assertEquals('/ /', $this->invokeMethod($this->storage, 'addSeparators', [' ']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'addSeparators', ['/']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'addSeparators', ['//']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'addSeparators', ['////']));
        $this->assertEquals('/b/', $this->invokeMethod($this->storage, 'addSeparators', ['b']));
        $this->assertEquals('/b/', $this->invokeMethod($this->storage, 'addSeparators', ['/b']));
        $this->assertEquals('/b/', $this->invokeMethod($this->storage, 'addSeparators', ['/b/']));
        $this->assertEquals('/b/', $this->invokeMethod($this->storage, 'addSeparators', ['/b//']));
        $this->assertEquals('/b/', $this->invokeMethod($this->storage, 'addSeparators', ['//b//']));
        $this->assertEquals('/a/b/', $this->invokeMethod($this->storage, 'addSeparators', ['a/b']));
        $this->assertEquals('/a/b/', $this->invokeMethod($this->storage, 'addSeparators', ['a/b/']));
        $this->assertEquals('/a/b/', $this->invokeMethod($this->storage, 'addSeparators', ['/a/b/']));
        $this->assertEquals('/a b/', $this->invokeMethod($this->storage, 'addSeparators', ['a b']));
        $this->assertEquals('/a b/c/', $this->invokeMethod($this->storage, 'addSeparators', ['a b/c']));
    }

    public function testJoinPaths()
    {
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'joinPaths', ['1', '2']));
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'joinPaths', ['/1', '/2']));
        $this->assertEquals('/1/2/', $this->invokeMethod($this->storage, 'joinPaths', ['1/', '2/']));
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'joinPaths', ['1/', '/2']));
        $this->assertEquals('/1/2/', $this->invokeMethod($this->storage, 'joinPaths', ['/1', '2/']));
        $this->assertEquals('/1/2/', $this->invokeMethod($this->storage, 'joinPaths', ['/1/', '/2/']));
    }

    public function testGetBaseName()
    {
        $this->assertEquals('test.txt', $this->invokeMethod($this->storage, 'getBaseName', ['test.txt']));
        $this->assertEquals('test.txt', $this->invokeMethod($this->storage, 'getBaseName', ['/test.txt']));
        $this->assertEquals('test.txt', $this->invokeMethod($this->storage, 'getBaseName', ['/mike/test.txt']));
        $this->assertEquals('b', $this->invokeMethod($this->storage, 'getBaseName', ['/a/b']));
        $this->assertEquals('b', $this->invokeMethod($this->storage, 'getBaseName', ['/a/b/']));
        $this->assertEquals('b', $this->invokeMethod($this->storage, 'getBaseName', ['a/b']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getBaseName', ['']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getBaseName', ['/']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getBaseName', ['/////']));
    }

    public function testGetParent()
    {
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getParent', ['']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getParent', [' ']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getParent', ['/']));
        $this->assertEquals('/', $this->invokeMethod($this->storage, 'getParent', ['////']));
        $this->assertEquals('/parent', $this->invokeMethod($this->storage, 'getParent', ['/parent/child/']));
        $this->assertEquals('/1/2/3/4', $this->invokeMethod($this->storage, 'getParent', ['/1/2/3/4/5/']));
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'getParent', ['1/2/3']));
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'getParent', ['1/2/3/']));
        $this->assertEquals('/1/2', $this->invokeMethod($this->storage, 'getParent', ['/1/2/3/']));
    }

    public function testDeleteFiles()
    {
        $this->storage->createFile('/', 'sample22.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/sample22.txt');

        $this->storage->deleteFile('sample22.txt');

        $this->assertFileNotExists(TEST_REPOSITORY.'/sample22.txt');
    }

    public function testCreateAndDeleteDirectory()
    {
        $this->storage->createDir('/', 'sample22');
        $this->storage->createDir('/sample22/subsample', 'sample22');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/sample22');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/sample22/subsample');

        $this->storage->deleteDir('sample22');

        $this->assertDirectoryNotExists(TEST_REPOSITORY.'/sample22');
        $this->assertDirectoryNotExists(TEST_REPOSITORY.'/sample22/subsample');
    }

    public function testReadFileStream()
    {
        $this->storage->createFile('/', 'a.txt');
        $ret = $this->storage->readStream('a.txt');

        $this->assertEquals($ret['filename'], 'a.txt');
        $this->assertIsResource($ret['stream']);
    }

    public function testReadFileStreamMissingFileThrowsException()
    {
        $this->expectException(Exception::class);

        $this->storage->readStream('missing');
    }

    public function testCannotStreamDirectory()
    {
        $this->storage->createDir('/', 'sub');

        $this->expectException(Exception::class);

        $this->storage->readStream('sub');
    }

    public function testDirCheck()
    {
        $this->storage->createDir('/', 'sub');
        $this->storage->createDir('/', 'empty');
        $this->storage->createFile('/sub', 'd.txt');
        $this->storage->createDir('/sub', 'sub1');
        $this->storage->createFile('/sub/sub1', 'f.txt');
        $this->storage->createDir('/sub', 'empty');
        $this->storage->createDir('/', 'john');
        $this->storage->createFile('/', 'a.txt');

        $this->assertTrue($this->storage->isDir('/sub'));
        $this->assertTrue($this->storage->isDir('/sub/sub1'));
        $this->assertTrue($this->storage->isDir('/john'));
        $this->assertTrue($this->storage->isDir('/empty'));
        $this->assertTrue($this->storage->isDir('/sub/empty'));
        $this->assertFalse($this->storage->isDir('a.txt'));
        $this->assertFalse($this->storage->isDir('/sub/d.txt'));
        $this->assertFalse($this->storage->isDir('/sub/sub1/f.txt'));
    }

    public function testRenameFile()
    {
        $this->storage->createFile('/', 'a.txt');

        $this->storage->rename('/', 'a.txt', 'a1.txt');

        $this->assertFalse($this->storage->fileExists('/a.txt'));
        $this->assertTrue($this->storage->fileExists('/a1.txt'));
    }

    public function testRenameFileToExistingDestinationUpcountsFilenameRecursively()
    {
        $this->storage->createFile('/', 'a.txt');
        $this->storage->createFile('/', 'a (1).txt');
        $this->storage->createFile('/', 'test.txt');

        $this->storage->rename('/', 'test.txt', 'a.txt');

        $this->assertTrue($this->storage->fileExists('/a.txt'));
        $this->assertTrue($this->storage->fileExists('/a (1).txt'));
        $this->assertTrue($this->storage->fileExists('/a (2).txt')); // result
    }

    public function testRenameFileInSubfolder()
    {
        $this->storage->createDir('/', 'john');
        $this->storage->createFile('/john', 'john.txt');

        $this->storage->rename('/john/', 'john.txt', 'john2.txt');

        $this->assertFalse($this->storage->fileExists('/john/john.txt'));
        $this->assertTrue($this->storage->fileExists('/john/john2.txt'));
    }

    public function testRenameFileWithPathPrefix()
    {
        $this->storage->setPathPrefix('/john/');
        $this->storage->createFile('/', 'john.txt');
        $this->storage->rename('/', 'john.txt', 'john2.txt');

        $this->assertFalse($this->storage->fileExists('/john.txt'));
        $this->assertTrue($this->storage->fileExists('/john2.txt'));
    }

    public function testRenameNonexistingFileThrowsException()
    {
        $this->expectException(Exception::class);

        $this->storage->move('/', 'nonexisting.txt', 'a1.txt');
    }

    public function testCreatingFile()
    {
        $this->storage->createFile('/', 'sample22');
        $ret = $this->storage->getDirectoryCollection('/');
        $this->assertStringContainsString('sample22', json_encode($ret));

        $this->storage->createFile('/sub/', 'sample33');
        $ret = $this->storage->getDirectoryCollection('/sub/');
        $this->assertStringContainsString('sample33', json_encode($ret));
    }

    public function testCreatingFileUpcountsNameIfAlreadyExists()
    {
        $this->assertFalse($this->storage->fileExists('/test.txt'));
        $this->assertFalse($this->storage->fileExists('/test (1).txt'));

        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/', 'test.txt');

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
    }

    public function testCreatingFileUpcountsNameRecursivelyIfAlreadyExists()
    {
        $this->storage->createFile('/', 'test.txt');
        $this->storage->createFile('/', 'test (1).txt');

        // this file
        $this->storage->createFile('/', 'test.txt');

        $this->assertTrue($this->storage->fileExists('/test.txt'));
        $this->assertTrue($this->storage->fileExists('/test (1).txt'));
        $this->assertTrue($this->storage->fileExists('/test (2).txt')); // ends up here
    }

    public function testGetSeparator()
    {
        $separator = $this->storage->getSeparator();

        $this->assertEquals($this->separator, $separator);
    }

    public function testCopyFile()
    {
        $this->storage->setPathPrefix('/john');
        $this->storage->createFile('/', 'john.txt');
        $this->storage->createDir('/', 'johnsub');
        $this->storage->createFile('/johnsub', 'sub.txt');

        $this->assertFalse($this->storage->fileExists('/johnsub/john.txt'));

        $this->storage->copyFile('/john.txt', '/johnsub/');

        $this->assertTrue($this->storage->fileExists('/johnsub/john.txt'));

        $this->assertFalse($this->storage->fileExists('/sub.txt'));

        $this->storage->copyFile('/johnsub/sub.txt', '/');

        $this->assertTrue($this->storage->fileExists('/sub.txt'));
    }

    public function testCopyMissingFileThrowsException()
    {
        $this->storage->createDir('/', 'tmp');

        $this->expectException(Exception::class);
        $this->storage->copyFile('/missing.txt', '/tmp/');
    }

    public function testCopyMissingDirCreatedADirOnDestination()
    {
        $this->storage->createDir('/', 'tmp');

        $this->storage->copyDir('/missing/', '/tmp/');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/tmp/missing/');
    }

    public function testCopyDir()
    {
        $this->storage->createDir('/', '/john');
        $this->storage->createDir('/john', '/johnsub');
        $this->storage->createDir('/', '/jane');

        $this->storage->copyDir('/john/johnsub', '/jane/');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/johnsub');
    }

    public function testCopyDirWithSubDirs()
    {
        $this->storage->createDir('/', '/sub');
        $this->storage->createDir('/sub', '/sub1');
        $this->storage->createDir('/', '/jane');

        $this->storage->copyDir('/sub', '/jane/');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/sub');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/sub/sub1');
    }

    public function testCopyDirWithEmptySubDir()
    {
        $this->storage->createDir('/', 'tmp');
        $this->storage->createDir('/tmp/', 'sample22');
        $this->storage->createDir('/tmp/sample22/', 'subsample1');
        $this->storage->createDir('/tmp/sample22/', 'subsample2');
        $this->storage->createFile('/tmp/sample22/subsample2', 'zzzz');

        $this->assertDirectoryNotExists(TEST_REPOSITORY.'/jane/sample22');

        $this->storage->copyDir('/tmp/sample22', '/jane/');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/sample22');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/sample22/subsample2');
        $this->assertTrue($this->storage->fileExists('/jane/sample22/subsample2/zzzz'));
    }

    public function testCopyEmptyDir()
    {
        $this->storage->createDir('/', 'dest');
        $this->storage->createDir('/', 'tmp');

        $this->storage->copyDir('/tmp', '/dest');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/dest/tmp');
    }

    public function testCopyDirOverExistingUpcountsDestinationDirname()
    {
        /*
         * /dest/tmp/
         * /dest/tmp/a.txt
         * /tmp/
         * /tmp/b.txt
         *
         * copy /tmp/ => /dest/
         *
         * /dest/tmp/
         * /dest/tmp/a.txt
         * /dest/tmp (1)/
         * /dest/tmp (1)/b.txt
         *
         */
        $this->storage->createDir('/', 'dest');
        $this->storage->createDir('/dest', 'tmp');
        $this->storage->createFile('/dest/tmp/', 'a.txt');
        $this->storage->createDir('/', 'tmp');
        $this->storage->createFile('/tmp/', 'b.txt');

        $this->storage->copyDir('/tmp', '/dest');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/dest/tmp/');
        $this->assertTrue($this->storage->fileExists('/dest/tmp/a.txt'));

        $this->assertDirectoryExists(TEST_REPOSITORY.'/dest/tmp (1)');
        $this->assertTrue($this->storage->fileExists('/dest/tmp (1)/b.txt'));
    }

    public function testMoveFile()
    {
        $this->storage->createFile('/', 'file.txt');
        $this->storage->createDir('/', 'tmp');
        $this->storage->move('/file.txt', '/tmp/file.txt');

        $this->assertFalse($this->storage->fileExists('/file.txt'));
        $this->assertTrue($this->storage->fileExists('/tmp/file.txt'));
    }

    public function testMoveDirectory()
    {
        $this->storage->createDir('/', 'test1');
        $this->storage->createDir('/', 'test2');
        $this->storage->move('/test1', '/test2/test1/');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/test2/test1/');
    }

    public function testCannotGoUpTheHomeDirUsingPathFiddle()
    {
        $this->storage->createFile('/', 'hidden.txt');
        $this->storage->createDir('/', 'johnsub');
        $this->storage->createFile('/johnsub', 'john.txt');
        $this->storage->setPathPrefix('/johnsub');

        $ret = $this->storage->getDirectoryCollection('/');
        $ret->resetTimestamps(-1);
        $this->assertJsonStringEqualsJsonString(json_encode([
            'location' => '/',
            'files' => [
                0 => [
                    'type' => 'file',
                    'path' => '/john.txt',
                    'name' => 'john.txt',
                    'permissions' => 644,
                    'size' => 0,
                    'time' => -1,
                ],
            ],
        ]), json_encode($ret));

        $ret = $this->storage->getDirectoryCollection('/..');
        $ret->resetTimestamps(-1);
        $this->assertJsonStringEqualsJsonString(json_encode([
            'location' => '/..',
            'files' => [
                0 => [
                    'type' => 'back',
                    'path' => '/',
                    'name' => '..',
                    'permissions' => -1,
                    'size' => 0,
                    'time' => -1,
                ],
                1 => [
                    'type' => 'file',
                    'path' => '/john.txt',
                    'name' => 'john.txt',
                    'permissions' => 644,
                    'size' => 0,
                    'time' => -1,
                ],
            ],
        ]), json_encode($ret));
    }
}
