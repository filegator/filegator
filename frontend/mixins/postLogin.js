import api from '../api/api'

/**
 * True when a user must be routed to the picker before file operations
 * can proceed. Shared by main.js bootstrap, router.beforeEach, and
 * SelectFolder.vue's defensive mount guard so the three sites can't drift.
 */
export function needsFolderPicker(user) {
  if (!user) return false
  const homedirs = Array.isArray(user.homedirs) ? user.homedirs : []
  return homedirs.length > 1 && !user.active_homedir
}

/**
 * Decide where to route the user immediately after a successful login
 * (or after the bootstrap `getUser` fetch on page load).
 *
 *   homedirs.length === 0 (guest) — go to `/`. Guest is its own auth
 *   path with a single built-in homedir; the picker is never relevant.
 *
 *   homedirs.length === 1 — go straight into the file browser. The
 *   backend auto-seeded SESSION_ACTIVE_HOMEDIR at login. The response
 *   payload's active_homedir confirms it; only fall back to a defensive
 *   selectFolder() when the payload doesn't already match.
 *
 *   homedirs.length > 1 — if the server already seeded an active_homedir
 *   drop them in the browser; otherwise route to the picker.
 */
export function routeAfterLogin(user, router, store) {
  const homedirs = (user && Array.isArray(user.homedirs)) ? user.homedirs : []
  const active = user && user.active_homedir ? user.active_homedir : null

  if (homedirs.length === 0) {
    router.push('/').catch(() => {})
    return
  }

  if (homedirs.length === 1) {
    const only = homedirs[0]
    if (active === only) {
      // Server already knows; no need for a round-trip on every page load.
      router.push('/').catch(() => {})
      return
    }
    // Bootstrap path didn't seed (or session expired). Fire-and-forget;
    // ensureActiveHomedir on the backend will also auto-seed if this fails.
    api.selectFolder({ homedir: only })
      .then(() => {
        if (store) store.commit('setActiveHomedir', only)
      })
      .catch(() => {})
      .finally(() => {
        router.push('/').catch(() => {})
      })
    return
  }

  // Multi-folder: route to picker unless the active selection is still valid.
  if (active && homedirs.indexOf(active) !== -1) {
    router.push('/').catch(() => {})
  } else {
    router.push('/select-folder').catch(() => {})
  }
}
