/**
 * A per-row "⋯" menu.
 *
 * v7 put every action for what you selected behind one menu instead of showing
 * icons in each row. Four icons times twenty rows is visual noise; one dot is
 * not, and a menu has room for action names.
 *
 * Only one menu is open at a time and it closes on Escape, on a click anywhere
 * else, or when an action is chosen.
 */

import { create } from "./dom.js";
import { t } from "./i18n.js";

/** The open menu, so opening another closes it. */
let open = null;
/** Called with the anchor when a menu opens, and with null when it closes. */
let notify = () => {};

/**
 * Subscribes to menu open/close.
 *
 * The listing uses it to keep the row under an open menu highlighted: without
 * it, moving the mouse to the menu un-highlights the row, and the menu no
 * longer looks attached to anything.
 */
export function onRowMenuChange(fn) {
  notify = fn || (() => {});
}

/** Closes whatever menu is open. */
export function closeRowMenu() {
  if (!open) {
    return;
  }
  const { element, onDocument, onKey } = open;
  open = null;
  element.remove();
  document.removeEventListener("click", onDocument, true);
  document.removeEventListener("keydown", onKey, true);
  notify(null);
}

/**
 * Opens a menu anchored to a button.
 *
 * options:
 *   anchor  - the button the menu belongs to
 *   label   - accessible name for the menu ("Actions for notes.txt")
 *   actions - [{ id, label, icon, danger }]
 *   onChoose - (id) => void
 */
export function openRowMenu({ anchor, label, actions, onChoose, at }) {
  closeRowMenu();
  if (!actions.length) {
    return;
  }

  const menu = create("div", {
    className: "row-menu",
    attrs: { role: "menu", "aria-label": label },
  });

  for (const action of actions) {
    const item = create("button", {
      className: `row-menu-item${action.danger ? " is-danger" : ""}`,
      attrs: { type: "button", role: "menuitem", "data-action": action.id },
    }, [
      create("span", { className: "row-menu-icon", text: action.icon || "" }),
      create("span", { text: t(action.label) }),
    ]);
    item.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      closeRowMenu();
      onChoose(action.id);
    });
    menu.append(item);
  }

  // Positioned against the button: the menu would otherwise be clipped by the
  // table's overflow, so it lives in the page and follows the button.
  document.body.append(menu);
  // Widths can read as 0 while the menu is still being laid out; the fallbacks
  // keep the arithmetic meaningful rather than producing NaN.
  const width = menu.offsetWidth || 200;
  const height = menu.offsetHeight || 0;
  const viewWidth = window.innerWidth || document.documentElement?.clientWidth || 1024;
  const margin = 8;
  let left;
  let top;

  if (at) {
    // A right-click puts the menu's top-left corner at the pointer, and folds
    // it back when it would leave the window. A button click instead aligns
    // the menu's right edge with the button, so it opens inward.
    left = at.x;
    top = at.y;
  } else {
    const box = anchor.getBoundingClientRect();
    left = box.right - width;
    top = box.bottom + 4;
    if (top + height > window.innerHeight - margin) {
      top = Math.max(margin, box.top - height - 4);
    }
  }

  left = Math.max(margin, Math.min(left, viewWidth - width - margin));
  top = Math.max(margin, Math.min(top, (window.innerHeight || 800) - height - margin));
  menu.style.left = `${Math.round(left)}px`;
  menu.style.top = `${Math.round(top)}px`;

  // Any click outside closes, including on another row's button.
  const onDocument = (event) => {
    if (!menu.contains(event.target) && !anchor.contains(event.target)) {
      closeRowMenu();
    }
  };
  const onKey = (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      closeRowMenu();
      anchor.focus();
    }
  };
  // Deferred so the click that opened the menu does not immediately close it.
  setTimeout(() => {
    document.addEventListener("click", onDocument, true);
  }, 0);
  document.addEventListener("keydown", onKey, true);

  open = { element: menu, onDocument, onKey };
  notify(anchor);
  const first = menu.querySelector("button");
  if (first) {
    first.focus();
  }
}
