<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Auth;

use Exception;
use Filegator\Kernel\Request;
use Filegator\Services\Auth\Adapters\LDAP;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Session\Adapters\SessionStorage;
use Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage;
use Tests\TestCase;

/**
 * Regression tests for GHSA-4r6c-gx7v-584w (session fixation + stale session
 * authorization in the LDAP adapter).
 *
 * The adapter reaches the real directory only in getUsers()/verifyPassword(),
 * so each test injects those two seams with an in-memory directory. Everything
 * under test - session id regeneration on login, per-request revalidation, the
 * login-lookup replay and the directory-outage fallback - is the real adapter
 * code, and the suite runs in CI with no LDAP server.
 *
 * @internal
 */
class LdapSessionTest extends TestCase
{
    private $request;

    private $session;

    protected function setUp(): void
    {
        $this->request = new Request();
        $this->session = new SessionStorage($this->request);
        $this->session->init([
            'handler' => function () {
                return new MockFileSessionStorage();
            },
        ]);

        parent::setUp();
    }

    /**
     * Build an LDAP adapter whose directory access is faked.
     *
     * @param callable $getUsers fn(?string $username): array
     * @param array    $mapping  ldap_userFieldMapping (RemoveDomains/AddDomain)
     */
    private function makeAuth(callable $getUsers, array $mapping = []): LDAP
    {
        return new class ($this->session, $this->nullLogger(), $getUsers, $mapping) extends LDAP {
            private $getUsersFn;

            public function __construct($session, $logger, callable $getUsersFn, array $mapping)
            {
                parent::__construct($session, $logger);
                $this->getUsersFn = $getUsersFn;
                $this->ldap_userFieldMapping = $mapping;
            }

            protected function getUsers(string $username = null): array
            {
                return ($this->getUsersFn)($username);
            }

            protected function verifyPassword($auth_user, $password)
            {
                return $password === 'correct-horse';
            }
        };
    }

    private function nullLogger(): LoggerInterface
    {
        return new class implements LoggerInterface {
            public function log(string $message, int $level = 0)
            {
            }
        };
    }

    private function alice(array $overrides = []): array
    {
        return array_merge([
            'username' => 'alice',
            'name' => 'Alice',
            'role' => 'user',
            'homedir' => '/alice',
            'permissions' => 'read|download',
            'userDN' => 'uid=alice,ou=people,dc=example,dc=com',
        ], $overrides);
    }

    private function sessionId()
    {
        return $this->request->getSession()->getId();
    }

    public function testSessionIdIsRegeneratedOnLogin()
    {
        $auth = $this->makeAuth(function () {
            return [$this->alice()];
        });

        // an attacker-fixed session id is in place before the victim logs in
        $this->session->set('anything', 1);
        $fixed_id = $this->sessionId();
        $this->assertNotEmpty($fixed_id);

        $this->assertTrue($auth->authenticate('alice', 'correct-horse'));

        $this->assertNotEquals(
            $fixed_id,
            $this->sessionId(),
            'session id must change on login to prevent session fixation'
        );
    }

    public function testAuthenticatedUserIsReturnedWhileDirectoryIsUnchanged()
    {
        $auth = $this->makeAuth(function () {
            return [$this->alice()];
        });

        $this->assertTrue($auth->authenticate('alice', 'correct-horse'));

        $user = $auth->user();
        $this->assertNotNull($user);
        $this->assertEquals('alice', $user->getUsername());
    }

    public function testDowngradedAccountLosesItsLiveSession()
    {
        // the shared closure lets a later request observe a changed directory;
        // a fresh adapter instance models that next request over the same session
        $permissions = 'read|write|download';
        $directory = function () use (&$permissions) {
            return [$this->alice(['permissions' => $permissions])];
        };

        $login = $this->makeAuth($directory);
        $this->assertTrue($login->authenticate('alice', 'correct-horse'));
        $this->assertNotNull($login->user());

        // the account is downgraded in the directory after login
        $permissions = 'read';

        $next = $this->makeAuth($directory);
        $this->assertNull(
            $next->user(),
            'a changed account must invalidate the existing session'
        );
    }

    public function testChangedHomedirLosesItsLiveSession()
    {
        $homedir = '/alice';
        $directory = function () use (&$homedir) {
            return [$this->alice(['homedir' => $homedir])];
        };

        $login = $this->makeAuth($directory);
        $this->assertTrue($login->authenticate('alice', 'correct-horse'));
        $this->assertNotNull($login->user());

        // the account is re-homed after login
        $homedir = '/';

        $next = $this->makeAuth($directory);
        $this->assertNull(
            $next->user(),
            'a changed home directory must invalidate the existing session'
        );
    }

    public function testRemovedAccountLosesItsLiveSession()
    {
        $present = true;
        $directory = function () use (&$present) {
            return $present ? [$this->alice()] : [];
        };

        $login = $this->makeAuth($directory);
        $this->assertTrue($login->authenticate('alice', 'correct-horse'));
        $this->assertNotNull($login->user());

        // the account is removed from the directory after login
        $present = false;

        $next = $this->makeAuth($directory);
        $this->assertNull(
            $next->user(),
            'a removed account must invalidate the existing session'
        );
    }

    public function testRevalidationReplaysTheLoginLookupKey()
    {
        // Faithful fake: the directory attribute is the bare uid "alice"; the
        // adapter appends username_AddDomain. getUsers() honours its argument
        // and only matches the bare uid, so if user() searched with the
        // domain-carrying stored username the lookup would miss and the user
        // would be wrongly logged out. This makes the SESSION_LOOKUP replay
        // load-bearing rather than tautological.
        $getUsers = function (?string $username) {
            if ($username !== null && strtolower($username) !== 'alice') {
                return [];
            }

            return [$this->alice(['username' => 'alice@example.com'])];
        };

        $auth = $this->makeAuth($getUsers, ['username_AddDomain' => '@example.com']);

        $this->assertTrue($auth->authenticate('alice', 'correct-horse'));

        $user = $auth->user();
        $this->assertNotNull($user, 'a valid AddDomain user must stay logged in across requests');
        $this->assertEquals('alice@example.com', $user->getUsername());
    }

    public function testDirectoryOutageKeepsTheExistingSession()
    {
        // getUsers() succeeds at login, then the directory becomes unreachable
        $down = false;
        $auth = $this->makeAuth(function () use (&$down) {
            if ($down) {
                throw new Exception('Cannot Connect to LDAP server');
            }

            return [$this->alice()];
        });

        $this->assertTrue($auth->authenticate('alice', 'correct-horse'));

        $down = true;

        $user = $auth->user();
        $this->assertNotNull(
            $user,
            'a transient directory outage must not log the user out or throw'
        );
        $this->assertEquals('alice', $user->getUsername());
    }

    public function testReloginReplacesThePreviousUser()
    {
        // the per-request memoisation must not serve a stale user across a re-login
        $current = 'alice';
        $auth = $this->makeAuth(function () use (&$current) {
            return [$this->alice([
                'username' => $current,
                'userDN' => 'uid='.$current.',ou=people,dc=example,dc=com',
            ])];
        });

        $this->assertTrue($auth->authenticate('alice', 'correct-horse'));
        $this->assertEquals('alice', $auth->user()->getUsername());

        // a different user logs in on the same adapter instance
        $current = 'bob';
        $this->assertTrue($auth->authenticate('bob', 'correct-horse'));

        $this->assertEquals(
            'bob',
            $auth->user()->getUsername(),
            're-login must invalidate the per-request user cache'
        );
    }
}
