<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\EventDispatcher\Events;

use Filegator\Services\EventDispatcher\Event;
use Filegator\Services\Auth\User;

/**
 * Event for authentication operations
 */
class AuthEvent extends Event
{
    // Event names for authentication operations
    public const BEFORE_LOGIN = 'auth.login.before';
    public const AFTER_LOGIN = 'auth.login.after';
    public const LOGIN_FAILED = 'auth.login.failed';
    public const BEFORE_LOGOUT = 'auth.logout.before';
    public const AFTER_LOGOUT = 'auth.logout.after';
    public const BEFORE_PASSWORD_CHANGE = 'auth.password_change.before';
    public const AFTER_PASSWORD_CHANGE = 'auth.password_change.after';
    public const USER_CREATED = 'auth.user.created';
    public const USER_UPDATED = 'auth.user.updated';
    public const USER_DELETED = 'auth.user.deleted';
    public const LOCKOUT = 'auth.lockout';

    /**
     * @var string|null The username
     */
    protected $username;

    /**
     * @var User|null The user object
     */
    protected $user;

    /**
     * @var string|null Client IP address
     */
    protected $ipAddress;

    /**
     * @var bool Whether authentication succeeded
     */
    protected $success = false;

    /**
     * @var string|null Failure reason
     */
    protected $failureReason;

    public function __construct(
        string $eventName,
        ?string $username = null,
        ?User $user = null,
        ?string $ipAddress = null,
        bool $success = false,
        ?string $failureReason = null,
        array $additionalData = []
    ) {
        parent::__construct($eventName, array_merge([
            'username' => $username,
            'ip_address' => $ipAddress,
            'success' => $success,
            'failure_reason' => $failureReason,
        ], $additionalData));

        $this->username = $username;
        $this->user = $user;
        $this->ipAddress = $ipAddress;
        $this->success = $success;
        $this->failureReason = $failureReason;
    }

    public function getUsername(): ?string
    {
        return $this->username;
    }

    public function setUsername(?string $username): self
    {
        $this->username = $username;
        $this->data['username'] = $username;
        return $this;
    }

    public function getUser(): ?User
    {
        return $this->user;
    }

    public function setUser(?User $user): self
    {
        $this->user = $user;
        return $this;
    }

    public function getIpAddress(): ?string
    {
        return $this->ipAddress;
    }

    public function setIpAddress(?string $ipAddress): self
    {
        $this->ipAddress = $ipAddress;
        $this->data['ip_address'] = $ipAddress;
        return $this;
    }

    public function isSuccess(): bool
    {
        return $this->success;
    }

    public function setSuccess(bool $success): self
    {
        $this->success = $success;
        $this->data['success'] = $success;
        return $this;
    }

    public function getFailureReason(): ?string
    {
        return $this->failureReason;
    }

    public function setFailureReason(?string $failureReason): self
    {
        $this->failureReason = $failureReason;
        $this->data['failure_reason'] = $failureReason;
        return $this;
    }
}
