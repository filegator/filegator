<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests;

use Filegator\Services\Auth\Adapters\JsonFile;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Service;

class MockUsers extends JsonFile implements Service, AuthInterface
{
    private $users_array = [];

    public function init(array $config = [])
    {
        $this->addMockUsers();
    }

    protected function getUsers(): array
    {
        return $this->users_array;
    }

    protected function saveUsers(array $users)
    {
        return $this->users_array = $users;
    }

    public function user(): ?User
    {
        return $this->session ? $this->session->get(self::SESSION_KEY, null) : null;
    }


    private function addMockUsers()
    {
        $guest = new User();
        $guest->setRole('guest');
        $guest->setHomedir('/');
        $guest->setUsername('guest');
        $guest->setName('Guest');
        $guest->setPermissions([]);

        $admin = new User();
        $admin->setRole('admin');
        $admin->setHomedir('/');
        $admin->setUsername('admin@example.com');
        $admin->setName('Admin');
        $admin->setPermissions(['read', 'write', 'upload', 'download', 'batchdownload', 'zip']);

        $john = new User();
        $john->setRole('user');
        $john->setHomedir('/john');
        $john->setUsername('john@example.com');
        $john->setName('John Doe');
        $john->setPermissions(['read', 'write', 'upload', 'download', 'batchdownload']);

        $jane = new User();
        $jane->setRole('user');
        $jane->setHomedir('/jane');
        $jane->setUsername('jane@example.com');
        $jane->setName('Jane Doe');
        $jane->setPermissions(['read', 'write']);

        // A second regular user with its own home directory and the permissions
        // required to reach the batch-download endpoints. Used to prove that one
        // user cannot download another user's batch archive (GHSA-f74m-x83r-c4v4).
        $jack = new User();
        $jack->setRole('user');
        $jack->setHomedir('/jack');
        $jack->setUsername('jack@example.com');
        $jack->setName('Jack Doe');
        $jack->setPermissions(['read', 'write', 'download', 'batchdownload']);

        // Two distinct users whose usernames collapse to the same value under a
        // [^0-9a-zA-Z_] sanitiser (both become "bobsmithexamplecom"), used to
        // prove the upload tmpfs namespace stays isolated across such a pair.
        $bob = new User();
        $bob->setRole('user');
        $bob->setHomedir('/bob');
        $bob->setUsername('bobsmith@example.com');
        $bob->setName('Bob Smith');
        $bob->setPermissions(['read', 'write', 'upload', 'download']);

        $bob2 = new User();
        $bob2->setRole('user');
        $bob2->setHomedir('/bob2');
        $bob2->setUsername('bob.smith@example.com');
        $bob2->setName('Bob Smith II');
        $bob2->setPermissions(['read', 'write', 'upload', 'download']);

        $this->add($guest, '');
        $this->add($admin, 'admin123');
        $this->add($john, 'john123');
        $this->add($jane, 'jane123');
        $this->add($jack, 'jack123');
        $this->add($bob, 'bob123');
        $this->add($bob2, 'bob2123');
    }

}
