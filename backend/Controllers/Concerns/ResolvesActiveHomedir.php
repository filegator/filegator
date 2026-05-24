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

        // Re-read the current user's record from the auth adapter so we
        // see any in-flight admin edit to their homedirs.
        $fresh = $this->auth->find($effective->getUsername());
        $homedirs = $fresh ? $fresh->getHomeDirs() : $effective->getHomeDirs();

        if (empty($homedirs)) {
            $response->json('Account has no folders configured', 422);
            return false;
        }

        $active = $this->session->get(FileController::SESSION_ACTIVE_HOMEDIR, null);

        // Auto-seed for single-folder users every request — covers fresh
        // sessions, post-admin-edit drifts, and any reseting we missed.
        if (count($homedirs) === 1) {
            $active = $homedirs[0];
            $this->session->set(FileController::SESSION_ACTIVE_HOMEDIR, $active);
        }

        if ($active === null || ! in_array($active, $homedirs, true)) {
            $response->json('No folder selected', 422);
            return false;
        }

        $this->storage->setPathPrefix($active);
        return true;
    }
}
