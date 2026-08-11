<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Tests\TestCase;

/**
 * Regression tests for GHSA-f74m-x83r-c4v4.
 *
 * GET /batchdownload used to stream a batch archive to anyone who supplied its
 * id, without checking that the current session created it. These tests lock in
 * the fix: a batch archive is bound to the session that created it and can only
 * be downloaded by that session.
 *
 * @internal
 */
class BatchDownloadAuthorizationTest extends TestCase
{
    protected $timestamp;

    protected function setUp(): void
    {
        $this->resetTempDir();

        $this->timestamp = time();
    }

    protected function tearDown(): void
    {
        $this->resetTempDir();
    }

    public function testCannotDownloadAnotherUsersBatchArchive()
    {
        // John (victim) creates a batch archive of a file inside his home dir.
        $uniqid = $this->createSecretBatchArchiveAs('john@example.com', 'john123', '/john');

        // Jack (attacker) is a separate regular user, with a different home dir
        // and the read/download/batchdownload permissions the route requires, so
        // the request reaches the controller instead of being stopped by the router.
        $this->signIn('jack@example.com', 'jack123');

        // Jack first creates an archive of his own, so his session is not empty.
        // The check has to be per archive id, not merely "this session created
        // some archive".
        mkdir(TEST_REPOSITORY.'/jack');
        file_put_contents(TEST_REPOSITORY.'/jack/jack.txt', 'jack');

        $this->sendRequest('POST', '/batchdownload', [
            'items' => [
                [
                    'type' => 'file',
                    'path' => '/jack.txt',
                    'name' => 'jack.txt',
                    'time' => $this->timestamp,
                ],
            ],
        ]);

        $this->assertOk();

        // Jack replays John's archive id.
        $this->sendRequest('GET', '/batchdownload', [
            'uniqid' => $uniqid,
        ]);

        // Before the fix this returned 200 with John's ZIP. Now the ownership
        // check denies it: Jack is redirected to '/' and gets no archive.
        $this->assertStatus(302);
        $this->assertEquals('/', $this->response->headers->get('Location'));
        $this->assertNotEquals(
            'attachment; filename=archive.zip',
            $this->streamedResponse->headers->get('content-disposition')
        );

        // The denied request must not consume or delete the victim's archive.
        $this->assertFileExists(TEST_TMP_PATH.$uniqid);
    }

    public function testUnknownBatchArchiveIdIsDenied()
    {
        // A valid, fully-permissioned user cannot fish for archive ids they
        // never created.
        $this->signIn('john@example.com', 'john123');

        $this->sendRequest('GET', '/batchdownload', [
            'uniqid' => 'unknownbatcharchiveid0000000000000000000000000000',
        ]);

        $this->assertStatus(302);
        $this->assertEquals('/', $this->response->headers->get('Location'));
    }

    // Declared last because the streamed download leaves the archive handle
    // open, like the existing download tests in FilesTest do.
    public function testOwnerCanDownloadOwnBatchArchive()
    {
        $uniqid = $this->createSecretBatchArchiveAs('john@example.com', 'john123', '/john');

        // The same session that created the archive downloads it.
        $this->sendRequest('GET', '/batchdownload', [
            'uniqid' => $uniqid,
        ]);

        $this->assertOk();

        $headers = $this->streamedResponse->headers;
        $this->assertEquals('application/octet-stream', $headers->get('content-type'));
        $this->assertEquals('attachment; filename=archive.zip', $headers->get('content-disposition'));
    }

    private function createSecretBatchArchiveAs(string $username, string $password, string $homedir): string
    {
        $this->signIn($username, $password);

        mkdir(TEST_REPOSITORY.$homedir);
        file_put_contents(TEST_REPOSITORY.$homedir.'/secret.txt', 'top secret');

        $this->sendRequest('POST', '/batchdownload', [
            'items' => [
                [
                    'type' => 'file',
                    'path' => '/secret.txt',
                    'name' => 'secret.txt',
                    'time' => $this->timestamp,
                ],
            ],
        ]);

        $this->assertOk();

        $res = json_decode($this->response->getContent());

        return $res->data->uniqid;
    }
}
