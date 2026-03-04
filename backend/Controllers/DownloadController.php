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
use Filegator\Services\Hooks\HooksInterface;
use Filegator\Services\PathACL\PathACLInterface;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Services\Storage\Filesystem;
use Filegator\Services\Tmpfs\TmpfsInterface;
use Symfony\Component\HttpFoundation\HeaderUtils;
use Symfony\Component\Mime\MimeTypes;

class DownloadController
{
    protected $auth;

    protected $session;

    protected $config;

    protected $storage;

    protected $pathacl;
    protected $hooks;

    public function __construct(Config $config, Session $session, AuthInterface $auth, Filesystem $storage, PathACLInterface $pathacl = null, HooksInterface $hooks = null)
    {
        $this->session = $session;
        $this->config = $config;
        $this->auth = $auth;
        $this->pathacl = $pathacl;
        $this->hooks = $hooks;

        $user = $this->auth->user() ?: $this->auth->getGuest();

        $this->storage = $storage;
        $this->storage->setPathPrefix($user->getHomeDir());
    }

    /**
     * Get the current user's username
     */
    protected function getUsername(): string
    {
        $user = $this->auth->user() ?: $this->auth->getGuest();
        return $user ? $user->getUsername() : 'guest';
    }

    /**
     * Check PathACL permission for the current user and path
     *
     * @param Request $request HTTP request object
     * @param string $path File/folder path
     * @param string $permission Permission to check
     * @return bool True if allowed
     */
    protected function checkPathACL(Request $request, string $path, string $permission): bool
    {
        // If PathACL is not injected or not enabled, allow (fall back to global permissions)
        if (!$this->pathacl || !$this->pathacl->isEnabled()) {
            return true;
        }

        $user = $this->auth->user() ?: $this->auth->getGuest();
        $clientIp = $request->getClientIp();

        return $this->pathacl->checkPermission($user, $clientIp, $path, $permission);
    }

    /**
     * Return 403 Forbidden response with error message
     *
     * @param Response $response Response object
     * @param string $message Error message
     * @return Response
     */
    protected function forbidden(Response $response, string $message = 'Access denied'): Response
    {
        $response->json($message, 403);
        return $response;
    }

    public function download(Request $request, Response $response, StreamedResponse $streamedResponse)
    {
        $path = (string) base64_decode($request->input('path'));

        // Check PathACL permission for download
        if (!$this->checkPathACL($request, $path, 'download')) {
            return $this->forbidden($response);
        }

        try {
            $file = $this->storage->readStream($path);
        } catch (\Exception $e) {
            return $response->redirect('/');
        }

        // Trigger onDownload hook
        if ($this->hooks) {
            $this->hooks->trigger('onDownload', [
                'file_path' => $path,
                'file_name' => $file['filename'],
                'file_size' => $file['filesize'] ?? 0,
                'user' => $this->getUsername(),
            ]);
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

        $extension = pathinfo($file['filename'], PATHINFO_EXTENSION);
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

        // Check PathACL permission for each item before creating archive
        foreach ($items as $item) {
            if (!$this->checkPathACL($request, $item->path, 'download')) {
                return $this->forbidden($response, 'Access denied: cannot download item ' . $item->path);
            }
        }

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

            // Trigger onDownload hook for each item in batch
            if ($this->hooks) {
                $this->hooks->trigger('onDownload', [
                    'file_path' => $item->path,
                    'file_name' => $item->name,
                    'type' => $item->type,
                    'batch_download' => true,
                    'user' => $this->getUsername(),
                ]);
            }
        }

        $archiver->closeArchive();

        return $response->json(['uniqid' => $uniqid]);
    }

    public function batchDownloadStart(Request $request, StreamedResponse $streamedResponse, TmpfsInterface $tmpfs)
    {
        $uniqid = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('uniqid'));
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
}
