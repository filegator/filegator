<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

use Filegator\Config\Config;
use Filegator\Services\Service;
use Filegator\Services\Tmpfs\TmpfsInterface;

/**
 * Per-IP and per-username brute-force counters for the MFA second step
 * and admin step-up auth. Both axes share `lockout_attempts` /
 * `lockout_timeout` config and the byte-count counter semantics of the
 * existing AuthController per-IP password lockout.
 *
 * Per-IP closes the obvious case (attacker hammers MFA from one box).
 * Per-username closes the rotating-IP variant (attacker proxies through
 * a botnet) — the lock travels with the account, not the source. The
 * tradeoff is intentional DoS-by-username: anyone who knows a victim's
 * username can lock that user out for `lockout_timeout` seconds. The
 * bound makes this acceptable; alternative is unbounded brute force.
 */
class MfaLockout implements Service
{
    protected $tmpfs;

    protected $config;

    public function __construct(TmpfsInterface $tmpfs, Config $config)
    {
        $this->tmpfs = $tmpfs;
        $this->config = $config;
    }

    public function init(array $config = []) {}

    public function isLocked(string $ip, string $username): bool
    {
        return $this->isCounterLocked($this->ipKey($ip))
            || $this->isCounterLocked($this->usernameKey($username));
    }

    public function recordFailure(string $ip, string $username): void
    {
        $this->tmpfs->write($this->ipKey($ip), 'x', true);
        $this->tmpfs->write($this->usernameKey($username), 'x', true);
    }

    /**
     * Reset the username counter on a legitimate success. We do not reset
     * the IP counter — that mirrors the existing per-IP password lockout
     * behaviour and prevents an attacker who guesses one valid TOTP from
     * resetting the brute-force budget for the IP they're hammering from.
     */
    public function clearForUsername(string $username): void
    {
        $key = $this->usernameKey($username);
        if ($this->tmpfs->exists($key)) {
            $this->tmpfs->remove($key);
        }
    }

    protected function isCounterLocked(string $lockfile): bool
    {
        $attempts = (int) $this->config->get('lockout_attempts', 5);
        $timeout = (int) $this->config->get('lockout_timeout', 15);

        foreach ($this->tmpfs->findAll($lockfile) as $flock) {
            if (time() - $flock['time'] >= $timeout) {
                $this->tmpfs->remove($flock['name']);
            }
        }

        return $this->tmpfs->exists($lockfile)
            && strlen($this->tmpfs->read($lockfile)) >= $attempts;
    }

    protected function ipKey(string $ip): string
    {
        return md5($ip).'.mfa.lock';
    }

    protected function usernameKey(string $username): string
    {
        return 'mfa_user_'.md5($username).'.lock';
    }
}
