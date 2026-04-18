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
use Filegator\Services\Hooks\HooksInterface;
use Filegator\Services\PathACL\PathACLInterface;
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

        $this->separator = $this->storage->getSeparator();
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

    public function changeDirectory(Request $request, Response $response)
    {
        $path = $request->input('to', $this->separator);

        // Check PathACL permission for the target directory
        if (!$this->checkPathACL($request, $path, 'read')) {
            return $this->forbidden($response);
        }

        $this->session->set(self::SESSION_CWD, $path);

        $content = $this->storage->getDirectoryCollection($path);

        // Filter out items the user cannot access and add path permissions
        $content = $this->filterDirectoryByACL($request, $content, $path);

        return $response->json($content);
    }

    public function getDirectory(Request $request, Response $response)
    {
        $path = $request->input('dir', $this->session->get(self::SESSION_CWD, $this->separator));

        // Check PathACL permission
        if (!$this->checkPathACL($request, $path, 'read')) {
            return $this->forbidden($response);
        }

        $content = $this->storage->getDirectoryCollection($path);

        // Filter out items the user cannot access and add path permissions
        $content = $this->filterDirectoryByACL($request, $content, $path);

        return $response->json($content);
    }

    /**
     * Filter directory collection to remove items user cannot access
     *
     * @param Request $request HTTP request object
     * @param \Filegator\Services\Storage\DirectoryCollection $collection Directory collection
     * @param string $currentPath Current directory path (for getting path permissions)
     * @return \Filegator\Services\Storage\DirectoryCollection Filtered collection
     */
    protected function filterDirectoryByACL(Request $request, $collection, string $currentPath = '/')
    {
        // Debug: Log PathACL status
        error_log("[PathACL DEBUG] FileController::filterDirectoryByACL - pathacl injected: " . ($this->pathacl ? 'YES' : 'NO'));
        error_log("[PathACL DEBUG] FileController::filterDirectoryByACL - pathacl enabled: " . ($this->pathacl && $this->pathacl->isEnabled() ? 'YES' : 'NO'));

        // If PathACL is not enabled, return unfiltered
        if (!$this->pathacl || !$this->pathacl->isEnabled()) {
            error_log("[PathACL DEBUG] FileController::filterDirectoryByACL - Returning UNFILTERED collection (PathACL disabled or not injected)");
            return $collection;
        }

        $user = $this->auth->user() ?: $this->auth->getGuest();
        $clientIp = $request->getClientIp();

        error_log("[PathACL DEBUG] FileController::filterDirectoryByACL - user: " . $user->getUsername() . ", clientIp: " . $clientIp);

        // Get effective permissions for the current directory and include in response
        $effectivePermissions = $this->pathacl->getEffectivePermissions($user, $clientIp, $currentPath);
        $collection->setPathPermissions($effectivePermissions);
        error_log("[PathACL DEBUG] Path permissions for '{$currentPath}': " . json_encode($effectivePermissions));

        // Filter items based on read permission
        $collection->filter(function ($item) use ($user, $clientIp) {
            // Always allow the "back" navigation item (..)
            if ($item['type'] === 'back') {
                return true;
            }

            // Check if user can read this path
            return $this->pathacl->checkPermission($user, $clientIp, $item['path'], 'read');
        });

        return $collection;
    }

    public function createNew(Request $request, Response $response)
    {
        $type = $request->input('type', 'file');
        $name = $request->input('name');
        $path = $this->session->get(self::SESSION_CWD, $this->separator);

        // Check PathACL permission
        if (!$this->checkPathACL($request, $path, 'write')) {
            return $this->forbidden($response);
        }

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

        // Check PathACL permission for destination
        if (!$this->checkPathACL($request, $destination, 'write')) {
            return $this->forbidden($response);
        }

        foreach ($items as $item) {
            // Check PathACL permission for each source item
            if (!$this->checkPathACL($request, $item->path, 'read')) {
                return $this->forbidden($response);
            }

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

        // Check PathACL permission for destination
        if (!$this->checkPathACL($request, $destination, 'write')) {
            return $this->forbidden($response);
        }

        foreach ($items as $item) {
            // Check PathACL permission for each source item (need write to move/delete)
            if (!$this->checkPathACL($request, $item->path, 'write')) {
                return $this->forbidden($response);
            }

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

        // Check PathACL permission for destination
        if (!$this->checkPathACL($request, $destination, 'zip')) {
            return $this->forbidden($response);
        }

        $archiver->createArchive($this->storage);

        foreach ($items as $item) {
            // Check PathACL permission for each item to be zipped
            if (!$this->checkPathACL($request, $item->path, 'read')) {
                return $this->forbidden($response);
            }

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

        // Check PathACL permission for source (need read)
        if (!$this->checkPathACL($request, $source, 'read')) {
            return $this->forbidden($response);
        }

        // Check PathACL permission for destination (need write)
        if (!$this->checkPathACL($request, $destination, 'write')) {
            return $this->forbidden($response);
        }

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
            // Check PathACL permission for chmod operation
            if (!$this->checkPathACL($request, $item->path, 'chmod')) {
                return $this->forbidden($response);
            }

            $this->storage->chmod($item->path, $permissions, $recursive);
        }

        return $response->json('Done');
    }

    public function renameItem(Request $request, Response $response)
    {
        $destination = $request->input('destination', $this->separator);
        $from = $request->input('from');
        $to = $request->input('to');

        // Build full source path
        $sourcePath = trim($destination, $this->separator) . $this->separator . ltrim($from, $this->separator);

        // Check PathACL permission for rename (need write)
        if (!$this->checkPathACL($request, $sourcePath, 'write')) {
            return $this->forbidden($response);
        }

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
            // Check PathACL permission for delete (need write)
            if (!$this->checkPathACL($request, $item->path, 'write')) {
                return $this->forbidden($response);
            }

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

        // Build full file path
        $filePath = $path . $this->separator . $name;

        // Check PathACL permission for write
        if (!$this->checkPathACL($request, $filePath, 'write')) {
            return $this->forbidden($response);
        }

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
