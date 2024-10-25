<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Adriano Hänggli <https://github.com/ahaenggli>
 *
 */

namespace Filegator\Services\Auth\Adapters;

use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Auth\UsersCollection;
use Filegator\Services\Service;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Services\Logger\LoggerInterface;
use Monolog\Logger;

/**
 * @codeCoverageIgnore
 */
class LDAP implements Service, AuthInterface
{
    const SESSION_KEY = 'LDAP_auth';
    const GUEST_USERNAME = 'guest';

    protected $session;
    protected $private_repos = false;
    protected $ldap_server;
    protected $ldap_bindDN;
    protected $ldap_bindPass;
    protected $ldap_baseDN;
    protected $ldap_filter;
    protected $ldap_attributes;
    protected $ldap_userFieldMapping;
    protected $logger;

    public function __construct(Session $session, LoggerInterface $logger)
    {
        $this->session = $session;
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        if (!isset($config['ldap_server']) || empty($config['ldap_server']))
            throw new \Exception('config ldap_server missing');

        if (!extension_loaded('ldap'))
            throw new \Exception('ldap extension missing');

        if ($connect = ldap_connect($config['ldap_server'])) {
            @ldap_set_option($connect, LDAP_OPT_PROTOCOL_VERSION, 3);
            @ldap_set_option($connect, LDAP_OPT_REFERRALS, 0);

            $this->private_repos = $config['private_repos'];
            $this->ldap_server = $config['ldap_server'];
            $this->ldap_bindDN = $config['ldap_bindDN'];
            $this->ldap_bindPass = $config['ldap_bindPass'];
            $this->ldap_baseDN = $config['ldap_baseDN'];
            $this->ldap_filter = $config['ldap_filter'];
            $this->ldap_attributes = isset($config['ldap_attributes']) ? $config['ldap_attributes'] : ['*'];
            $this->ldap_userFieldMapping = $config['ldap_userFieldMapping'];
        } else {
            @ldap_close($connect);
            throw new \Exception('could not connect to domain');
        }

        @ldap_close($connect);
    }

    public function user(): ?User
    {
        return $this->session ? $this->session->get(self::SESSION_KEY, null) : null;
    }

    public function authenticate($username, $password): bool
    {
        // prevent anonymous binding
        if (!isset($password) || empty($password))
            return false;
        if (!isset($username) || empty($username))
            return false;

        // remove (optional) domains from the username
        if (!empty($this->ldap_userFieldMapping['username_RemoveDomains']) && is_array($this->ldap_userFieldMapping['username_RemoveDomains'])) {
            $username = str_replace($this->ldap_userFieldMapping['username_RemoveDomains'], '', $username);
        }

        $all_users = $this->getUsers($username);

        // add the domain to the username
        if (!empty($this->ldap_userFieldMapping['username_AddDomain'])) {
            if (strpos($username, $this->ldap_userFieldMapping['username_AddDomain']) === false) {
                $username .= $this->ldap_userFieldMapping['username_AddDomain'];
            }
        }

        foreach ($all_users as &$u) {
            if (strtolower($u['username']) == strtolower($username) && $this->verifyPassword($u['userDN'], $password)) {
                $user = $this->mapToUserObject($u);
                $this->store($user);
                return true;
            }
        }

        return false;
    }

    public function forget()
    {
        return $this->session->invalidate();
    }

    public function store(User $user)
    {
        return $this->session->set(self::SESSION_KEY, $user);
    }

    public function update($username, User $user, $password = ''): User
    {
        return new User(); // not used
    }

    public function add(User $user, $password): User
    {
        return new User(); // not used
    }

    public function delete(User $user)
    {
        return true; // not used
    }

    public function find($username): ?User
    {
        foreach ($this->getUsers($username) as $user) {
            if (strtolower($user['username']) == strtolower($username)) {
                return $this->mapToUserObject($user);
            }
        }

        return null;
    }

    public function getGuest(): User
    {
        $guest = $this->find(self::GUEST_USERNAME);

        if (!$guest || !$guest->isGuest()) {
            $guest = new User();
            $guest->setUsername('guest');
            $guest->setName('Guest');
            $guest->setRole('guest');
            $guest->setHomedir('/');
            $guest->setPermissions([]);
            return $guest;
        }

        return $guest;
    }

    public function allUsers(): UsersCollection
    {
        $users = new UsersCollection();

        foreach ($this->getUsers() as $user) {
            $users->addUser($this->mapToUserObject($user));
        }

        return $users;
    }

    protected function mapToUserObject(array $user): User
    {
        $new = new User();
        $new->setUsername($user['username']);
        $new->setName($user['name']);
        $new->setRole($user['role']);
        $new->setHomedir($user['homedir']);
        $new->setPermissions($user['permissions'], true);
        return $new;
    }

    protected function getUsers(string $username = null): array
    {
        $ldapConn = @ldap_connect($this->ldap_server);
        if (!$ldapConn)
            throw new \Exception('Cannot Connect to LDAP server');
        @ldap_set_option($ldapConn, LDAP_OPT_PROTOCOL_VERSION, 3);
        @ldap_set_option($ldapConn, LDAP_OPT_REFERRALS, 0);

        $ldapBind = @ldap_bind($ldapConn, $this->ldap_bindDN, $this->ldap_bindPass);
        if (!$ldapBind)
            throw new \Exception('Cannot Bind to LDAP server: Wrong credentials?');

        // search the LDAP server for users
        $filter = $this->ldap_filter;
        if (!empty($username))
            $filter = '(&' . $filter . '(' . $this->ldap_userFieldMapping['username'] . '=' . $username . '))';

        $ldapSearch = @ldap_search($ldapConn, $this->ldap_baseDN, $filter, $this->ldap_attributes);

        if (!$ldapSearch) {
            $this->logger->log($filter);
            throw new \Exception('Cannot search LDAP server: Wrong filter?');
        }

        $ldapResults = @ldap_get_entries($ldapConn, $ldapSearch);
        @ldap_close($ldapConn);

        $users = [];

        for ($item = 0; $item < $ldapResults['count']; $item++) {

            $missingAttributes = [];

            // Check if all required attributes are present
            foreach (['username', 'name', 'userDN'] as $attribute) {
                if (!isset($ldapResults[$item][$this->ldap_userFieldMapping[$attribute]])) {
                    $missingAttributes[] = '`' . $this->ldap_userFieldMapping[$attribute] . '` as `' . $attribute . '`';
                }
            }
            // if any required attribute is missing, log an info message
            if (!empty($missingAttributes)) {
                $this->logger->log('Missing LDAP attribues: ' . implode(', ', $missingAttributes) . '. Please check the spelling (including upper or lower case).', Logger::WARNING);
            } else {
                $user = [];
                $user['username'] = $ldapResults[$item][$this->ldap_userFieldMapping['username']][0];
                $user['name'] = $ldapResults[$item][$this->ldap_userFieldMapping['name']][0];
                $user['role'] = 'user';
                $user['homedir'] = '/';
                $user['permissions'] = $this->ldap_userFieldMapping['default_permissions'];
                $user['userDN'] = $ldapResults[$item][$this->ldap_userFieldMapping['userDN']];

                if (!empty($this->ldap_userFieldMapping['username_AddDomain'])) {
                    if (strpos($user['username'], $this->ldap_userFieldMapping['username_AddDomain']) === false)
                        $user['username'] .= $this->ldap_userFieldMapping['username_AddDomain'];
                }

                if (is_array($this->ldap_userFieldMapping['admin_usernames'])) {
                    if (in_array($user['username'], $this->ldap_userFieldMapping['admin_usernames']))
                        $user['role'] = 'admin';
                }

                // private repositories for each user?
                if ($this->private_repos) {
                    $user['homedir'] = '/' . $user['username'];
                }

                // ...but not for admins
                if ($user['role'] == 'admin') {
                    $user['homedir'] = '/';
                    $user['permissions'] = 'read|write|upload|download|batchdownload|zip|chmod';
                }

                if (is_array($user) && !empty($user))
                    $users[] = $user;
            }
        }
        // print_r($users); // uncomment this line to see all available ldap-login-users
        return is_array($users) ? $users : [];
    }

    private function verifyPassword($auth_user, $password)
    {
        if (!isset($this->ldap_server) || empty($this->ldap_server))
            return false;
        if (!extension_loaded('ldap'))
            return false;
        $connect = @ldap_connect($this->ldap_server);
        if ($connect) {
            ldap_set_option($connect, LDAP_OPT_PROTOCOL_VERSION, 3);
            ldap_set_option($connect, LDAP_OPT_REFERRALS, 0);
            $bind = @ldap_bind($connect, $auth_user, $password);
            if ($bind) {
                @ldap_close($connect);
                return true;
            } else {
                @ldap_close($connect);
                return false;
            }
        }

        @ldap_close($connect);
        return false;
    }
}
