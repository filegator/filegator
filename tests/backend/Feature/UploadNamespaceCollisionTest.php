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
 * Hardening regression test for the upload tmpfs namespace (follow-up to
 * GHSA-44r8-3p76-84mw).
 *
 * The per-user upload namespace was derived by stripping every character
 * outside [0-9a-zA-Z_] from the username. Two distinct users whose usernames
 * differ only by stripped characters (here bobsmith@example.com and
 * bob.smith@example.com, both collapsing to "bobsmithexamplecom") therefore
 * shared one namespace, re-opening the cross-user temporary-file access the
 * advisory closed. Hashing the raw username keeps the namespaces distinct.
 *
 * @internal
 */
class UploadNamespaceCollisionTest extends TestCase
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

    public function testCollidingUsersDoNotShareTheUploadNamespace()
    {
        // A file the other Bob left in the namespace that a character-stripping
        // sanitiser would assign to both accounts ("bobsmithexamplecom").
        $shared = TEST_TMP_PATH.'multipart_bobsmithexamplecom_SHARED_secret.txt';
        file_put_contents($shared, 'VICTIM SECRET');

        // Attacker: the colliding sibling account, with its own home dir.
        $this->signIn('bob.smith@example.com', 'bob2123');
        mkdir(TEST_REPOSITORY.'/bob2');

        $fp = fopen(TEST_FILE, 'w');
        fwrite($fp, 'x');
        fclose($fp);
        $files = ['file' => new UploadedFile(TEST_FILE, 'dummy.txt', 'text/plain', null, true)];

        // Empty-assembly trick: resumableTotalChunks/Size = 0 skips the concat
        // loop, so the code assembles straight from an existing tmpfs file
        // addressed by the attacker-supplied identifier + filename within the
        // attacker's own namespace.
        $this->sendRequest('POST', '/upload', [
            'resumableChunkNumber' => 1,
            'resumableTotalChunks' => 0,
            'resumableTotalSize' => 0,
            'resumableIdentifier' => 'SHARED',
            'resumableFilename' => 'secret.txt',
            'resumableRelativePath' => '/',
        ], $files);

        // The other Bob's file must not have leaked into the attacker's home ...
        $this->assertFileNotExists(TEST_REPOSITORY.'/bob2/secret.txt');
        // ... and must still be intact in the temporary directory.
        $this->assertFileExists($shared);
        $this->assertEquals('VICTIM SECRET', file_get_contents($shared));
    }

    public function testNormalUploadStillWorks()
    {
        $this->signIn('bobsmith@example.com', 'bob123');
        mkdir(TEST_REPOSITORY.'/bob');

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
        $this->assertFileExists(TEST_REPOSITORY.'/bob/sample.txt');
        $this->assertEquals('hello world', file_get_contents(TEST_REPOSITORY.'/bob/sample.txt'));
    }
}
