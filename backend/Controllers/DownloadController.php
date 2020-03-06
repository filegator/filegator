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

class DownloadController
{
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
                    ob_flush();
                    flush();
                }
                fclose($file['stream']);
            }
            // @codeCoverageIgnoreEnd
        });

        $contentDisposition = HeaderUtils::DISPOSITION_ATTACHMENT;
        $contentType = 'application/octet-stream';

        if (pathinfo($file['filename'], PATHINFO_EXTENSION) == 'pdf') {
            $contentDisposition = HeaderUtils::DISPOSITION_INLINE;
            $contentType = 'application/pdf';
        }

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

    public function batchDownloadStart(Request $request, StreamedResponse $streamedResponse, TmpfsInterface $tmpfs)
    {
        $uniqid = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('uniqid'));

        $streamedResponse->setCallback(function () use ($tmpfs, $uniqid) {
            // @codeCoverageIgnoreStart
            set_time_limit(0);
            $file = $tmpfs->readStream($uniqid);
            if ($file['stream']) {
                while (! feof($file['stream'])) {
                    echo fread($file['stream'], 1024 * 8);
                    ob_flush();
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

        // close session so we can continue streaming, note: dev is single-threaded
        $this->session->save();

        $streamedResponse->send();
    }
}
