<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Config\Config;
use Filegator\Services\Auth\MfaLockout;
use Filegator\Services\Tmpfs\Adapters\Tmpfs;
use Tests\TestCase;

/**
 * Per-IP + per-username MFA brute-force counters. Unit-tested directly so
 * we can exercise TTL expiry with a tiny lockout_timeout without paying
 * HTTP round-trip overhead (which made the equivalent feature test flaky).
 *
 * @internal
 */
class MfaLockoutTest extends TestCase
{
    protected $tmpfs;

    protected function setUp(): void
    {
        parent::setUp();
        $this->resetTempDir();

        $this->tmpfs = new Tmpfs();
        $this->tmpfs->init([
            'path' => TEST_TMP_PATH,
            // Disable opportunistic GC so we test our own age-based cleanup.
            'gc_probability_perc' => 0,
            'gc_older_than' => 60 * 60 * 24 * 2,
        ]);
    }

    protected function makeLockout(int $attempts, int $timeout): MfaLockout
    {
        $config = new Config(['lockout_attempts' => $attempts, 'lockout_timeout' => $timeout]);
        $lockout = new MfaLockout($this->tmpfs, $config);
        $lockout->init([]);
        return $lockout;
    }

    public function testTripsAfterAttemptsThresholdReached()
    {
        $lockout = $this->makeLockout(3, 60);

        $this->assertFalse($lockout->isLocked('1.1.1.1', 'alice'));
        $lockout->recordFailure('1.1.1.1', 'alice');
        $this->assertFalse($lockout->isLocked('1.1.1.1', 'alice'));
        $lockout->recordFailure('1.1.1.1', 'alice');
        $this->assertFalse($lockout->isLocked('1.1.1.1', 'alice'));
        $lockout->recordFailure('1.1.1.1', 'alice');
        // 3rd failure: counter == attempts → locked.
        $this->assertTrue($lockout->isLocked('1.1.1.1', 'alice'));
    }

    public function testPerUsernameLockoutSurvivesRotatingIps()
    {
        $lockout = $this->makeLockout(3, 60);

        // Each failure from a different IP — per-IP never trips.
        $lockout->recordFailure('1.1.1.1', 'alice');
        $lockout->recordFailure('2.2.2.2', 'alice');
        $lockout->recordFailure('3.3.3.3', 'alice');

        // From a fresh IP the per-username counter still locks.
        $this->assertTrue($lockout->isLocked('9.9.9.9', 'alice'));
    }

    public function testDifferentUsernameNotLockedOut()
    {
        $lockout = $this->makeLockout(2, 60);
        $lockout->recordFailure('1.1.1.1', 'alice');
        $lockout->recordFailure('2.2.2.2', 'alice');
        $this->assertTrue($lockout->isLocked('9.9.9.9', 'alice'));
        // bob is unaffected by alice's lockout.
        $this->assertFalse($lockout->isLocked('9.9.9.9', 'bob'));
    }

    public function testLockoutClearsAfterTimeoutExpires()
    {
        // 1-second timeout — viable here because no HTTP round-trips
        // separate the failures.
        $lockout = $this->makeLockout(2, 1);

        $lockout->recordFailure('1.1.1.1', 'alice');
        $lockout->recordFailure('2.2.2.2', 'alice');
        $this->assertTrue($lockout->isLocked('9.9.9.9', 'alice'));

        sleep(2);

        // Stale counters get GC'd on the next isLocked call.
        $this->assertFalse($lockout->isLocked('9.9.9.9', 'alice'));
    }

    public function testClearForUsernameRemovesOnlyUsernameCounter()
    {
        $lockout = $this->makeLockout(2, 60);
        $lockout->recordFailure('1.1.1.1', 'alice');
        $lockout->recordFailure('1.1.1.1', 'alice');
        $this->assertTrue($lockout->isLocked('1.1.1.1', 'alice'));

        $lockout->clearForUsername('alice');

        // Username counter cleared, but the per-IP counter is intact — an
        // attacker who guessed one right code on a hammering IP doesn't
        // get a fresh budget.
        $this->assertTrue($lockout->isLocked('1.1.1.1', 'alice'));
        // From a fresh IP though, alice is no longer locked.
        $this->assertFalse($lockout->isLocked('9.9.9.9', 'alice'));
    }
}
