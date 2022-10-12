<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Auth;

use Filegator\Services\Auth\Adapters\JsonFile;

/**
 * @internal
 */
class JsonFileTest extends AuthTest
{
    private $mock_file = TEST_DIR.'/mockusers.json';

    protected function tearDown(): void
    {
        @unlink($this->mock_file);
        @unlink($this->mock_file.'.blank');
    }

    public function setAuth()
    {
        @unlink($this->mock_file);
        @touch($this->mock_file.'.blank');

        $this->auth = new JsonFile($this->session);
        $this->auth->init([
            'file' => $this->mock_file,
        ]);
    }
}
