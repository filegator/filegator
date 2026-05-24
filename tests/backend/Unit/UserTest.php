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
            'homedirs' => [],
            'username' => '',
            'name' => '',
            'permissions' => [],
        ], $decoded);
    }

    public function testJsonSerializeKeysAreStable()
    {
        // The exact key set returned by jsonSerialize. Phase 2 added
        // `homedirs` (array) as a sibling of `homedir` (scalar). Phase 10
        // will drop `homedir`. Updating this assertion is the explicit
        // signal that a key was added or removed — guard against unintended
        // changes.
        $user = new User();
        $user->setRole('user');
        $user->setUsername('john.doe@example.com');
        $user->setName('John Doe');
        $user->setHomedirs(['/john']);
        $user->setPermissions(['read', 'write']);

        $decoded = json_decode(json_encode($user), true);
        $expected = ['role', 'homedir', 'homedirs', 'username', 'name', 'permissions'];
        sort($expected);
        $actualKeys = array_keys($decoded);
        sort($actualKeys);

        $this->assertEquals($expected, $actualKeys, 'jsonSerialize key set changed unexpectedly');
    }

    public function testGetHomeDirReturnsStringSet()
    {
        // Legacy single-string contract — setHomedir/getHomeDir is now a
        // shim over setHomedirs/getHomeDirs. The shim must preserve the
        // string round-trip for any code path that hasn't migrated yet.
        $user = new User();
        $user->setHomedir('/some/path');

        $this->assertSame('/some/path', $user->getHomeDir());
        $this->assertIsString($user->getHomeDir());
    }

    public function testSetHomedirRoutesThroughSetHomedirs()
    {
        // Calling setHomedir wraps the value as a single-element homedirs
        // array, so getHomeDirs() and getHomeDir() agree.
        $user = new User();
        $user->setHomedir('/single');

        $this->assertSame(['/single'], $user->getHomeDirs());
        $this->assertSame('/single', $user->getHomeDir());
    }

    public function testSetHomedirsNormalisesEntries()
    {
        // Trim, drop empties + non-strings, re-index. Pin so the
        // normalisation can't accidentally drift to allow blanks through.
        $user = new User();
        $user->setHomedirs([' /a ', '', '/b', '   ', '/c', 42]);

        $this->assertSame(['/a', '/b', '/c'], $user->getHomeDirs());
    }

    public function testGetHomeDirReturnsFirstElement()
    {
        // Back-compat shim semantics: first element of the array.
        $user = new User();
        $user->setHomedirs(['/first', '/second']);

        $this->assertSame('/first', $user->getHomeDir());
    }

    public function testGetHomeDirReturnsEmptyStringWhenNoFolders()
    {
        $user = new User();

        $this->assertSame('', $user->getHomeDir());
        $this->assertSame([], $user->getHomeDirs());
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
