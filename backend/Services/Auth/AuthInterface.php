<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

interface AuthInterface
{
    public function user(): ?User;

    public function authenticate($username, $password): bool;

    public function forget();

    public function find($username): ?User;

    public function store(User $user);

    public function update($username, User $user, $password = ''): User;

    public function add(User $user, $password): User;

    public function delete(User $user);

    public function getGuest(): User;

    public function allUsers(): UsersCollection;
}
