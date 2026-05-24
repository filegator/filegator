<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Tests\TestCase;

/**
 * @internal
 */
class AdminTest extends TestCase
{
    public function testOnlyAdminCanPerformUserActions()
    {
        $this->signOut();

        $this->sendRequest('GET', '/listusers');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/storeuser');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/updateuser/test@example.com');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/deleteuser/test@example.com');
        $this->assertStatus(404);
    }

    public function testListUsers()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                [
                    'role' => 'guest',
                    'permissions' => [],
                    'homedir' => '/',
                    'username' => 'guest',
                    'name' => 'Guest',
                ],
                [
                    'role' => 'admin',
                    'permissions' => [],
                    'homedir' => '/',
                    'username' => 'admin@example.com',
                    'name' => 'Admin',
                ],
                [
                    'role' => 'user',
                    'permissions' => [],
                    'homedir' => '/john',
                    'username' => 'john@example.com',
                    'name' => 'John Doe',
                ],
                [
                    'role' => 'user',
                    'permissions' => [],
                    'homedir' => '/jane',
                    'username' => 'jane@example.com',
                    'name' => 'Jane Doe',
                ],
            ],
        ]);
    }

    public function testAddingNewUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'mike@example.com',
            'role' => 'user',
            'permissions' => [],
            'password' => 'pass123',
            'homedir' => '/john',
        ]);
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'role' => 'user',
                'permissions' => [],
                'homedir' => '/john',
                'username' => 'mike@example.com',
                'name' => 'Mike Test',
            ],
        ]);
    }

    public function testAddingNewUserValidation()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => '',
            'username' => '',
            'role' => 'user',
            'permissions' => [],
            'password' => 'pass123',
            'homedir' => '',
        ]);
        $this->assertStatus(422);

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'mike@example.com',
            'role' => 'bear',
            'permissions' => ['xxx'],
            'password' => 'pass123',
            'homedir' => '/john',
        ]);
        $this->assertStatus(422);
    }

    public function testUpdatingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'Johnny Doe',
            'username' => 'john2@example.com',
            'role' => 'admin',
            'permissions' => ['read', 'write'],
            'homedir' => '/jane',
        ]);
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'role' => 'admin',
                'permissions' => ['read', 'write'],
                'homedir' => '/jane',
                'username' => 'john2@example.com',
                'name' => 'Johnny Doe',
            ],
        ]);
    }

    public function testDeletingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/john@example.com');
        $this->assertOk();
    }

    public function testUpdatingNonExistingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/nonexisting@example.com');
        $this->assertStatus(422);
    }

    public function testUpdatingUserValidation()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => '',
            'username' => '',
            'homedir' => '',
        ]);
        $this->assertStatus(422);

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'something',
            'username' => 'something',
            'homedir' => '/',
            'permissions' => ['xxx', 'write'],
        ]);
        $this->assertStatus(422);
    }

    public function testDeletingNonExistingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/nonexisting@example.com');
        $this->assertStatus(422);
    }

    public function testAddingOrEditingUserWithUsernameThatIsAlreadyTaken()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'admin@example.com',
            'role' => 'user',
            'password' => '123',
            'permissions' => [],
            'homedir' => '/mike',
        ]);

        $this->assertStatus(422);

        $this->sendRequest('POST', '/updateuser/admin@example.com', [
            'name' => 'Admin',
            'username' => 'john@example.com',
            'role' => 'admin',
            'permissions' => [],
            'homedir' => '/',
        ]);

        $this->assertStatus(422);
    }

    // --------------------------------------------------------------------
    // Pins for the existing admin-input boundary behaviour. The multi-folder
    // refactor preserves these contracts; the tests guard the seams.
    // --------------------------------------------------------------------

    public function testStoreUserHomedirIsAdminPrefixJoined()
    {
        // Pin the exact admin-prefix join behaviour in storeUser
        // (AdminController.php ~line 87-91): the supplied homedir is
        // rtrim'd/ltrim'd and concatenated under the admin's homedir, with
        // NO `..` normalization at create time. Runtime safety for users
        // whose homedir contains `..` comes from Filesystem::applyPathPrefix
        // sandboxing every storage operation — not from this step. We pin
        // the existing string-concatenation shape so the multi-folder
        // refactor (which loops the same join over each homedirs[] element)
        // doesn't accidentally change the result.
        $this->signIn('admin@example.com', 'admin123');

        // Admin's homedir is '/'. Supplied homedir 'subdir' should land as
        // '/subdir'. Supplied homedir '/subdir' should also land as '/subdir'.
        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'Alpha',
            'username'    => 'alpha@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedir'     => 'alpha',
        ]);
        $this->assertOk();

        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'Beta',
            'username'    => 'beta@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedir'     => '/beta',
        ]);
        $this->assertOk();

        $this->sendRequest('GET', '/listusers');
        $rows = json_decode($this->response->getContent(), true)['data'];
        $byName = [];
        foreach ($rows as $u) {
            $byName[$u['username']] = $u['homedir'];
        }

        $this->assertSame('/alpha', $byName['alpha@example.com'] ?? null);
        $this->assertSame('/beta', $byName['beta@example.com'] ?? null);
    }

    public function testUpdateUserHomedirCanBeAnyString()
    {
        // updateUser does NOT apply the admin-prefix join — it accepts the
        // homedir field as-is. Pin this asymmetry so the multi-folder
        // refactor preserves it (storeUser joins, updateUser does not).
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedir'     => '/relocated/explicit',
        ]);
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'username' => 'john@example.com',
                'homedir'  => '/relocated/explicit',
            ],
        ]);
    }

    public function testListUsersShapeIncludesHomedirField()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->assertOk();

        $rows = json_decode($this->response->getContent(), true)['data'];
        $this->assertNotEmpty($rows);
        foreach ($rows as $u) {
            $this->assertArrayHasKey('homedir', $u, 'listUsers row missing homedir key');
            $this->assertArrayHasKey('homedirs', $u, 'listUsers row missing homedirs key (Phase 2)');
            $this->assertArrayHasKey('username', $u);
            $this->assertArrayHasKey('role', $u);
            $this->assertArrayHasKey('name', $u);
            $this->assertArrayHasKey('permissions', $u);
        }
    }

    // --------------------------------------------------------------------
    // Phase 4: storeUser/updateUser accept `homedirs[]` while preserving
    // back-compat for the legacy `homedir` scalar payload shape.
    // --------------------------------------------------------------------

    public function testStoreUserAcceptsHomedirsArray()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'Multi Maker',
            'username'    => 'mm@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedirs'    => ['mfolderA', '/mfolderB'],
        ]);
        $this->assertOk();

        $this->sendRequest('GET', '/listusers');
        $rows = json_decode($this->response->getContent(), true)['data'];
        $mm = null;
        foreach ($rows as $u) {
            if ($u['username'] === 'mm@example.com') $mm = $u;
        }
        $this->assertNotNull($mm);
        // Admin is at '/' — join is /<each>. Both shapes ('plain' and
        // '/leading-slash') normalise to /plain and /leading-slash.
        $this->assertSame(['/mfolderA', '/mfolderB'], $mm['homedirs']);
        // Back-compat scalar key is the first element.
        $this->assertSame('/mfolderA', $mm['homedir']);
    }

    public function testStoreUserAcceptsLegacyHomedirString()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Legacy frontend payload — single string. Must still work through
        // Phase 10.
        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'Legacy',
            'username'    => 'legacy@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedir'     => '/legacy',
        ]);
        $this->assertOk();

        $this->sendRequest('GET', '/listusers');
        $rows = json_decode($this->response->getContent(), true)['data'];
        $legacy = null;
        foreach ($rows as $u) {
            if ($u['username'] === 'legacy@example.com') $legacy = $u;
        }
        $this->assertNotNull($legacy);
        $this->assertSame(['/legacy'], $legacy['homedirs']);
    }

    public function testStoreUserRejectsEmptyHomedirs()
    {
        $this->signIn('admin@example.com', 'admin123');

        // Empty array
        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'No Folders',
            'username'    => 'nf@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedirs'    => [],
        ]);
        $this->assertStatus(422);

        // Blank-only strings
        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'No Folders',
            'username'    => 'nf@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedirs'    => ['', '   ', '  '],
        ]);
        $this->assertStatus(422);

        // Missing both keys entirely
        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'No Folders',
            'username'    => 'nf@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
        ]);
        $this->assertStatus(422);
    }

    public function testStoreUserAdminPrefixJoinAppliesToEachElement()
    {
        // Pin: each element of homedirs gets the same admin-prefix join
        // that the pre-refactor scalar storeUser did to its single input.
        // Admin is at '/' in the test fixture; supplied 'x' becomes '/x'.
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name'        => 'JoinTest',
            'username'    => 'jt@example.com',
            'role'        => 'user',
            'permissions' => [],
            'password'    => 'pass123',
            'homedirs'    => ['a', 'b', 'c'],
        ]);
        $this->assertOk();

        $this->sendRequest('GET', '/listusers');
        $rows = json_decode($this->response->getContent(), true)['data'];
        foreach ($rows as $u) {
            if ($u['username'] === 'jt@example.com') {
                $this->assertSame(['/a', '/b', '/c'], $u['homedirs']);
                return;
            }
        }
        $this->fail('jt@example.com not found in listUsers');
    }

    public function testUpdateUserHomedirsArrayPath()
    {
        // Move john from single-folder to multi-folder via updateUser.
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedirs'    => ['/jext1', '/jext2'],
        ]);
        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'username' => 'john@example.com',
                'homedirs' => ['/jext1', '/jext2'],
                // back-compat scalar = first element
                'homedir'  => '/jext1',
            ],
        ]);
    }

    public function testUpdateUserNoPrefixJoin()
    {
        // Asymmetry pin: updateUser stores the supplied value verbatim,
        // unlike storeUser which prefixes with the admin's homedir.
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name'        => 'John Doe',
            'username'    => 'john@example.com',
            'role'        => 'user',
            'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload'],
            'homedirs'    => ['raw-no-leading-slash'],
        ]);
        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'homedirs' => ['raw-no-leading-slash'],
            ],
        ]);
    }
}
