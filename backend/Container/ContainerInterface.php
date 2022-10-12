<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Container;

interface ContainerInterface
{
    public function get($name);

    public function set(string $name, $value);

    public function call($callable, array $parameters = []);
}
