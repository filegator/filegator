<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Tmpfs;

interface TmpfsInterface
{
    public function exists(string $filename): bool;

    public function findAll($pattern): array;

    public function write(string $filename, $data, $append);

    public function read(string $filename): string;

    public function readStream(string $filename): array;

    public function remove(string $filename);

    public function getFileLocation(string $filename): string;

    public function clean(int $older_than);
}
