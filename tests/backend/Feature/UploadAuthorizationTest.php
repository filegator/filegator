<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Symfony\Component\HttpFoundation\File\UploadedFile;
use Tests\TestCase;

/**
 * Regression tests for GHSA-44r8-3p76-84mw.
 *
 * POST /upload used the request-supplied resumableFilename directly as a key
 * into the shared temporary directory, so a user could read, store and delete
 * any temporary file by name. These tests lock in the fix: the upload assembly
 * only ever touches the request's own namespaced temporary files.
 *
 * @internal
 */
class UploadAuthorizationTest extends TestCase
{
    protected function setUp(): void
    {
        $this->resetTempDir();

        parent::setUp();
    }

    protected function tearDown(): void
    {
        $this->resetTempDir();
    }

    public function testUploadCannotReachAnotherUsersTemporaryFile()
    {
        // A transient file that does not belong to the attacker, sitting in the
        // shared temporary directory (e.g. another user's pending batch archive).
        file_put_contents(TEST_TMP_PATH.'victimsecret', 'VICTIM SECRET');

        // Attacker: a regular user holding the upload permission, with a home dir
        // of their own.
        $this->signIn('john@example.com', 'john123');
        mkdir(TEST_REPOSITORY.'/john');

        $fp = fopen(TEST_FILE, 'w');
        fwrite($fp, 'x');
        fclose($fp);
        $files = ['file' => new UploadedFile(TEST_FILE, 'dummy.txt', 'text/plain', null, true)];

        // The empty-assembly trick: resumableTotalChunks/Size = 0 skips the
        // concatenation loop, so the pre-fix code assembled straight from the
        // existing tmpfs file named by resumableFilename.
        $this->sendRequest('POST', '/upload', [
            'resumableChunkNumber' => 1,
            'resumableTotalChunks' => 0,
            'resumableTotalSize' => 0,
            'resumableIdentifier' => 'ATTACK',
            'resumableFilename' => 'victimsecret',
            'resumableRelativePath' => '/',
        ], $files);

        // The victim's file must not have leaked into the attacker's home dir ...
        $this->assertFileNotExists(TEST_REPOSITORY.'/john/victimsecret');
        // ... and it must still be intact in the temporary directory.
        $this->assertFileExists(TEST_TMP_PATH.'victimsecret');
        $this->assertEquals('VICTIM SECRET', file_get_contents(TEST_TMP_PATH.'victimsecret'));
    }

    public function testNormalUploadStillWorks()
    {
        $this->signIn('john@example.com', 'john123');
        mkdir(TEST_REPOSITORY.'/john');

        $fp = fopen(TEST_FILE, 'w');
        fwrite($fp, 'hello world');
        fclose($fp);
        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $this->sendRequest('POST', '/upload', [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableTotalChunks' => 1,
            'resumableTotalSize' => 11,
            'resumableIdentifier' => 'NORMAL',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ], $files);

        $this->assertOk();
        $this->assertFileExists(TEST_REPOSITORY.'/john/sample.txt');
        $this->assertEquals('hello world', file_get_contents(TEST_REPOSITORY.'/john/sample.txt'));
    }
}
