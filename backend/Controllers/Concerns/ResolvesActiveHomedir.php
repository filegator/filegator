<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Controllers\Concerns;

use Filegator\Controllers\FileController;
use Filegator\Kernel\Response;

/**
 * Resolves the user's "active homedir" — the folder a multi-folder user
 * has currently selected, or the only homedir for a single-folder user.
 *
 * Used by FileController / UploadController / DownloadController. The
 * trait must be applied to a controller class that has $this->auth (an
 * AuthInterface), $this->session (a SessionStorageInterface), and
 * $this->storage (a Filesystem). The constructor of those controllers
 * stores those refs but does NOT call setPathPrefix — every public method
 * calls ensureActiveHomedir() as its first line, which both validates
 * the selection and applies the prefix lazily.
 *
 * The lazy resolution is deliberate: a constructor-time setPathPrefix
 * would force us to throw from __construct when a multi-folder user
 * hasn't selected a folder yet, which surfaces as a 500. The
 * ensure-then-act pattern lets us return a clean 422 instead.
 */
trait ResolvesActiveHomedir
{
    /**
     * Validate and apply the user's active homedir to the storage prefix.
     * Returns true on success, false if the response has already been
     * populated with an error (422 / 404) — caller should `return;`.
     *
     * Lookup order:
     * 1. Single-folder users get auto-seeded — their only homedir wins.
     * 2. Multi-folder users with a valid SESSION_ACTIVE_HOMEDIR proceed.
     * 3. Anything else (no session entry, stale entry pointing at a
     *    homedir the user no longer has) becomes a 422.
     *
     * Live homedirs come from $this->auth->find($username) — NOT from the
     * cached $this->auth->user() — so an admin removing a user's folder
     * mid-session is honoured on the very next request.
     */
    protected function ensureActiveHomedir(Response $response): bool
    {
        $current = $this->auth->user();
        $guest = $current ? null : $this->auth->getGuest();
        $effective = $current ?: $guest;

        if (! $effective) {
            $response->json('Not authenticated', 401);
            return false;
        }

        // Guests are routed through their built-in homedir without picker
        // logic — guest auth flows aren't impacted by the multi-folder
        // refactor.
        if ($effective->isGuest()) {
            $this->storage->setPathPrefix($effective->getHomeDir());
            return true;
        }

        // $effective came from auth->user(), which verifies the session
        // hash against the live row. A mid-session admin edit to homedirs
        // changes the hash → user() returns null → effective falls back
        // to guest. So the cached User's homedirs are guaranteed to match
        // the live row whenever we reach this line, no extra find() needed.
        $homedirs = $effective->getHomeDirs();

        if (empty($homedirs)) {
            $response->json('Account has no folders configured', 422);
            return false;
        }

        $active = $this->session->get(FileController::SESSION_ACTIVE_HOMEDIR, null);

        // Auto-seed for single-folder users — covers fresh sessions and
        // any reset we missed. Skip the session write when it's already
        // correct to avoid a no-op session-storage hit on every request.
        if (count($homedirs) === 1) {
            $only = $homedirs[0];
            if ($active !== $only) {
                $this->session->set(FileController::SESSION_ACTIVE_HOMEDIR, $only);
            }
            $active = $only;
        }

        if ($active === null || ! in_array($active, $homedirs, true)) {
            $response->json('No folder selected', 422);
            return false;
        }

        $this->storage->setPathPrefix($active);
        return true;
    }
}
