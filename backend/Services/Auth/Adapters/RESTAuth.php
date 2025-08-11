<?php

namespace Filegator\Services\Auth\Adapters;

use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Auth\UsersCollection;
use Filegator\Services\Service;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Services\Logger\LoggerInterface;

class RESTAuth implements Service, AuthInterface
{
    const SESSION_KEY = 'rest_auth';
    const SESSION_USER_DATA = 'rest_auth_user_data';

    protected $session;
    protected $config;
    protected $logger;

    public function __construct(Session $session, LoggerInterface $logger)
    {
        $this->session = $session;
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        $this->config = $config;
    }

    public function user(): ?User
    {
        if (!$this->session) return null;

        return $this->session->get(self::SESSION_KEY, null);
    }

    public function authenticate($username, $password): bool
    {
        $url = $this->config['url'];
        
        $this->logger->log("RESTAuth: Attempting authentication for user: $username");

        $data = [
            'username' => $username,
            'password' => $password
        ];

        $options = [
            'http' => [
                'method' => 'POST',
                'header' => 'Content-Type: application/json',
                'content' => json_encode($data),
                'ignore_errors' => true,
                'timeout' => 10
            ]
        ];

        $context = stream_context_create($options);
        $response = file_get_contents($url, false, $context);

        $this->logger->log("RESTAuth: API response received: " . ($response ? 'SUCCESS' : 'FAILED'));

        if ($response === false) {
            $this->logger->log("RESTAuth: Connection to $url failed");
            return false;
        }

        $result = json_decode($response, true);

        if (!$result || !isset($result['success']) || !$result['success']) {
            $this->logger->log("RESTAuth: Authentication failed for $username - " . json_encode($result));
            return false;
        }

        $this->logger->log("RESTAuth: Authentication successful for $username");
        
        $user = $this->createUser($result['user']);
        $this->store($user);
        $this->session->set(self::SESSION_USER_DATA, $result['user']);

        return true;
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
        // Not needed for external auth - just return the user
        return $user;
    }

    public function add(User $user, $password): User
    {
        // Not needed for external auth
        return $user;
    }

    public function delete(User $user)
    {
        // Not needed for external auth
        return true;
    }

    public function find($username): ?User
    {
        // Could implement API call to get user info if needed
        return null;
    }

    public function getGuest(): User
    {
        $guest = new User();
        $guest->setUsername('guest');
        $guest->setName('Guest');
        $guest->setRole('guest');
        $guest->setHomedir('/');
        $guest->setPermissions([]);

        return $guest;
    }

    public function allUsers(): UsersCollection
    {
        // Return empty collection - not needed for external auth
        return new UsersCollection();
    }

    protected function createUser(array $data): User
    {
        $user = new User();
        $user->setUsername($data['username']);
        $user->setName($data['name'] ?? $data['username']);
        $user->setRole($data['role'] ?? 'user');
        $user->setHomedir($data['homedir'] ?? '/');
        $user->setPermissions($data['permissions'] ?? 'read|write|upload|download|create', true);

        return $user;
    }
}