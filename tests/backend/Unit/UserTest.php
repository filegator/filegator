<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Services\Auth\User;
use Tests\TestCase;
use Exception;

/**
 * @internal
 */
class UserTest extends TestCase
{
    public function testUser()
    {
        $name = 'John Doe';
        $username = 'john.doe@example.com';
        $homedir = '/';
        $role = 'user';
        $permissions = ['read'];

        $user = new User($role, $homedir, $username, $name);
        $user->setRole($role);
        $user->setHomedir($homedir);
        $user->setUsername($username);
        $user->setName($name);
        $user->setPermissions($permissions);

        $this->assertTrue($user->isUser());

        $decoded = json_decode(json_encode($user));
        $this->assertEquals('user', $decoded->role);
        $this->assertEquals($username, $decoded->username);
        $this->assertEquals($name, $decoded->name);
        $this->assertEquals($permissions, $decoded->permissions);
    }

    public function testAdmin()
    {
        $user = new User();
        $user->setRole('admin');

        $this->assertTrue($user->isAdmin());
    }

    public function testGuest()
    {
        $user = new User();

        $this->assertTrue($user->isGuest());

        $decoded = json_decode(json_encode($user), true);
        $this->assertEquals([
            'role' => 'guest',
            'homedir' => '',
            'username' => '',
            'name' => '',
            'permissions' => [],
        ], $decoded);
    }

    public function testUserCannotGetNonExistingRole()
    {
        $user = new User();

        $this->expectException(Exception::class);

        $user->setRole('nonexistent');
    }

    public function testUserCannotGetNonExistingPermision()
    {
        $user = new User();

        $this->expectException(Exception::class);

        $user->setPermissions(['read', 'write', 'nonexistent']);
    }

    public function testUserHasRole()
    {
        $user = new User();
        $user->setRole('user');

        $this->assertTrue($user->hasRole('user'));
        $this->assertTrue($user->hasRole(['user']));
        $this->assertTrue($user->hasRole(['admin', 'guest', 'user']));

        $this->assertFalse($user->hasRole(['admin', 'guest']));
        $this->assertFalse($user->hasRole('admin'));
    }

    public function testDefaultUserHasNoPermissions()
    {
        $user = new User();

        $this->assertTrue($user->hasPermissions([]));
        $this->assertFalse($user->hasPermissions('read'));
        $this->assertFalse($user->hasPermissions(['read']));
        $this->assertFalse($user->hasPermissions(['write', 'upload', 'read']));
        $this->assertFalse($user->hasPermissions(['write', 'upload']));
        $this->assertFalse($user->hasPermissions('upload'));
    }

    public function testUserHasOnlyPermissionsSet()
    {
        $user = new User();
        $user->setPermissions(['read', 'write']);

        $this->assertTrue($user->hasPermissions('read'));
        $this->assertTrue($user->hasPermissions(['read']));
        $this->assertTrue($user->hasPermissions('write'));
        $this->assertTrue($user->hasPermissions(['write']));
        $this->assertTrue($user->hasPermissions(['read', 'write']));
        $this->assertTrue($user->hasPermissions(['write', 'read']));
        $this->assertFalse($user->hasPermissions(['write', 'upload', 'read']));
        $this->assertFalse($user->hasPermissions(['write', 'upload']));
        $this->assertFalse($user->hasPermissions('upload'));
    }

    public function testUserCanHaveReadPermissions()
    {
        $user = new User();
        $user->setPermissions(['read']);

        $this->assertTrue($user->hasPermissions('read'));
    }

    public function testUserCanHaveWritePermissions()
    {
        $user = new User();
        $user->setPermissions(['write']);

        $this->assertTrue($user->hasPermissions('write'));
    }

    public function testUserCanHaveUploadPermissions()
    {
        $user = new User();
        $user->setPermissions(['upload']);

        $this->assertTrue($user->hasPermissions('upload'));
    }

    public function testUserCanHaveDownloadPermissions()
    {
        $user = new User();
        $user->setPermissions(['download']);

        $this->assertTrue($user->hasPermissions('download'));
    }

    public function testUserCanHaveBatchDownloadPermissions()
    {
        $user = new User();
        $user->setPermissions(['batchdownload']);

        $this->assertTrue($user->hasPermissions('batchdownload'));
    }

    public function testUserCanHaveZipPermissions()
    {
        $user = new User();
        $user->setPermissions(['zip']);

        $this->assertTrue($user->hasPermissions('zip'));
    }
}
