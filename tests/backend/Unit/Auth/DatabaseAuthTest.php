<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Auth;

use Filegator\Services\Auth\Adapters\Database;

/**
 * @internal
 */
class DatabaseAuthTest extends AuthTest
{
    protected $conn;

    public function setAuth()
    {
        $this->auth = new Database($this->session);
        $this->auth->init([
            'driver' => 'sqlite',
            'dsn' => 'sqlite::memory:',
            'database' => 'tests/backend/tmp/temp/sqlite',
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
        $this->conn->fetch('SELECT * FROM users WHERE username = ?', 'admin');
    }
}
