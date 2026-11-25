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
        $user->setHomeDir($homedir);
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

    public function testUsernameToHomeDirUsesSafeUsernames()
    {
        foreach (['alice', 'alice..', '..alice', 'john@example.com', 'jürgen'] as $username) {
            $this->assertEquals('/' . $username, User::usernameToHomeDir($username, false));
        }
    }

    public function testUsernameToHomeDirRejectsUnsafeUsernames()
    {
        foreach (['', '.', '..', '...', 'a/b', 'a\\b', "a\x00b", 'a:b', 'a|b', "\xff"] as $username) {
            $this->assertNull(User::usernameToHomeDir($username, false), "'{$username}' should be rejected");
        }
    }

    public function testUsernameToHomeDirSecuredIsBase32()
    {
        // RFC 4648 test vectors, lowercase and unpadded
        foreach (['f' => 'my', 'fo' => 'mzxq', 'foo' => 'mzxw6', 'foob' => 'mzxw6yq', 'fooba' => 'mzxw6ytb', 'foobar' => 'mzxw6ytboi'] as $username => $result) {
            $this->assertEquals('/' . $result, User::usernameToHomeDir($username, true));
        }
        $this->assertNull(User::usernameToHomeDir('', true));
    }

    public function testUsernameToHomeDirSecuredAcceptsAnyUsernameWithoutCollisions()
    {
        $usernames = ['alice', 'alice..', 'Alice', '.', '..', 'a/b', 'a\\b', "\xff"];
        $homedirs = array_map(function ($username) {
            return User::usernameToHomeDir($username, true);
        }, $usernames);

        foreach ($homedirs as $homedir) {
            $this->assertRegExp('#^/[a-z2-7]+$#', $homedir);
        }
        $this->assertCount(count($usernames), array_unique($homedirs));
    }
}
