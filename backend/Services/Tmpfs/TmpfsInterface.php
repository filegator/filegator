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

    /**
     * Atomically create $filename only if it does not already exist.
     * Returns true if THIS call created the file, false if it was already there.
     * The check-and-create is a single O_EXCL filesystem operation, so two
     * concurrent callers cannot both observe absence and both create — used
     * for SETNX-style replay markers.
     */
    public function addIfAbsent(string $filename, string $data = '1'): bool;

    /**
     * Atomically read the current count from $filename (treating each byte as
     * one increment), and if it is below $max, append one byte and return the
     * new count. Otherwise return -1 to signal "limit exceeded; nothing
     * written". flock(LOCK_EX) is held across the read/write so two callers
     * cannot both observe a sub-max value and both append.
     */
    public function incrementCounterIfBelow(string $filename, int $max): int;
}
