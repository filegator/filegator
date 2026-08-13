<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Controllers;

use Filegator\Config\Config;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Kernel\StreamedResponse;
use Filegator\Services\Archiver\ArchiverInterface;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Services\Storage\Filesystem;
use Filegator\Services\Tmpfs\TmpfsInterface;
use Symfony\Component\HttpFoundation\HeaderUtils;
use Symfony\Component\Mime\MimeTypes;

class DownloadController
{
    // Session key holding the set of batch archive ids created by the current
    // session. Used to authorize batch downloads (see batchDownloadStart).
    const BATCH_ARCHIVES_SESSION_KEY = 'batch_download_archives';

    // Upper bound on the ids kept in the session, so it cannot grow forever.
    // Archives are normally downloaded right after being created.
    const BATCH_ARCHIVES_LIMIT = 20;

    protected $auth;

    protected $session;

    protected $config;

    protected $storage;

    public function __construct(Config $config, Session $session, AuthInterface $auth, Filesystem $storage)
    {
        $this->session = $session;
        $this->config = $config;
        $this->auth = $auth;

        $user = $this->auth->user() ?: $this->auth->getGuest();

        $this->storage = $storage;
        $this->storage->setPathPrefix($user->getHomeDir());
    }

    public function download(Request $request, Response $response, StreamedResponse $streamedResponse)
    {
        try {
            $file = $this->storage->readStream((string) base64_decode($request->input('path')));
        } catch (\Exception $e) {
            return $response->redirect('/');
        }

        $streamedResponse->setCallback(function () use ($file) {
            // @codeCoverageIgnoreStart
            set_time_limit(0);
            if ($file['stream']) {
                while (! feof($file['stream'])) {
                    echo fread($file['stream'], 1024 * 8);
                    if (ob_get_level() > 0) {ob_flush();}
                    flush();
                }
                fclose($file['stream']);
            }
            // @codeCoverageIgnoreEnd
        });

        $extension = strtolower(pathinfo($file['filename'], PATHINFO_EXTENSION));
        $mimes = (new MimeTypes())->getMimeTypes($extension);
        $contentType = !empty($mimes) ? $mimes[0] : 'application/octet-stream';

        $disposition = HeaderUtils::DISPOSITION_ATTACHMENT;

        $download_inline = (array)$this->config->get('download_inline', ['pdf']);
        if (in_array($extension, $download_inline) || in_array('*', $download_inline)) {
            $disposition = HeaderUtils::DISPOSITION_INLINE;
        }

        $contentDisposition = HeaderUtils::makeDisposition($disposition, $file['filename'], 'file');

        $streamedResponse->headers->set(
            'Content-Disposition',
            $contentDisposition
        );
        $streamedResponse->headers->set(
            'Content-Type',
            $contentType
        );
        $streamedResponse->headers->set(
            'Content-Transfer-Encoding',
            'binary'
        );
        $streamedResponse->headers->set(
            'X-Content-Type-Options',
            'nosniff'
        );
        if (isset($file['filesize'])) {
            $streamedResponse->headers->set(
                'Content-Length',
                $file['filesize']
            );
        }
        // @codeCoverageIgnoreStart
        if (APP_ENV == 'development') {
            $streamedResponse->headers->set(
                'Access-Control-Allow-Origin',
                $request->headers->get('Origin')
            );
            $streamedResponse->headers->set(
                'Access-Control-Allow-Credentials',
                'true'
            );
        }
        // @codeCoverageIgnoreEnd

        // close session so we can continue streaming, note: dev is single-threaded
        $this->session->save();

        $streamedResponse->send();
    }

    public function batchDownloadCreate(Request $request, Response $response, ArchiverInterface $archiver)
    {
        $items = $request->input('items', []);

        $uniqid = $archiver->createArchive($this->storage);

        // Bind this archive to the current session so that later only its
        // creator can download it. Must run before the session is saved below.
        $this->rememberBatchArchive($uniqid);

        // close session
        $this->session->save();

        foreach ($items as $item) {
            if ($item->type == 'dir') {
                $archiver->addDirectoryFromStorage($item->path);
            }
            if ($item->type == 'file') {
                $archiver->addFileFromStorage($item->path);
            }
        }

        $archiver->closeArchive();

        return $response->json(['uniqid' => $uniqid]);
    }

    public function batchDownloadStart(Request $request, Response $response, StreamedResponse $streamedResponse, TmpfsInterface $tmpfs)
    {
        $uniqid = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('uniqid'));

        // Authorization: only the session that created this archive may download
        // it. Without this check any user who learns another user's archive id
        // (e.g. from access logs) could retrieve their files. See GHSA-f74m-x83r-c4v4.
        if (! $this->ownsBatchArchive($uniqid)) {
            return $response->redirect('/');
        }

        $file = $tmpfs->readStream($uniqid);

        $streamedResponse->setCallback(function () use ($file, $tmpfs, $uniqid) {
            // @codeCoverageIgnoreStart
            set_time_limit(0);
            if ($file['stream']) {
                while (! feof($file['stream'])) {
                    echo fread($file['stream'], 1024 * 8);
                    if (ob_get_level() > 0) {ob_flush();}
                    flush();
                }
                fclose($file['stream']);
            }
            $tmpfs->remove($uniqid);
            // @codeCoverageIgnoreEnd
        });

        $streamedResponse->headers->set(
            'Content-Disposition',
            HeaderUtils::makeDisposition(
                HeaderUtils::DISPOSITION_ATTACHMENT,
                $this->config->get('frontend_config.default_archive_name'),
                'archive.zip'
            )
        );
        $streamedResponse->headers->set(
            'Content-Type',
            'application/octet-stream'
        );
        $streamedResponse->headers->set(
            'Content-Transfer-Encoding',
            'binary'
        );
        if (isset($file['filesize'])) {
            $streamedResponse->headers->set(
                'Content-Length',
                $file['filesize']
            );
        }
        // close session so we can continue streaming, note: dev is single-threaded
        $this->session->save();

        $streamedResponse->send();
    }

    protected function rememberBatchArchive(string $uniqid)
    {
        $archives = (array) $this->session->get(self::BATCH_ARCHIVES_SESSION_KEY, []);

        $archives[$uniqid] = true;

        if (count($archives) > self::BATCH_ARCHIVES_LIMIT) {
            $archives = array_slice($archives, -self::BATCH_ARCHIVES_LIMIT, null, true);
        }

        $this->session->set(self::BATCH_ARCHIVES_SESSION_KEY, $archives);
    }

    protected function ownsBatchArchive(string $uniqid): bool
    {
        if ($uniqid === '') {
            return false;
        }

        $archives = (array) $this->session->get(self::BATCH_ARCHIVES_SESSION_KEY, []);

        return isset($archives[$uniqid]);
    }
}
