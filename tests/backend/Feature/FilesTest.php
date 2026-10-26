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

        mkdir(TEST_REPOSITORY . '/john');
        mkdir(TEST_REPOSITORY . '/john/johnsub');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);

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

        mkdir(TEST_REPOSITORY . '/john');
        mkdir(TEST_REPOSITORY . '/john/johnsub');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);

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

        mkdir(TEST_REPOSITORY . '/john');
        mkdir(TEST_REPOSITORY . '/john/johnsub');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);

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

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);
        file_put_contents(TEST_REPOSITORY . '/john/john.txt', '123456');
        touch(TEST_REPOSITORY . '/john/image.jpg', $this->timestamp);
        touch(TEST_REPOSITORY . '/john/vector.svg', $this->timestamp);
        touch(TEST_REPOSITORY . '/john/inlinedoc.pdf', $this->timestamp);

        $path_encoded = base64_encode('john.txt');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''john.txt", $headers->get('content-disposition'));
        $this->assertEquals('text/plain', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(6, $headers->get('content-length'));
        $this->assertOk();

        $path_encoded = base64_encode('image.jpg');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''image.jpg", $headers->get('content-disposition'));
        $this->assertEquals('image/jpeg', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(0, $headers->get('content-length'));
        $this->assertOk();

        $path_encoded = base64_encode('vector.svg');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);
        $headers = $this->streamedResponse->headers;
        $this->assertEquals("attachment; filename=file; filename*=utf-8''vector.svg", $headers->get('content-disposition'));
        $this->assertEquals('image/svg+xml', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertOk();

        $path_encoded = base64_encode('inlinedoc.pdf');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);
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

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/john.pdf', $this->timestamp);

        $path_encoded = base64_encode('john.pdf');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $headers = $this->streamedResponse->headers;
        $this->assertEquals("inline; filename=file; filename*=utf-8''john.pdf", $headers->get('content-disposition'));
        $this->assertEquals('application/pdf', $headers->get('content-type'));
        $this->assertEquals('binary', $headers->get('content-transfer-encoding'));
        $this->assertEquals(0, $headers->get('content-length'));

        $this->assertOk();
    }

    public function testDownloadUppercaseExtensionPDFFileHeadersInline()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/JOHN.PDF', $this->timestamp);

        $path_encoded = base64_encode('JOHN.PDF');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $headers = $this->streamedResponse->headers;
        // uppercase extension should still be previewed inline (download_inline => ['pdf'])
        $this->assertEquals("inline; filename=file; filename*=utf-8''JOHN.PDF", $headers->get('content-disposition'));
        $this->assertEquals('application/pdf', $headers->get('content-type'));

        $this->assertOk();
    }

    public function testDownloadUTF8File()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/ąčęėįšųū.txt', $this->timestamp);

        $path_encoded = base64_encode('/ąčęėįšųū.txt');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $this->assertOk();
    }

    public function testGuestCannotDownloadFilesWithoutDownloadPermissions()
    {
        touch(TEST_REPOSITORY . '/test.txt', $this->timestamp);

        $path_encoded = base64_encode('test.txt');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $this->assertStatus(404);
    }

    public function testDownloadFileOnlyWithPermissions()
    {
        // jane does not have download permissions
        $username = 'jane@example.com';
        $this->signIn($username, 'jane123');

        mkdir(TEST_REPOSITORY . '/jane');
        touch(TEST_REPOSITORY . '/jane/jane.txt', $this->timestamp);

        $path_encoded = base64_encode('jane.txt');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $this->assertStatus(404);
    }

    public function testDownloadMissingFileThrowsRedirect()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        $path_encoded = base64_encode('missing.txt');
        $this->sendRequest('GET', '/download&path=' . $path_encoded);

        $this->assertStatus(302);
    }

    public function testRenameJohnsFile()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);

        $this->sendRequest('POST', '/renameitem', [
            'from' => '/john.txt',
            'to' => '/john2.txt',
        ]);
        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY . '/john/john2.txt');
        $this->assertFileNotExists(TEST_REPOSITORY . '/john/john.txt');
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

        mkdir(TEST_REPOSITORY . '/john');

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

        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/maximus');
        $this->assertFileExists(TEST_REPOSITORY . '/john/maximus/samplefile.txt');
    }

    public function testCopyAdminFiles()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY . '/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY . '/c.zip', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/sub');
        mkdir(TEST_REPOSITORY . '/sub/sub1');
        mkdir(TEST_REPOSITORY . '/john');
        mkdir(TEST_REPOSITORY . '/john/johnsub');

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

        $this->assertFileExists(TEST_REPOSITORY . '/john/johnsub/a.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/john/johnsub/c.zip');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/johnsub/sub/');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/johnsub/sub/sub1');
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

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY . '/b.txt', $this->timestamp);

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

        $this->assertFileExists(TEST_REPOSITORY . '/john/a.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/john/b.txt');
        $this->assertFileNotExists(TEST_REPOSITORY . '/a.txt');
        $this->assertFileNotExists(TEST_REPOSITORY . '/b.txt');
    }

    public function testMoveDirsWithContent()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY . '/sub');
        mkdir(TEST_REPOSITORY . '/sub/sub1');
        touch(TEST_REPOSITORY . '/sub/sub1/f.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/jane');
        touch(TEST_REPOSITORY . '/jane/cookie.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/john');

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

        $this->assertDirectoryNotExists(TEST_REPOSITORY . '/jane');
        $this->assertDirectoryNotExists(TEST_REPOSITORY . '/sub');
        $this->assertFileNotExists(TEST_REPOSITORY . '/sub/sub1/f.txt');

        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/jane');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/sub');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/john/sub/sub1');
        $this->assertFileExists(TEST_REPOSITORY . '/john/sub/sub1/f.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/john/jane/cookie.txt');
    }

    public function testCheckConflicts()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY . '/dest_dir');
        touch(TEST_REPOSITORY . '/dest_dir/exists.txt');
        mkdir(TEST_REPOSITORY . '/dest_dir/exists_folder');

        $items = [
            [
                'name' => 'exists.txt',
                'type' => 'file',
                'path' => '/exists.txt',
            ],
            [
                'name' => 'exists_folder',
                'type' => 'dir',
                'path' => '/exists_folder',
            ],
            [
                'name' => 'unique.txt',
                'type' => 'file',
                'path' => '/unique.txt',
            ],
        ];

        $this->sendRequest('POST', '/checkconflicts', [
            'destination' => '/dest_dir',
            'items' => $items,
        ]);

        $this->assertOk();
        $data = $this->decodeResponseJson()['data'];
        $this->assertCount(2, $data['conflicts']);
        $this->assertEquals('exists.txt', $data['conflicts'][0]['name']);
        $this->assertEquals('exists_folder', $data['conflicts'][1]['name']);
    }

    public function testMoveItemsWithReplaceAction()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        // Setup source and dest
        mkdir(TEST_REPOSITORY . '/src_plugin');
        file_put_contents(TEST_REPOSITORY . '/src_plugin/setup.php', 'new setup');
        mkdir(TEST_REPOSITORY . '/src_plugin/inc');
        file_put_contents(TEST_REPOSITORY . '/src_plugin/inc/helper.php', 'new helper');

        mkdir(TEST_REPOSITORY . '/dest_plugin');
        file_put_contents(TEST_REPOSITORY . '/dest_plugin/setup.php', 'old setup');
        mkdir(TEST_REPOSITORY . '/dest_plugin/inc');
        file_put_contents(TEST_REPOSITORY . '/dest_plugin/inc/helper.php', 'old helper');
        file_put_contents(TEST_REPOSITORY . '/dest_plugin/inc/preserved.php', 'keep this');

        $items = [
            [
                'name' => 'setup.php',
                'type' => 'file',
                'path' => '/src_plugin/setup.php',
            ],
            [
                'name' => 'inc',
                'type' => 'dir',
                'path' => '/src_plugin/inc',
            ],
        ];

        $this->sendRequest('POST', '/moveitems', [
            'destination' => '/dest_plugin',
            'items' => $items,
            'action' => 'replace',
        ]);

        $this->assertOk();

        // Check destination
        $this->assertFileExists(TEST_REPOSITORY . '/dest_plugin/setup.php');
        $this->assertEquals('new setup', file_get_contents(TEST_REPOSITORY . '/dest_plugin/setup.php'));
        $this->assertFileExists(TEST_REPOSITORY . '/dest_plugin/inc/helper.php');
        $this->assertEquals('new helper', file_get_contents(TEST_REPOSITORY . '/dest_plugin/inc/helper.php'));
        $this->assertFileExists(TEST_REPOSITORY . '/dest_plugin/inc/preserved.php');
        $this->assertEquals('keep this', file_get_contents(TEST_REPOSITORY . '/dest_plugin/inc/preserved.php'));

        // Check source cleaned up
        $this->assertFileNotExists(TEST_REPOSITORY . '/src_plugin/setup.php');
        $this->assertDirectoryNotExists(TEST_REPOSITORY . '/src_plugin/inc');
    }

    public function testMoveItemsWithRenameAction()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY . '/src_ren');
        file_put_contents(TEST_REPOSITORY . '/src_ren/file.txt', 'source content');

        mkdir(TEST_REPOSITORY . '/dest_ren');
        file_put_contents(TEST_REPOSITORY . '/dest_ren/file.txt', 'dest content');

        $items = [
            [
                'name' => 'file.txt',
                'type' => 'file',
                'path' => '/src_ren/file.txt',
            ],
        ];

        $this->sendRequest('POST', '/moveitems', [
            'destination' => '/dest_ren',
            'items' => $items,
            'action' => 'rename',
        ]);

        $this->assertOk();

        // Dest should have original and upcounted
        $this->assertFileExists(TEST_REPOSITORY . '/dest_ren/file.txt');
        $this->assertEquals('dest content', file_get_contents(TEST_REPOSITORY . '/dest_ren/file.txt'));
        $this->assertFileExists(TEST_REPOSITORY . '/dest_ren/file (1).txt');
        $this->assertEquals('source content', file_get_contents(TEST_REPOSITORY . '/dest_ren/file (1).txt'));
    }

    public function testCopyItemsWithReplaceAction()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY . '/src_cp');
        file_put_contents(TEST_REPOSITORY . '/src_cp/copy.txt', 'new copy content');

        mkdir(TEST_REPOSITORY . '/dest_cp');
        file_put_contents(TEST_REPOSITORY . '/dest_cp/copy.txt', 'old copy content');

        $items = [
            [
                'name' => 'copy.txt',
                'type' => 'file',
                'path' => '/src_cp/copy.txt',
            ],
        ];

        $this->sendRequest('POST', '/copyitems', [
            'destination' => '/dest_cp',
            'items' => $items,
            'action' => 'replace',
        ]);

        $this->assertOk();

        // Source is preserved
        $this->assertFileExists(TEST_REPOSITORY . '/src_cp/copy.txt');
        // Destination is replaced
        $this->assertFileExists(TEST_REPOSITORY . '/dest_cp/copy.txt');
        $this->assertEquals('new copy content', file_get_contents(TEST_REPOSITORY . '/dest_cp/copy.txt'));
    }

    public function testCopyItemsWithRenameAction()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        mkdir(TEST_REPOSITORY . '/src_cp_ren');
        file_put_contents(TEST_REPOSITORY . '/src_cp_ren/item.txt', 'source copy');

        mkdir(TEST_REPOSITORY . '/dest_cp_ren');
        file_put_contents(TEST_REPOSITORY . '/dest_cp_ren/item.txt', 'dest copy');

        $items = [
            [
                'name' => 'item.txt',
                'type' => 'file',
                'path' => '/src_cp_ren/item.txt',
            ],
        ];

        $this->sendRequest('POST', '/copyitems', [
            'destination' => '/dest_cp_ren',
            'items' => $items,
            'action' => 'rename',
        ]);

        $this->assertOk();

        // Source is preserved
        $this->assertFileExists(TEST_REPOSITORY . '/src_cp_ren/item.txt');
        // Destination has original and upcounted
        $this->assertFileExists(TEST_REPOSITORY . '/dest_cp_ren/item.txt');
        $this->assertEquals('dest copy', file_get_contents(TEST_REPOSITORY . '/dest_cp_ren/item.txt'));
        $this->assertFileExists(TEST_REPOSITORY . '/dest_cp_ren/item (1).txt');
        $this->assertEquals('source copy', file_get_contents(TEST_REPOSITORY . '/dest_cp_ren/item (1).txt'));
    }

    public function testZipFilesOnly()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY . '/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY . '/b.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/john');

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

        $this->assertFileExists(TEST_REPOSITORY . '/a.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/b.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/john/compressed.zip');
    }

    public function testZipFilesAndDirectories()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        touch(TEST_REPOSITORY . '/a.txt', $this->timestamp);
        touch(TEST_REPOSITORY . '/b.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/sub');
        mkdir(TEST_REPOSITORY . '/jane');

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

        $this->assertFileExists(TEST_REPOSITORY . '/a.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/b.txt');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/sub');
        $this->assertFileExists(TEST_REPOSITORY . '/jane/compressed2.zip');
    }

    public function testUnzipArchive()
    {
        $username = 'admin@example.com';
        $this->signIn($username, 'admin123');

        copy(TEST_ARCHIVE, TEST_REPOSITORY . '/c.zip');
        mkdir(TEST_REPOSITORY . '/jane');

        $this->sendRequest('POST', '/unzipitem', [
            'item' => '/c.zip',
            'destination' => '/jane',
        ]);

        $this->assertOk();

        $this->assertFileExists(TEST_REPOSITORY . '/jane/one.txt');
        $this->assertFileExists(TEST_REPOSITORY . '/jane/two.txt');
        $this->assertDirectoryExists(TEST_REPOSITORY . '/jane/onetwo');
        $this->assertFileExists(TEST_REPOSITORY . '/jane/onetwo/three.txt');
    }

    public function testDownloadMultipleItems()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY . '/john');
        touch(TEST_REPOSITORY . '/john/john.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/john/johnsub');
        touch(TEST_REPOSITORY . '/john/johnsub/sub.txt', $this->timestamp);
        mkdir(TEST_REPOSITORY . '/john/johnsub/sub2');

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

        mkdir(TEST_REPOSITORY . '/john');
        file_put_contents(TEST_REPOSITORY . '/john/john.txt', 'lorem ipsum');

        $this->sendRequest('POST', '/savecontent', [
            'name' => 'john.txt',
            'content' => 'lorem ipsum new',
        ]);

        $this->assertOk();

        $updated = file_get_contents(TEST_REPOSITORY . '/john/john.txt');

        $this->assertEquals('lorem ipsum new', $updated);
    }

    public function testUpdateFileContentInSubDir()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        mkdir(TEST_REPOSITORY . '/john');
        mkdir(TEST_REPOSITORY . '/john/sub');
        file_put_contents(TEST_REPOSITORY . '/john/sub/john.txt', 'lorem ipsum');

        $this->sendRequest('POST', '/changedir', [
            'to' => '/sub/',
        ]);

        $this->sendRequest('POST', '/savecontent', [
            'name' => 'john.txt',
            'content' => 'lorem ipsum new',
        ]);

        $this->assertOk();

        $updated = file_get_contents(TEST_REPOSITORY . '/john/sub/john.txt');

        $this->assertEquals('lorem ipsum new', $updated);
    }
}
