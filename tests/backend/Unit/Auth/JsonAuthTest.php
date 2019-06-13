<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Auth;

use Filegator\Kernel\Request;
use Filegator\Services\Auth\Adapters\JsonFile;
use Filegator\Services\Session\Adapters\SessionStorage;

/**
 * @internal
 */
class JsonFileTest extends AuthTest
{
    private $mock_file = TEST_DIR.'/mockusers.json';

    public function tearDown(): void
    {
        @unlink($this->mock_file);
        @unlink($this->mock_file.'.blank');
    }

    public function setAuth()
    {
        @unlink($this->mock_file);
        @touch($this->mock_file.'.blank');

        $session = new SessionStorage(new Request());
        $session->init([
            'session_handler' => 'mockfilesession',
            'available' => [
                'mockfilesession' => function () {
                    return new \Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage();
                },
            ],
        ]);

        $this->auth = new JsonFile($session);
        $this->auth->init([
            'file' => $this->mock_file,
        ]);
    }
}
