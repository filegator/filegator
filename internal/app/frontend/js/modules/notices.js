/**
 * Instance-wide notices.
 *
 * Rendered above the header on every screen so an administrator cannot miss a
 * condition that only the server log would otherwise show. The data comes from
 * an admin-only endpoint, so non-administrators simply see nothing.
 */

import { api } from "./api.js";
import { clear, create, el, hide, show } from "./dom.js";
import { t } from "./i18n.js";

/** Builds the notice list from a status response. */
export function noticesFor(status) {
  const items = [];
  if (status.default_admin_password) {
    items.push({
      kind: "warning",
      // Named so the notice bar can attach the "Change password" button to
      // this row and no other, without comparing English sentences.
      key: "default_admin_password",
      text: t("The admin account still uses the default password. Change it now."),
    });
  }
  if (status.config_read_only) {
    items.push({
      kind: "warning",
      text: t("The configuration file is read-only. Changes are not saved after a restart."),
    });
  }
  if (status.users_file_read_only) {
    items.push({
      kind: "error",
      text: t("The users file is read-only. User changes cannot be saved."),
    });
  }
  return items;
}

/**
 * Builds the notices for what this start created or could not resolve.
 *
 * These come from /api/config rather than the admin status endpoint because
 * they are about the running process, not about the instance's settings, and
 * they are written for the operator: plain sentences naming the paths involved,
 * in the language the server logged them in.
 */
export function startupNotices(notes) {
  const items = [];
  for (const note of notes || []) {
    if (!note || !note.text) {
      continue;
    }
    items.push({ kind: note.kind === "warning" ? "warning" : "info", text: note.text });
  }
  return items;
}

/**
 * Creates the global notice bar.
 *
 * options:
 *   onChangePassword() - opens the password form for the current admin
 *   startup()          - notices about what this start of the server created,
 *                        read from the public configuration
 */
export function createNotices({ onChangePassword, startup, user: initialUser }) {
  const bar = el("global-notices");
  let current = [];
  // Kept so refresh() can tell whether asking is worthwhile at all.
  let user = initialUser || null;
  // What the server created on this start. Shown to administrators only: a
  // regular user can do nothing with "created private/users.json".
  const startNotes = typeof startup === "function" ? startup() : startup || [];

  function render() {
    clear(bar);
    if (!current.length) {
      hide(bar);
      document.body.classList.remove("has-notices");
      return;
    }
    for (const item of current) {
      const row = create("div", { className: `global-notice is-${item.kind}` }, [
        create("span", { className: "global-notice-text", text: item.text }),
      ]);
      if (item.action) {
        const button = create("button", {
          className: "global-notice-action",
          text: item.action.label,
          attrs: { type: "button" },
        });
        button.addEventListener("click", item.action.run);
        row.append(button);
      }
      bar.append(row);
    }
    show(bar);
    document.body.classList.add("has-notices");
  }

  return {
    /**
     * Fetches the status and re-renders.
     *
     * Only called for administrators. The guard is repeated here because the
     * endpoint is admin-only: a stray call from elsewhere would put a 403 in
     * the console for no display benefit.
     */
    async refresh() {
      if (!user || user.role !== "admin") {
        current = [];
        render();
        return;
      }
      let status;
      try {
        status = await api.adminStatus();
      } catch {
        // 403 for regular users; 401 before login. Nothing to show.
        current = [];
        render();
        return;
      }
      // Start-up notices come first: they describe the state of the server,
      // while the rest are settings the administrator can change here.
      current = [...startupNotices(startNotes), ...noticesFor(status)];
      if (status.default_admin_password && onChangePassword) {
        const password = current.find((n) => n.key === "default_admin_password");
        if (password) {
          password.action = { label: t("Change password"), run: onChangePassword };
        }
      }
      render();
    },

    /** Hides the bar, for example after logging out. */
    clear() {
      current = [];
      render();
    },

    /** Records who is signed in, since notices are administrator-only. */
    setUser(next) {
      user = next || null;
    },
  };
}
