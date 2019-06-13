<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests;

use Filegator\Kernel\Response;

class FakeResponse extends Response
{
    public function send()
    {
        // do nothing
    }
}
