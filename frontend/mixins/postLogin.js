import api from '../api/api'

/**
 * Decide where to route the user immediately after a successful login
 * (or after the bootstrap `getUser` fetch on page load).
 *
 *   homedirs.length === 0 (guest) — go to `/`. Guest is its own auth
 *   path with a single built-in homedir; the picker is never relevant.
 *
 *   homedirs.length === 1 — go straight into the file browser. The
 *   backend will have already auto-seeded SESSION_ACTIVE_HOMEDIR via
 *   seedActiveHomedirAfterLogin, so file-op requests work immediately.
 *   We also fire a defensive api.selectFolder() — harmless if the
 *   server already picked, useful when the user landed via the
 *   bootstrap getUser path (which doesn't seed).
 *
 *   homedirs.length > 1 — if the server already seeded an
 *   active_homedir (e.g. they previously picked one and the session
 *   survived) drop them in the browser; otherwise route to the picker.
 */
export function routeAfterLogin(user, router, store) {
  const homedirs = (user && Array.isArray(user.homedirs)) ? user.homedirs : []
  const active = user && user.active_homedir ? user.active_homedir : null

  if (homedirs.length === 0) {
    // Guest or unauthenticated.
    router.push('/').catch(() => {})
    return
  }

  if (homedirs.length === 1) {
    const only = homedirs[0]
    // Defensive — server may not have seeded for the bootstrap path.
    api.selectFolder({ homedir: only })
      .then(() => {
        if (store) store.commit('setActiveHomedir', only)
      })
      .catch(() => {
        // If the call fails the file-op endpoints will still auto-seed
        // for single-folder users via ensureActiveHomedir, so this is
        // truly best-effort.
      })
      .finally(() => {
        router.push('/').catch(() => {})
      })
    return
  }

  // Multi-folder: pick first, route to picker, or skip if already active.
  if (active && homedirs.indexOf(active) !== -1) {
    router.push('/').catch(() => {})
  } else {
    router.push('/select-folder').catch(() => {})
  }
}
