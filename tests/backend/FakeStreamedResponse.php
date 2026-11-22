<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests;

use Filegator\Kernel\StreamedResponse;

class FakeStreamedResponse extends StreamedResponse
{
    /**
     * Called by DownloadController (e.g. download() and batchDownloadStart())
     * at the end of file download requests.
     *
     * In production, Symfony's StreamedResponse::send() flushes the file chunks to
     * the HTTP client, where the callback closes the stream handle and deletes any
     * temporary archive.
     *
     * In tests, FakeStreamedResponse overrides send() to prevent dumping file chunks
     * to stdout. We execute the callback inside nested output buffering so that the
     * stream is closed and temporary files are removed, preventing Windows file
     * locking errors when tearDown() cleans up the temporary directories.
     */
    public function send()
    {
        if ($this->callback) {
            $level = ob_get_level();
            ob_start();
            ob_start();
            ($this->callback)();
            while (ob_get_level() > $level) {
                ob_end_clean();
            }
        }
    }
}
