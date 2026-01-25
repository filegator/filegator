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
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Hooks\HooksInterface;
use Filegator\Services\PathACL\PathACLInterface;
use Filegator\Services\Storage\Filesystem;
use Filegator\Services\Tmpfs\TmpfsInterface;

class UploadController
{
    protected $auth;

    protected $config;

    protected $storage;

    protected $tmpfs;

    protected $hooks;

    protected $pathacl;

    public function __construct(Config $config, AuthInterface $auth, Filesystem $storage, TmpfsInterface $tmpfs, HooksInterface $hooks = null, PathACLInterface $pathacl = null)
    {
        $this->config = $config;
        $this->auth = $auth;
        $this->tmpfs = $tmpfs;
        $this->hooks = $hooks;
        $this->pathacl = $pathacl;

        $user = $this->auth->user() ?: $this->auth->getGuest();

        $this->storage = $storage;
        $this->storage->setPathPrefix($user->getHomeDir());
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

    public function chunkCheck(Request $request, Response $response)
    {
        $file_name = $request->input('resumableFilename', 'file');
        $identifier = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('resumableIdentifier'));
        $chunk_number = (int) $request->input('resumableChunkNumber');

        $chunk_file = 'multipart_'.$identifier.$file_name.'.part'.$chunk_number;

        if ($this->tmpfs->exists($chunk_file)) {
            return $response->json('Chunk exists', 200);
        }

        return $response->json('Chunk does not exists', 204);
    }

    public function upload(Request $request, Response $response)
    {
        $file_name = $request->input('resumableFilename', 'file');
        $destination = $request->input('resumableRelativePath');
        $chunk_number = (int) $request->input('resumableChunkNumber');
        $total_chunks = (int) $request->input('resumableTotalChunks');
        $total_size = (int) $request->input('resumableTotalSize');
        $identifier = (string) preg_replace('/[^0-9a-zA-Z_]/', '', (string) $request->input('resumableIdentifier'));

        // Check PathACL permission for upload (check on first chunk only)
        if ($chunk_number == 1) {
            if (!$this->checkPathACL($request, $destination, 'upload')) {
                return $this->forbidden($response);
            }
        }

        $filebag = $request->files;
        $file = $filebag->get('file');

        $overwrite_on_upload = (bool) $this->config->get('overwrite_on_upload', false);

        // php 8.1 fix
        // remove new key 'full_path' so it can preserve compatibility with symfony FileBag
        // see https://php.watch/versions/8.1/$_FILES-full-path
        if ($file && is_array($file) && array_key_exists('full_path', $file)) {
            unset($file['full_path']);
            $filebag->set('file', $file);
            $file = $filebag->get('file');
        }

        if (! $file || ! $file->isValid() || $file->getSize() > $this->config->get('frontend_config.upload_max_size')) {
            return $response->json('Bad file', 422);
        }

        $prefix = 'multipart_'.$identifier;

        if ($this->tmpfs->exists($prefix.'_error')) {
            return $response->json('Chunk too big', 422);
        }

        $stream = fopen($file->getPathName(), 'r');

        $this->tmpfs->write($prefix.$file_name.'.part'.$chunk_number, $stream);

        // check if all the parts present, and create the final destination file
        $chunks_size = 0;
        foreach ($this->tmpfs->findAll($prefix.'*') as $chunk) {
            $chunks_size += $chunk['size'];
        }

        // file too big, cleanup to protect server, set error trap
        if ($chunks_size > $this->config->get('frontend_config.upload_max_size')) {
            foreach ($this->tmpfs->findAll($prefix.'*') as $tmp_chunk) {
                $this->tmpfs->remove($tmp_chunk['name']);
            }
            $this->tmpfs->write($prefix.'_error', '');

            return $response->json('Chunk too big', 422);
        }

        // if all the chunks are present, create final file and store it
        if ($chunks_size >= $total_size) {
            for ($i = 1; $i <= $total_chunks; ++$i) {
                $part = $this->tmpfs->readStream($prefix.$file_name.'.part'.$i);
                $this->tmpfs->write($file_name, $part['stream'], true);
            }

            $final = $this->tmpfs->readStream($file_name);
            $res = $this->storage->store($destination, $final['filename'], $final['stream'], $overwrite_on_upload);

            // cleanup
            $this->tmpfs->remove($file_name);
            foreach ($this->tmpfs->findAll($prefix.'*') as $expired_chunk) {
                $this->tmpfs->remove($expired_chunk['name']);
            }

            // Trigger onUpload hook when file upload completes
            if ($res && $this->hooks) {
                $user = $this->auth->user() ?: $this->auth->getGuest();
                $this->hooks->trigger('onUpload', [
                    'file_path' => $destination,
                    'file_name' => $file_name,
                    'file_size' => $total_size,
                    'user' => $user ? $user->getUsername() : 'guest',
                    'home_dir' => $user ? $user->getHomeDir() : '/',
                ]);
            }

            return $res ? $response->json('Stored') : $response->json('Error storing file');
        }

        return $response->json('Uploaded');
    }
}
