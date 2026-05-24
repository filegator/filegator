<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Audit;

use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Service;

/**
 * Decides when to dispatch the weekly all-users audit digest and
 * builds the row data from the auth store.
 *
 * Deliberately in-app rather than driven by a system cron so the
 * deployment is self-contained: wherever the app runs, the digest
 * fires. The trade-off is that "fires" means "the next admin who
 * loads the user list once a week's worth of time has passed" —
 * if nobody opens the admin panel for two weeks, the digest waits
 * until they do. UAT explicitly accepted that trade-off.
 *
 * Concurrency: two admin requests landing simultaneously after the
 * interval elapses would both observe "due" if we only read the
 * state file. flock(LOCK_EX | LOCK_NB) on the same file makes the
 * check + timestamp update atomic — the second request gets
 * EWOULDBLOCK and skips this cycle silently. The send itself
 * happens *outside* the lock so a slow Postmark call cannot stall
 * a second concurrent admin request behind our file handle.
 *
 * Failure mode: we advance the timestamp *before* dispatching the
 * email. A transport failure logs and we wait another full
 * interval rather than retrying on every subsequent admin page
 * load (which, with Postmark down, would mean dozens of retry
 * attempts per day). Operators see the failure in app.log and
 * can manually re-trigger if needed.
 */
class WeeklyDigest implements Service
{
    const DEFAULT_INTERVAL_SECONDS = 604800; // 7 * 86400

    protected $audit;

    protected $logger;

    protected $stateFile;

    protected $intervalSeconds = self::DEFAULT_INTERVAL_SECONDS;

    public function __construct(AuditMailer $audit, LoggerInterface $logger)
    {
        $this->audit = $audit;
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        if (! empty($config['state_file'])) {
            $this->stateFile = (string) $config['state_file'];
        }
        if (isset($config['interval_seconds']) && (int) $config['interval_seconds'] > 0) {
            $this->intervalSeconds = (int) $config['interval_seconds'];
        }
    }

    /**
     * Cheap top-of-request check: returns immediately when the digest
     * is not due, the service is unconfigured, or another concurrent
     * request is already handling the send.
     */
    public function maybeFire(AuthInterface $auth): bool
    {
        if (! $this->stateFile) {
            return false;
        }

        // c+ opens for read+write and creates the file if missing without
        // truncating an existing one. b is for cross-platform consistency
        // on Windows even though we don't currently support it.
        $fh = @fopen($this->stateFile, 'c+b');
        if ($fh === false) {
            $this->logger->log('WeeklyDigest: cannot open state file '.$this->stateFile);
            return false;
        }

        if (! flock($fh, LOCK_EX | LOCK_NB)) {
            // Another request is mid-decision. Skip rather than wait — the
            // other request will either send or not, and either way the
            // user's HTTP path doesn't get blocked on a file lock.
            fclose($fh);
            return false;
        }

        try {
            $now = time();
            rewind($fh);
            $contents = stream_get_contents($fh);
            $state = ($contents !== false && $contents !== '') ? json_decode($contents, true) : [];
            if (! is_array($state)) $state = [];
            $last = (int) ($state['last_weekly_digest_at'] ?? 0);

            if ($last !== 0 && ($now - $last) < $this->intervalSeconds) {
                return false;
            }

            // Advance the timestamp now so a failed send doesn't loop-retry
            // on every subsequent admin request. Operators see failures in
            // the log and can manually re-trigger.
            $state['last_weekly_digest_at'] = $now;
            $encoded = json_encode($state, JSON_PRETTY_PRINT);
            ftruncate($fh, 0);
            rewind($fh);
            fwrite($fh, $encoded === false ? '{}' : $encoded);
            fflush($fh);
        } finally {
            flock($fh, LOCK_UN);
            fclose($fh);
        }

        // Build snapshot outside the lock. allUsers() may be a moderate
        // read in larger deployments; no reason to hold the lock for it.
        $rows = $this->buildRows($auth);
        $this->audit->sendWeeklyDigest($rows);

        return true;
    }

    /**
     * Read-only inspection. Returns the unix timestamp at which the next
     * digest is due, or 0 if no digest has fired yet (i.e. next call to
     * maybeFire() will fire immediately).
     */
    public function nextDueAt(): int
    {
        if (! $this->stateFile || ! is_file($this->stateFile)) return 0;
        $raw = @file_get_contents($this->stateFile);
        if ($raw === false || $raw === '') return 0;
        $state = json_decode($raw, true);
        $last = is_array($state) ? (int) ($state['last_weekly_digest_at'] ?? 0) : 0;
        return $last === 0 ? 0 : $last + $this->intervalSeconds;
    }

    protected function buildRows(AuthInterface $auth): array
    {
        $collection = $auth->allUsers();
        $meta = method_exists($auth, 'allUsersMeta') ? $auth->allUsersMeta() : [];

        $rows = [];
        foreach ($collection->all() as $u) {
            $username = $u->getUsername();
            // Guest is the built-in anonymous role with no real access; not
            // useful in the audit snapshot.
            if ($username === 'guest') continue;

            $rows[] = [
                'username' => $username,
                'name' => $u->getName(),
                'role' => $u->getRole(),
                // Both keys during the transition. AuditMailer prefers
                // `homedirs` and falls back to `homedir`; Phase 10 will
                // drop the scalar from this snapshot.
                'homedir' => $u->getHomeDir(),
                'homedirs' => $u->getHomeDirs(),
                'permissions' => $u->getPermissions(),
                'mfa_enabled' => (bool) ($meta[$username]['enabled'] ?? false),
                'email' => $meta[$username]['email'] ?? null,
            ];
        }

        // Stable alphabetical order so week-over-week diffs (eye scans)
        // line up cleanly.
        usort($rows, function ($a, $b) {
            return strcasecmp((string) $a['username'], (string) $b['username']);
        });

        return $rows;
    }
}
