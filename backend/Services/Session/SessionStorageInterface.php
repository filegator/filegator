<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Session;

interface SessionStorageInterface
{
    public function set(string $key, $data);

    public function get(string $key, $default = null);

    public function invalidate();

    public function save();
}
