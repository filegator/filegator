<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

use Filegator\Utils\Collection;

class UsersCollection implements \JsonSerializable
{
    use Collection;

    public function addUser(User $user)
    {
        $this->add($user);
    }
}
