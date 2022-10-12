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
 * @internal
 */
class UploadTest extends TestCase
{
    protected function setUp(): void
    {
        $this->resetTempDir();

        parent::setUp();
    }

    public function testSimpleFileUpload()
    {
        $this->signIn('john@example.com', 'john123');

        // create 5Kb dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 0.5 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 0.5 * 1024 * 1024,
            'resumableTotalChunks' => 1,
            'resumableTotalSize' => 0.5 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-SIMPLE-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('GET', '/upload', $data, $files);

        $this->assertStatus(204);

        $this->sendRequest('POST', '/upload', $data, $files);

        $this->assertOk();

        $this->sendRequest('POST', '/getdir', [
            'dir' => '/',
        ]);

        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'files' => [
                    0 => [
                        'type' => 'file',
                        'path' => '/sample.txt',
                        'name' => 'sample.txt',
                    ],
                ],
            ],
        ]);
    }

    public function testFileUploadWithTwoChunks()
    {
        $this->signIn('john@example.com', 'john123');

        // create 1MB dummy file part 1
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 1 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 1 * 1024 * 1024,
            'resumableTotalChunks' => 2,
            'resumableTotalSize' => 1.5 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-MULTIPART-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        // part does not exists
        $this->sendRequest('GET', '/upload', $data, $files);
        $this->assertStatus(204);

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertOk();

        // this part should already exists, no need to upload again
        $this->sendRequest('GET', '/upload', $data, $files);
        $this->assertOk();

        // create 512Kb dummy file part 2
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 0.5 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 2,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 0.5 * 1024 * 1024,
            'resumableTotalChunks' => 2,
            'resumableTotalSize' => 1.5 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-MULTIPART-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        // part does not exists
        $this->sendRequest('GET', '/upload', $data, $files);
        $this->assertStatus(204);

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertOk();

        $this->sendRequest('POST', '/getdir', [
            'dir' => '/',
        ]);

        $this->assertResponseJsonHas([
            'data' => [
                'files' => [
                    0 => [
                        'type' => 'file',
                        'name' => 'sample.txt',
                        'path' => '/sample.txt',
                        'size' => 1572864,
                    ],
                ],
            ],
        ]);
    }

    public function testUploadInvalidFile()
    {
        $this->signIn('john@example.com', 'john123');

        $file = [
            'tmp_name' => TEST_FILE,
            'full_path' => 'something', // new in php 8.1
            'name' => 'something',
            'type' => 'application/octet-stream',
            'size' => 12345,
            'error' => 0,
        ];

        $files = ['file' => $file];
        $data = [];

        $this->sendRequest('GET', '/upload', $data, $files);

        $this->assertStatus(204);

        $this->sendRequest('POST', '/upload', $data, $files);

        $this->assertStatus(422);
    }

    public function testUploadFileBiggerThanAllowed()
    {
        $this->signIn('john@example.com', 'john123');

        // create 3MB dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 3 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 1 * 1024 * 1024,
            'resumableTotalChunks' => 1,
            'resumableTotalSize' => 1 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-FAILED-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);

        $this->assertStatus(422);
    }

    public function testUploadFileBiggerThanAllowedUsingChunks()
    {
        $this->signIn('john@example.com', 'john123');

        // create 1MB dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 1 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 1 * 1024 * 1024,
            'resumableTotalChunks' => 3,
            'resumableTotalSize' => 2 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-FAILED2-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertOk();

        // create 512Kb dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, .5 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];
        $data = [
            'resumableChunkNumber' => 2,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 0.5 * 1024 * 1024,
            'resumableTotalChunks' => 3,
            'resumableTotalSize' => 2 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-FAILED2-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertOk();

        // create 1MB dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 1 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];
        $data = [
            'resumableChunkNumber' => 3,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 1 * 1024 * 1024,
            'resumableTotalChunks' => 3,
            'resumableTotalSize' => 2 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-FAILED2-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertStatus(422);

        // create 1MB dummy file
        $fp = fopen(TEST_FILE, 'w');
        fseek($fp, 1 * 1024 * 1024 - 1, SEEK_CUR);
        fwrite($fp, 'a');
        fclose($fp);

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];
        $data = [
            'resumableChunkNumber' => 3,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 1 * 1024 * 1024,
            'resumableTotalChunks' => 3,
            'resumableTotalSize' => 2 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-FAILED2-TEST',
            'resumableFilename' => 'sample.txt',
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);
        $this->assertStatus(422);
    }

    public function testFileUploadWithBadName()
    {
        $this->signIn('john@example.com', 'john123');

        $files = ['file' => new UploadedFile(TEST_FILE, 'sample.txt', 'text/plain', null, true)];

        $data = [
            'resumableChunkNumber' => 1,
            'resumableChunkSize' => 1048576,
            'resumableCurrentChunkSize' => 0.5 * 1024 * 1024,
            'resumableTotalChunks' => 1,
            'resumableTotalSize' => 0.5 * 1024 * 1024,
            'resumableType' => 'text/plain',
            'resumableIdentifier' => 'CHUNKS-SIMPLE-TEST',
            'resumableFilename' => "../\\s\"u<:>pe////rm?*|an\\.t\txt../;",
            'resumableRelativePath' => '/',
        ];

        $this->sendRequest('POST', '/upload', $data, $files);

        $this->assertOk();

        $this->sendRequest('POST', '/getdir', [
            'dir' => '/',
        ]);

        $this->assertResponseJsonHas([
            'data' => [
                'files' => [
                    0 => [
                        'type' => 'file',
                        'path' => '/..--s-u---pe----rm---an-.t-xt..--',
                        'name' => '..--s-u---pe----rm---an-.t-xt..--',
                    ],
                ],
            ],
        ]);
    }
}
