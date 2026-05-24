<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Exception;
use Tests\TestCase;

/**
 * @internal
 */
class FilesTest extends TestCase
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

    public function testGuestCannotListDirectories()
    {
        $this->signOut();

        $this->sendRequest('POST', '/changedir', [
            'to' => '/',
        ]);

        $this->assertStatus(404);

        $this->sendRequest('POST', '/getdir', [
            'to' => '/',
        ]);

        $this->assertStatus(404);
    }

    public function testUserCanChangeDir()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/johnsub');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);

        $this->sendRequest('POST', '/changedir', [
            'to' => '/',
        ]);

        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'files' => [
                    0 => [
                        'type' => 'dir',
                        'path' => '/johnsub',
                        'name' => 'johnsub',
                    ],
                    1 => [
                        'type' => 'file',
                        'path' => '/john.txt',
                        'name' => 'john.txt',
                        'time' => $this->timestamp,
                    ],
                ],
            ],
        ]);
    }

    public function testUserCanListHisHomeDir()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/johnsub');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);

        $this->sendRequest('POST', '/getdir', [
            'dir' => '/',
        ]);

        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'files' => [
                    0 => [
                        'type' => 'dir',
                        'path' => '/johnsub',
                        'name' => 'johnsub',
                    ],
                    1 => [
                        'type' => 'file',
                        'path' => '/john.txt',
                        'name' => 'john.txt',
                        'time' => $this->timestamp,
                    ],
                ],
            ],
        ]);
    }

    public function testDeleteItems()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/johnsub');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);

        $items = [
            0 => [
                'type' => 'dir',
                'path' => '/johnsub',
                'name' => 'johnsub',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/john.txt',
                'name' => 'john.txt',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/deleteitems', [
            'items' => $items,
        ]);

        $this->assertOk();
    }

    public function testDownloadFileHeaders()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);
        file_put_contents(TEST_REPOSITORY.'/john/john.txt', '123456');
        touch(TEST_REPOSITORY.'/john/image.jpg', $this->timestamp);
        touch(TEST_REPOSITORY.'/john/vector.svg', $this->timestamp);
        touch(TEST_REPOSITORY.'/john/inlinedoc.pdf', $this->timestamp);

        $path_encoded = base64_encode('john.txt');
        $this->sendRequest('GET', '/download&path='.$path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''john.txt", $headers->get('content-disposition'));
        $this->assertEquals('text/plain', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(6, $headers->get('content-length'));
        $this->assertOk();

        $path_encoded = base64_encode('image.jpg');
        $this->sendRequest('GET', '/download&path='.$path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''image.jpg", $headers->get('content-disposition'));
        $this->assertEquals('image/jpeg', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(0, $headers->get('content-length'));
        $this->assertOk();

        $path_encoded = base64_encode('vector.svg');
        $this->sendRequest('GET', '/download&path='.$path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''vector.svg", $headers->get('content-disposition'));
        $this->assertEquals('image/svg+xml', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertOk();

        $path_encoded = base64_encode('inlinedoc.pdf');
        $this->sendRequest('GET', '/download&path='.$path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("inline; filename=file; filename*=utf-8''inlinedoc.pdf", $headers->get('content-disposition'));
        $this->assertEquals('application/pdf', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertOk();
    }

    public function testDownloadPDFFileHeaders()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/john.pdf', $this->timestamp);

        $path_encoded = base64_encode('john.pdf');
        $this->sendRequest('GET', '/download&path='.$path_encoded);

        $headers = $this->streamedResponse->headers;
        $this->assertEquals("inline; filename=file; filename*=utf-8''john.pdf", $headers->get('content-disposition'));
        $this->assertEquals('application/pdf', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(0, $headers->get('content-length'));

        $this->assertOk();
    }

    public function testDownloadUTF8File()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/ąčęėįšųū.txt', $this->timestamp);

        $path_encoded = base64_encode('/ąčęėįšųū.txt');
        $this->sendRequest('GET', '/download&path='.$path_encoded);

        $this->assertOk();
    }

    public function testGuestCannotDownloadFilesWithoutDownloadPermissions()
    {
        touch(TEST_REPOSITORY.'/test.txt', $this->timestamp);

        $path_encoded = base64_encode('test.txt');
        $this->sendRequest('GET', '/download&path='.$path_encoded);

        $this->assertStatus(404);
    }

    public function testDownloadFileOnlyWithPermissions()
    {
        // jane does not have download permissions
        $username = 'jane@example.com';
        $this->signIn($username, 'jane123');

        mkdir(TEST_REPOSITORY.'/jane');
        touch(TEST_REPOSITORY.'/jane/jane.txt', $this->timestamp);

        $path_encoded = base64_encode('jane.txt');
        $this->sendRequest('GET', '/download&path='.$path_encoded);

        $this->assertStatus(404);
    }

    public function testDownloadMissingFileThrowsRedirect()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        $path_encoded = base64_encode('missing.txt');
        $this->sendRequest('GET', '/download&path='.$path_encoded);

        $this->assertStatus(302);
    }

    public function testRenameJohnsFile()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);

        $this->sendRequest('POST', '/renameitem', [
            'from' => '/john.txt',
            'to' => '/john2.txt',
        ]);
        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/john/john2.txt');
        $this->assertFileNotExists(TEST_REPOSITORY.'/john/john.txt');
    }

    public function testRenameMissingfileThrowsException()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        $this->expectException(Exception::class);

        $this->sendRequest('POST', '/renameitem', [
            'from' => 'missing.txt',
            'to' => 'john2.txt',
        ]);
    }

    public function testDeleteMissingItemsThrowsException()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/missing',
                'name' => 'missing',
                'time' => $this->timestamp,
            ],
        ];

        $this->expectException(Exception::class);

        $this->sendRequest('POST', '/deleteitems', [
            'items' => $items,
        ]);
    }

    public function testCreateNewDirAndFileInside()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');

        $this->sendRequest('POST', '/createnew', [
            'type' => 'dir',
            'name' => 'maximus',
        ]);
        $this->assertOk();

        $this->sendRequest('POST', '/changedir', [
            'to' => '/maximus/',
        ]);
        $this->assertOk();

        $this->sendRequest('POST', '/createnew', [
            'type' => 'file',
            'name' => 'samplefile.txt',
        ]);
        $this->assertOk();

        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/maximus');
        $this->assertFileExists(TEST_REPOSITORY.'/john/maximus/samplefile.txt');
    }

    public function testCopyAdminFiles()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY.'/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY.'/c.zip', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/sub');
        mkdir(TEST_REPOSITORY.'/sub/sub1');
        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/johnsub');

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/a.txt',
                'name' => 'a.txt',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/c.zip',
                'name' => 'c.zip',
                'time' => $this->timestamp,
            ],
            2 => [
                'type' => 'dir',
                'path' => '/sub',
                'name' => 'sub',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/copyitems', [
            'items' => $items,
            'destination' => '/john/johnsub/',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/john/johnsub/a.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/john/johnsub/c.zip');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/johnsub/sub/');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/johnsub/sub/sub1');
    }

    public function testCopyInvalidFilesThrowsException()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/missin.txt',
                'name' => 'missina.txt',
                'time' => $this->timestamp,
            ],
        ];

        $this->expectException(Exception::class);

        $this->sendRequest('POST', '/copyitems', [
            'items' => $items,
            'destination' => '/john/johnsub/',
        ]);
    }

    public function testMoveFiles()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY.'/b.txt', $this->timestamp);

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/a.txt',
                'name' => 'a.txt',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/b.txt',
                'name' => 'b.txt',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/moveitems', [
            'items' => $items,
            'destination' => '/john',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/john/a.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/john/b.txt');
        $this->assertFileNotExists(TEST_REPOSITORY.'/a.txt');
        $this->assertFileNotExists(TEST_REPOSITORY.'/b.txt');
    }

    public function testMoveDirsWithContent()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY.'/sub');
        mkdir(TEST_REPOSITORY.'/sub/sub1');
        touch(TEST_REPOSITORY.'/sub/sub1/f.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/jane');
        touch(TEST_REPOSITORY.'/jane/cookie.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/john');

        $items = [
            0 => [
                'type' => 'dir',
                'path' => '/sub',
                'name' => 'sub',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'dir',
                'path' => '/jane',
                'name' => 'jane',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/moveitems', [
            'items' => $items,
            'destination' => '/john',
        ]);

        $this->assertOk();

        $this->assertDirectoryNotExists(TEST_REPOSITORY.'/jane');
        $this->assertDirectoryNotExists(TEST_REPOSITORY.'/sub');
        $this->assertFileNotExists(TEST_REPOSITORY.'/sub/sub1/f.txt');

        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/jane');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/sub');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/john/sub/sub1');
        $this->assertFileExists(TEST_REPOSITORY.'/john/sub/sub1/f.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/john/jane/cookie.txt');
    }

    public function testZipFilesOnly()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY.'/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY.'/b.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/john');

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/a.txt',
                'name' => 'a.txt',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/b.txt',
                'name' => 'b.txt',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/zipitems', [
            'name' => 'compressed.zip',
            'items' => $items,
            'destination' => '/john',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/a.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/b.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/john/compressed.zip');
    }

    public function testZipFilesAndDirectories()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY.'/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY.'/b.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/sub');
        mkdir(TEST_REPOSITORY.'/jane');

        $items = [
            0 => [
                'type' => 'file',
                'path' => '/a.txt',
                'name' => 'a.txt',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/b.txt',
                'name' => 'b.txt',
                'time' => $this->timestamp,
            ],
            2 => [
                'type' => 'dir',
                'path' => '/sub',
                'name' => 'sub',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/zipitems', [
            'name' => 'compressed2.zip',
            'items' => $items,
            'destination' => '/jane',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/a.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/b.txt');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/sub');
        $this->assertFileExists(TEST_REPOSITORY.'/jane/compressed2.zip');
    }

    public function testUnzipArchive()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        copy(TEST_ARCHIVE, TEST_REPOSITORY.'/c.zip');
        mkdir(TEST_REPOSITORY.'/jane');

        $this->sendRequest('POST', '/unzipitem', [
            'item' => '/c.zip',
            'destination' => '/jane',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY.'/jane/one.txt');
        $this->assertFileExists(TEST_REPOSITORY.'/jane/two.txt');
        $this->assertDirectoryExists(TEST_REPOSITORY.'/jane/onetwo');
        $this->assertFileExists(TEST_REPOSITORY.'/jane/onetwo/three.txt');
    }

    public function testDownloadMultipleItems()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/john.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/john/johnsub');
        touch(TEST_REPOSITORY.'/john/johnsub/sub.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/john/johnsub/sub2');

        $items = [
            0 => [
                'type' => 'dir',
                'path' => '/johnsub',
                'name' => 'johnsub',
                'time' => $this->timestamp,
            ],
            1 => [
                'type' => 'file',
                'path' => '/john.txt',
                'name' => 'john.txt',
                'time' => $this->timestamp,
            ],
        ];

        $this->sendRequest('POST', '/batchdownload', [
            'items' => $items,
        ]);

        $this->assertOk();

        $res = json_decode($this->response->getContent());
        $uniqid = $res->data->uniqid;

        $this->sendRequest('GET', '/batchdownload', [
            'uniqid' => $uniqid,
        ]);

        $this->assertOk();

        // test headers
        $this->response->getContent();
        $headers = $this->streamedResponse->headers;
        $this->assertEquals('application/octet-stream', $headers->get('content-type'));
        $this->assertEquals('attachment; filename=archive.zip', $headers->get('content-disposition'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(414, $headers->get('content-length'));
    }

    public function testUpdateFileContent()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        file_put_contents(TEST_REPOSITORY.'/john/john.txt', 'lorem ipsum');

        $this->sendRequest('POST', '/savecontent', [
            'name' => 'john.txt',
            'content' => 'lorem ipsum new',
        ]);

        $this->assertOk();

        $updated = file_get_contents(TEST_REPOSITORY.'/john/john.txt');

        $this->assertEquals('lorem ipsum new', $updated);
    }

    public function testUpdateFileContentInSubDir()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/sub');
        file_put_contents(TEST_REPOSITORY.'/john/sub/john.txt', 'lorem ipsum');

        $this->sendRequest('POST', '/changedir', [
            'to' => '/sub/',
        ]);

        $this->sendRequest('POST', '/savecontent', [
            'name' => 'john.txt',
            'content' => 'lorem ipsum new',
        ]);

        $this->assertOk();

        $updated = file_get_contents(TEST_REPOSITORY.'/john/sub/john.txt');

        $this->assertEquals('lorem ipsum new', $updated);
    }

    // --------------------------------------------------------------------
    // Cross-user folder isolation — characterization pins.
    //
    // Today FileController constructs a single path prefix from the user's
    // homedir; every storage operation runs through Filesystem::applyPathPrefix
    // which collapses any `..` path back to the prefix root. These tests pin
    // that contract: a user A authenticated session cannot leak filesystem
    // contents from another user B's homedir, regardless of the crafted path
    // shape (absolute, traversal, mixed). This is the safety net the upcoming
    // multi-folder refactor must preserve.
    // --------------------------------------------------------------------

    protected function seedJohnAndJaneWithSecret(): void
    {
        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/john-public.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/jane');
        file_put_contents(TEST_REPOSITORY.'/jane/secret.txt', 'jane-private-payload');
    }

    public function testJohnCannotListJanesHomedirViaChangedirAbsolute()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        // Absolute path pointing at jane's homedir. applyPathPrefix joins it
        // under john's prefix → /john/jane → doesn't exist → empty listing.
        $this->sendRequest('POST', '/changedir', ['to' => '/jane']);

        $body = (string) $this->response->getContent();
        $this->assertStringNotContainsString('secret.txt', $body);
        $this->assertStringNotContainsString('jane-private-payload', $body);
    }

    public function testJohnCannotListJanesHomedirViaChangedirTraversal()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        // ../../jane and friends — Filesystem::applyPathPrefix collapses any
        // `..` segment to the prefix root.
        foreach (['../jane', '/../jane', '../../jane', '/../../jane'] as $path) {
            $this->sendRequest('POST', '/changedir', ['to' => $path]);
            $body = (string) $this->response->getContent();
            $this->assertStringNotContainsString(
                'secret.txt',
                $body,
                "Traversal path {$path} leaked jane's secret.txt"
            );
        }
    }

    public function testJohnCannotListJanesHomedirViaGetdir()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        foreach (['/jane', '../jane', '/../jane'] as $path) {
            $this->sendRequest('POST', '/getdir', ['dir' => $path]);
            $body = (string) $this->response->getContent();
            $this->assertStringNotContainsString(
                'secret.txt',
                $body,
                "getdir path {$path} leaked jane's secret.txt"
            );
        }
    }

    public function testJohnCannotReadJanesFileViaDownload()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        // Crafted relative + absolute paths; the download endpoint base64-decodes
        // the `path` query value and feeds it through Filesystem.
        foreach (['../jane/secret.txt', '/jane/secret.txt', '/../jane/secret.txt'] as $craft) {
            $path_encoded = base64_encode($craft);
            $this->sendRequest('GET', '/download&path='.$path_encoded);

            // Either a redirect (download-missing path) or any non-200 is fine —
            // the failure case is a 200 carrying jane's content.
            if ($this->response->isOk()) {
                // If somehow OK, body must not contain jane's payload.
                $body = (string) $this->response->getContent();
                $this->assertStringNotContainsString('jane-private-payload', $body, "Crafted path {$craft} leaked jane's content");
            }

            // jane's file must still exist on disk untouched.
            $this->assertSame('jane-private-payload', file_get_contents(TEST_REPOSITORY.'/jane/secret.txt'));
        }
    }

    public function testJohnCannotWriteIntoJanesHomedirViaSaveContent()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        // Crafted name with `..` — escapeDots() should strip the traversal
        // segment so it lands under /john, not /jane.
        try {
            $this->sendRequest('POST', '/savecontent', [
                'name' => '../jane/secret.txt',
                'content' => 'overwritten-by-john',
            ]);
        } catch (Exception $e) {
            // Acceptable: any thrown exception means the write didn't land.
        }

        // jane's file must be untouched.
        $this->assertSame('jane-private-payload', file_get_contents(TEST_REPOSITORY.'/jane/secret.txt'));
    }

    public function testJohnCannotRenameAcrossHomedirs()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        try {
            $this->sendRequest('POST', '/renameitem', [
                'from' => '/jane/secret.txt',
                'to'   => '/john-public.txt',
            ]);
        } catch (Exception $e) {
            // expected — source doesn't exist inside john's prefix.
        }

        $this->assertFileExists(TEST_REPOSITORY.'/jane/secret.txt');
        $this->assertSame('jane-private-payload', file_get_contents(TEST_REPOSITORY.'/jane/secret.txt'));
    }

    public function testJohnCannotMoveItemsIntoJanesHomedir()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        try {
            $this->sendRequest('POST', '/moveitems', [
                'items' => [
                    ['type' => 'file', 'path' => '/john-public.txt', 'name' => 'john-public.txt', 'time' => $this->timestamp],
                ],
                'destination' => '../jane',
            ]);
        } catch (Exception $e) {
        }

        // applyPathPrefix collapses `../` so destination becomes /john root.
        // The file may have been moved within john's homedir, but it must NOT
        // have landed inside jane's actual /jane folder.
        $this->assertFileNotExists(TEST_REPOSITORY.'/jane/john-public.txt');
        // jane's secret stays intact regardless.
        $this->assertFileExists(TEST_REPOSITORY.'/jane/secret.txt');
    }

    public function testJohnCannotCopyJanesFileIntoOwnHomedir()
    {
        $this->signIn('john@example.com', 'john123');
        $this->seedJohnAndJaneWithSecret();

        try {
            $this->sendRequest('POST', '/copyitems', [
                'items' => [
                    ['type' => 'file', 'path' => '/jane/secret.txt', 'name' => 'secret.txt', 'time' => $this->timestamp],
                ],
                'destination' => '/',
            ]);
        } catch (Exception $e) {
            // expected — source path doesn't resolve inside john's prefix.
        }

        // jane's secret content must not have been duplicated into /john.
        $this->assertFileNotExists(TEST_REPOSITORY.'/john/secret.txt');
        $this->assertSame('jane-private-payload', file_get_contents(TEST_REPOSITORY.'/jane/secret.txt'));
    }

    public function testChangedirTraversalCollapsesToHomedirRoot()
    {
        $this->signIn('john@example.com', 'john123');
        mkdir(TEST_REPOSITORY.'/john');
        touch(TEST_REPOSITORY.'/john/marker.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY.'/elsewhere');
        touch(TEST_REPOSITORY.'/elsewhere/elsewhere.txt', $this->timestamp);

        // All these escape attempts should collapse back to john's root, NOT
        // surface /elsewhere or any sibling content.
        foreach (['..', '../', '/../', '../..', '/../../', '../../../etc'] as $craft) {
            $this->sendRequest('POST', '/changedir', ['to' => $craft]);
            $body = (string) $this->response->getContent();

            $this->assertStringContainsString('marker.txt', $body, "Crafted path {$craft} did not resolve to john's root");
            $this->assertStringNotContainsString('elsewhere.txt', $body, "Crafted path {$craft} leaked sibling content");
        }
    }

    public function testSessionCwdIsolatedAcrossUsers()
    {
        // Pin the invariant that one user's SESSION_CWD cannot affect another
        // user's session. signIn() creates a fresh session, but the contract
        // is worth pinning so the multi-folder refactor (which adds
        // SESSION_ACTIVE_HOMEDIR alongside SESSION_CWD) can't accidentally
        // start sharing state across users.
        $this->signIn('john@example.com', 'john123');
        mkdir(TEST_REPOSITORY.'/john');
        mkdir(TEST_REPOSITORY.'/john/johnsub');
        $this->sendRequest('POST', '/changedir', ['to' => '/johnsub']);
        $this->assertOk();

        // Switch users — fresh session.
        $this->signIn('jane@example.com', 'jane123');
        mkdir(TEST_REPOSITORY.'/jane');
        touch(TEST_REPOSITORY.'/jane/jane-marker.txt', $this->timestamp);

        $this->sendRequest('POST', '/getdir');
        $body = (string) $this->response->getContent();

        // jane gets her own homedir root, not whatever john last set as cwd.
        $this->assertStringContainsString('jane-marker.txt', $body);
        $this->assertStringNotContainsString('johnsub', $body);
    }
}
