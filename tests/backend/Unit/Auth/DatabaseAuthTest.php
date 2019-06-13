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
use Filegator\Services\Auth\Adapters\Database;
use Filegator\Services\Session\Adapters\SessionStorage;

/**
 * @internal
 */
class DatabaseAuthTest extends AuthTest
{
    protected $conn;

    public function setAuth()
    {
        $session = new SessionStorage(new Request());
        $session->init([
            'session_handler' => 'mockfilesession',
            'available' => [
                'mockfilesession' => function () {
                    return new \Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage();
                },
            ],
        ]);

        $this->auth = new Database($session);
        $this->auth->init([
            'driver' => 'pdo',
            'dsn' => 'sqlite::memory:',
            'database' => 'users',
        ]);

        $this->conn = $this->auth->getConnection();

        $this->conn->query('DROP TABLE IF EXISTS [users]');
        $this->conn->query('CREATE TABLE [users] (
                [id] INTEGER PRIMARY KEY NOT NULL,
                [username] VARCHAR(255) NOT NULL,
                [name] VARCHAR(255) NOT NULL,
                [role] VARCHAR(20) NOT NULL,
                [permissions] VARCHAR(100) NOT NULL,
                [homedir] VARCHAR(1000) NOT NULL,
                [password] VARCHAR(255) NOT NULL

            )');
        $ret = $this->conn->fetch('SELECT * FROM users WHERE username = ?', 'admin');
    }
}
