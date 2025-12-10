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
use Filegator\Services\Archiver\ArchiverInterface;
use Filegator\Services\Auth\AuthInterface;
<<<<<<< Updated upstream
=======
use Filegator\Services\Hooks\HooksInterface;
use Filegator\Services\PathACL\PathACLInterface;
>>>>>>> Stashed changes
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Services\Storage\Filesystem;

class FileController
{
    const SESSION_CWD = 'current_path';

    protected $session;

    protected $auth;

    protected $config;

    protected $storage;

    protected $separator;

<<<<<<< Updated upstream
    public function __construct(Config $config, Session $session, AuthInterface $auth, Filesystem $storage)
=======
    protected $pathacl;

    protected $hooks;

    public function __construct(Config $config, Session $session, AuthInterface $auth, Filesystem $storage, PathACLInterface $pathacl = null, HooksInterface $hooks = null)
>>>>>>> Stashed changes
    {
        $this->session = $session;
        $this->config = $config;
        $this->auth = $auth;
<<<<<<< Updated upstream
=======
        $this->pathacl = $pathacl;
        $this->hooks = $hooks;
>>>>>>> Stashed changes

        $user = $this->auth->user() ?: $this->auth->getGuest();

        $this->storage = $storage;
        $this->storage->setPathPrefix($user->getHomeDir());

        $this->separator = $this->storage->getSeparator();
    }

<<<<<<< Updated upstream
=======
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
    protected function forbidden(Response $response, string $message = 'Access denied by path ACL'): Response
    {
        $response->setStatusCode(403);
        return $response->json(['error' => $message]);
    }

>>>>>>> Stashed changes
    public function changeDirectory(Request $request, Response $response)
    {
        $path = $request->input('to', $this->separator);

        $this->session->set(self::SESSION_CWD, $path);

        return $response->json($this->storage->getDirectoryCollection($path));
    }

    public function getDirectory(Request $request, Response $response)
    {
        $path = $request->input('dir', $this->session->get(self::SESSION_CWD, $this->separator));

        $content = $this->storage->getDirectoryCollection($path);

        return $response->json($content);
    }

    public function createNew(Request $request, Response $response)
    {
        $type = $request->input('type', 'file');
        $name = $request->input('name');
        $path = $this->session->get(self::SESSION_CWD, $this->separator);

        if ($type == 'dir') {
            $this->storage->createDir($path, $request->input('name'));
        }
        if ($type == 'file') {
            $this->storage->createFile($path, $request->input('name'));
        }

        // Trigger onCreate hook
        if ($this->hooks) {
            $fullPath = trim($path, $this->separator) . $this->separator . ltrim($name, $this->separator);
            $this->hooks->trigger('onCreate', [
                'file_path' => $fullPath,
                'file_name' => $name,
                'type' => $type,
                'user' => $this->getUsername(),
            ]);
        }

        return $response->json('Done');
    }

    public function copyItems(Request $request, Response $response)
    {
        $items = $request->input('items', []);
        $destination = $request->input('destination', $this->separator);

        foreach ($items as $item) {
            if ($item->type == 'dir') {
                $this->storage->copyDir($item->path, $destination);
            }
            if ($item->type == 'file') {
                $this->storage->copyFile($item->path, $destination);
            }

            // Trigger onCopy hook for each item
            if ($this->hooks) {
                $this->hooks->trigger('onCopy', [
                    'source_path' => $item->path,
                    'destination' => $destination,
                    'file_name' => $item->name,
                    'type' => $item->type,
                    'user' => $this->getUsername(),
                ]);
            }
        }

        return $response->json('Done');
    }

    public function moveItems(Request $request, Response $response)
    {
        $items = $request->input('items', []);
        $destination = $request->input('destination', $this->separator);

        foreach ($items as $item) {
            $full_destination = trim($destination, $this->separator)
                    .$this->separator
                    .ltrim($item->name, $this->separator);
            $this->storage->move($item->path, $full_destination);

            // Trigger onMove hook for each item
            if ($this->hooks) {
                $this->hooks->trigger('onMove', [
                    'source_path' => $item->path,
                    'destination_path' => $full_destination,
                    'file_name' => $item->name,
                    'type' => $item->type,
                    'user' => $this->getUsername(),
                ]);
            }
        }

        return $response->json('Done');
    }

    public function zipItems(Request $request, Response $response, ArchiverInterface $archiver)
    {
        $items = $request->input('items', []);
        $destination = $request->input('destination', $this->separator);
        $name = $request->input('name', $this->config->get('frontend_config.default_archive_name'));

        $archiver->createArchive($this->storage);

        foreach ($items as $item) {
            if ($item->type == 'dir') {
                $archiver->addDirectoryFromStorage($item->path);
            }
            if ($item->type == 'file') {
                $archiver->addFileFromStorage($item->path);
            }
        }

        $archiver->storeArchive($destination, $name);

        return $response->json('Done');
    }

    public function unzipItem(Request $request, Response $response, ArchiverInterface $archiver)
    {
        $source = $request->input('item');
        $destination = $request->input('destination', $this->separator);

        $archiver->uncompress($source, $destination, $this->storage);

        return $response->json('Done');
    }
    
    public function chmodItems(Request $request, Response $response)
    {
        $items = $request->input('items', []);
        $permissions = $request->input('permissions', 0);
        /** @var null|'all'|'folders'|'files' */
        $recursive = $request->input('recursive', null);

        foreach ($items as $item) {
            $this->storage->chmod($item->path, $permissions, $recursive);
        }

        return $response->json('Done');
    }

    public function renameItem(Request $request, Response $response)
    {
        $destination = $request->input('destination', $this->separator);
        $from = $request->input('from');
        $to = $request->input('to');

        $this->storage->rename($destination, $from, $to);

        // Trigger onRename hook
        if ($this->hooks) {
            $newPath = trim($destination, $this->separator) . $this->separator . ltrim($to, $this->separator);
            $this->hooks->trigger('onRename', [
                'old_path' => $sourcePath,
                'new_path' => $newPath,
                'old_name' => $from,
                'new_name' => $to,
                'directory' => $destination,
                'user' => $this->getUsername(),
            ]);
        }

        return $response->json('Done');
    }

    public function deleteItems(Request $request, Response $response)
    {
        $items = $request->input('items', []);

        foreach ($items as $item) {
            if ($item->type == 'dir') {
                $this->storage->deleteDir($item->path);
            }
            if ($item->type == 'file') {
                $this->storage->deleteFile($item->path);
            }

            // Trigger onDelete hook for each item
            if ($this->hooks) {
                $this->hooks->trigger('onDelete', [
                    'file_path' => $item->path,
                    'file_name' => $item->name,
                    'type' => $item->type,
                    'user' => $this->getUsername(),
                ]);
            }
        }

        return $response->json('Done');
    }

    public function saveContent(Request $request, Response $response)
    {
        $path = $request->input('dir', $this->session->get(self::SESSION_CWD, $this->separator));

        $name = $request->input('name');
        $content = $request->input('content');

        $stream = tmpfile();
        fwrite($stream, $content);
        rewind($stream);

        $this->storage->deleteFile($path.$this->separator.$name);
        $this->storage->store($path, $name, $stream);

        if (is_resource($stream)) {
            fclose($stream);
        }

        return $response->json('Done');
    }
}
