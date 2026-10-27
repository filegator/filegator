/**
 * The account menu.
 *
 * Everything about you as a person lives here: the theme, your password, and
 * signing out. Administration is not in it — Setup stays a button of its own,
 * because it is a place to go rather than a preference.
 *
 * The item list is plain data (`userMenuItems`), so which entries appear for a
 * guest or when an administrator has switched password changes off can be
 * checked without a DOM.
 */

import { create } from "./dom.js";
import { t } from "./i18n.js";
import { DARK } from "./theme.js";

/**
 * Returns the menu entries for a session, in display order.
 *
 * options:
 *   user           - the signed-in user, or null before one is known
 *   canChangePassword - the provider and the administrator both allow it
 *   canLogout      - security.allow_logout
 *   canPickLanguage - frontend_config.language_selection, and more than one
 *                     language available
 *   theme          - the theme in effect now ("light" or "dark")
 *
 * A guest gets neither password nor logout: there is no password, and its
 * "log in" opens a form rather than ending a session, so it is listed under
 * its own id and label instead of pretending to be a logout.
 */
export function userMenuItems({ user, canChangePassword, canLogout, canPickLanguage, theme }) {
  const isGuest = Boolean(user && user.guest);
  const items = [];

  // The label names the theme a click produces, which is what the toolbar
  // button did: the icon shows the destination, not the current state.
  items.push({
    id: "theme",
    label: theme === DARK ? "Light mode" : "Dark mode",
    icon: theme === DARK ? "☀️" : "🌙",
  });

  // A guest has no password of its own to change.
  if (!isGuest && canChangePassword !== false) {
    items.push({ id: "password", label: "Change password", icon: "🔑" });
  }

  // The language belongs to everyone, a guest included: a visitor reading the
  // login form in a language they do not know has nowhere else to change it.
  // Absent when the administrator locked the interface to one language.
  if (canPickLanguage) {
    items.push({ id: "language", label: "Language", icon: "🌐" });
  }

  if (isGuest) {
    items.push({ id: "login", label: "Log in", icon: "→" });
  } else if (canLogout !== false) {
    items.push({ id: "logout", label: "Logout", icon: "⎋" });
  }

  return items;
}

/**
 * Builds the menu in `element` and wires the button that opens it.
 *
 * options:
 *   button   - the header button that owns the menu
 *   element  - the menu container (role="menu")
 *   items    - () => entries, read on every open so the theme label is current
 *   onChoose - (id) => void
 *
 * Returns { refresh } to rebuild the open menu after a change, and { close }.
 * The menu closes on Escape, on a click outside, and after a choice — the same
 * rules as the row menu.
 */
export function createUserMenu({ button, element, items, onChoose }) {
  let isOpen = false;

  function close() {
    if (!isOpen) {
      return;
    }
    isOpen = false;
    element.classList.add("hidden");
    button.setAttribute("aria-expanded", "false");
    document.removeEventListener("click", onDocument, true);
    document.removeEventListener("keydown", onKey, true);
  }

  function render() {
    element.replaceChildren();
    for (const item of items()) {
      const entry = create("button", {
        className: `user-menu-item${item.danger ? " is-danger" : ""}`,
        attrs: { type: "button", role: "menuitem", "data-action": item.id },
      }, [
        create("span", { className: "user-menu-icon", "aria-hidden": "true", text: item.icon || "" }),
        create("span", { text: t(item.label) }),
      ]);
      entry.addEventListener("click", (event) => {
        event.preventDefault();
        event.stopPropagation();
        close();
        onChoose(item.id);
      });
      element.append(entry);
    }
  }

  function open() {
    if (isOpen) {
      close();
      return;
    }
    render();
    // Anchored under the button, right-aligned like the ⋯ menu of a row, and
    // placed in the page rather than in the header so the card's overflow
    // cannot clip it.
    element.classList.remove("hidden");
    const box = button.getBoundingClientRect();
    const width = element.offsetWidth || 200;
    const height = element.offsetHeight || 0;
    const margin = 8;
    const viewWidth = window.innerWidth || document.documentElement.clientWidth || 1024;
    let left = box.right - width;
    let top = box.bottom + 4;
    if (top + height > (window.innerHeight || 800) - margin) {
      top = Math.max(margin, box.top - height - 4);
    }
    left = Math.max(margin, Math.min(left, viewWidth - width - margin));
    element.style.left = `${Math.round(left)}px`;
    element.style.top = `${Math.round(top)}px`;

    isOpen = true;
    button.setAttribute("aria-expanded", "true");
    document.addEventListener("click", onDocument, true);
    document.addEventListener("keydown", onKey, true);
    const first = element.querySelector(".user-menu-item");
    if (first) {
      first.focus();
    }
  }

  function onDocument(event) {
    if (!element.contains(event.target) && !button.contains(event.target)) {
      close();
    }
  }

  function onKey(event) {
    if (event.key === "Escape") {
      close();
      button.focus();
      return;
    }
    // Arrow keys walk the items, as a menu is expected to. Home and End jump
    // to the ends.
    const entries = [...element.querySelectorAll(".user-menu-item")];
    if (!entries.length) {
      return;
    }
    const at = entries.indexOf(document.activeElement);
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      const next = (at + step + entries.length) % entries.length;
      entries[next].focus();
    } else if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      entries[event.key === "Home" ? 0 : entries.length - 1].focus();
    }
  }

  button.addEventListener("click", (event) => {
    event.preventDefault();
    open();
  });

  return {
    close,
    /** Rebuilds the open menu, e.g. after the theme item was chosen. */
    refresh() {
      if (isOpen) {
        render();
      }
    },
    isOpen: () => isOpen,
  };
}
