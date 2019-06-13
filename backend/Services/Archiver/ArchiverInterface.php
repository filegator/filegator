<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Archiver;

use Filegator\Services\Storage\Filesystem;

interface ArchiverInterface
{
    public function createArchive(Filesystem $storage): string;

    public function uncompress(string $source, string $destination, Filesystem $storage);

    public function addDirectoryFromStorage(string $path);

    public function addFileFromStorage(string $path);

    public function closeArchive();

    public function storeArchive($destination, $name);
}
