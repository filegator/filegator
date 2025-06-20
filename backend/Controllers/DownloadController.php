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
use Filegator\Services\Logger\LoggerInterface;

class DownloadController
{
    protected $auth;
    protected $session;
    protected $config;
    protected $storage;
    protected $logger;

    public function __construct(
        Config $config, 
        Session $session, 
        AuthInterface $auth, 
        Filesystem $storage,
        LoggerInterface $logger
    ) {
        $this->session = $session;
        $this->config = $config;
        $this->auth = $auth;
        $this->logger = $logger;

        // Set user-specific directory prefix for file storage
        $user = $this->auth->user() ?: $this->auth->getGuest();
        $this->storage = $storage;
        $this->storage->setPathPrefix($user->getHomeDir());
    }

    public function download(Request $request, Response $response, StreamedResponse $streamedResponse)
    {
        $user = $this->auth->user() ?: $this->auth->getGuest();
        $ip = $request->getClientIp();
        
        try {
            // Decode base64-encoded path from request
            $path = (string) base64_decode($request->input('path'));
            // Get file stream for download
            $file = $this->storage->readStream($path);
            
            // Log successful download event
            $this->logger->log("User {$user->getUsername()} downloaded file: {$file['filename']} (IP: $ip)");
        } catch (\Exception $e) {
            // Log download failure with error message
            $this->logger->log("Failed download attempt. Error: {$e->getMessage()} (IP: $ip)");
            return $response->redirect('/');
        }

        // Configure streamed response for file transfer
        $streamedResponse->setCallback(function () use ($file) {
            set_time_limit(0);
            if ($file['stream']) {
                while (! feof($file['stream'])) {
                    echo fread($file['stream'], 1024 * 8);
                    if (ob_get_level() > 0) {ob_flush();}
                    flush();
                }
                fclose($file['stream']);
            }
        });

        // Determine content type from file extension
        $extension = pathinfo($file['filename'], PATHINFO_EXTENSION);
        $mimes = (new MimeTypes())->getMimeTypes($extension);
        $contentType = !empty($mimes) ? $mimes[0] : 'application/octet-stream';

        // Set content disposition (attachment/inline)
        $disposition = HeaderUtils::DISPOSITION_ATTACHMENT;
        $download_inline = (array)$this->config->get('download_inline', ['pdf']);
        if (in_array($extension, $download_inline) || in_array('*', $download_inline)) {
            $disposition = HeaderUtils::DISPOSITION_INLINE;
        }

        // Configure response headers
        $contentDisposition = HeaderUtils::makeDisposition($disposition, $file['filename'], 'file');
        $streamedResponse->headers->set('Content-Disposition', $contentDisposition);
        $streamedResponse->headers->set('Content-Type', $contentType);
        $streamedResponse->headers->set('Content-Transfer-Encoding', 'binary');
        
        if (isset($file['filesize'])) {
            $streamedResponse->headers->set('Content-Length', $file['filesize']);
        }

        // CORS headers for development environment
        if (APP_ENV == 'development') {
            $streamedResponse->headers->set('Access-Control-Allow-Origin', $request->headers->get('Origin'));
            $streamedResponse->headers->set('Access-Control-Allow-Credentials', 'true');
        }

        // Save session to release lock before streaming
        $this->session->save();
        $streamedResponse->send();
    }

    public function batchDownloadCreate(
        Request $request, 
        Response $response, 
        ArchiverInterface $archiver
    ) {
        $user = $this->auth->user() ?: $this->auth->getGuest();
        $ip = $request->getClientIp();
        
        // Get selected items for batch download
        $items = $request->input('items', []);
        // Initialize archive with unique ID
        $uniqid = $archiver->createArchive($this->storage);

        $this->session->save();

        // Collect file/directory paths for archiving
        $files = [];
        foreach ($items as $item) {
            if ($item->type == 'dir') {
                $archiver->addDirectoryFromStorage($item->path);
                $files[] = "dir:{$item->path}";
            }
            if ($item->type == 'file') {
                $archiver->addFileFromStorage($item->path);
                $files[] = "file:{$item->path}";
            }
        }

        // Finalize archive file
        $archiver->closeArchive();
        
        // Log batch download initiation and contents
        $this->logger->log("User {$user->getUsername()} initiated batch download of " . count($items) . " items (IP: $ip)");
        $this->logger->log("Batch archive contents: " . implode(', ', $files));

        return $response->json(['uniqid' => $uniqid]);
    }

    public function batchDownloadStart(
        Request $request, 
        StreamedResponse $streamedResponse, 
        TmpfsInterface $tmpfs
    ) {
        $user = $this->auth->user() ?: $this->auth->getGuest();
        $ip = $request->getClientIp();
        
        // Sanitize unique archive ID from request
        $uniqid = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('uniqid'));
        // Get archive file stream
        $file = $tmpfs->readStream($uniqid);

        // Configure stream transfer with cleanup
        $streamedResponse->setCallback(function () use ($file, $tmpfs, $uniqid) {
            set_time_limit(0);
            if ($file['stream']) {
                while (! feof($file['stream'])) {
                    echo fread($file['stream'], 1024 * 8);
                    if (ob_get_level() > 0) {ob_flush();}
                    flush();
                }
                fclose($file['stream']);
            }
            $tmpfs->remove($uniqid); // Clean up temporary archive
        });

        // Configure archive file download headers
        $streamedResponse->headers->set(
            'Content-Disposition',
            HeaderUtils::makeDisposition(
                HeaderUtils::DISPOSITION_ATTACHMENT,
                $this->config->get('frontend_config.default_archive_name'),
                'archive.zip'
            )
        );
        $streamedResponse->headers->set('Content-Type', 'application/octet-stream');
        $streamedResponse->headers->set('Content-Transfer-Encoding', 'binary');
        
        if (isset($file['filesize'])) {
            $streamedResponse->headers->set('Content-Length', $file['filesize']);
        }

        $this->session->save();
        
        // Log archive file download event
        $this->logger->log("User {$user->getUsername()} downloading batch archive file (IP: $ip)");
        $streamedResponse->send();
    }
}
