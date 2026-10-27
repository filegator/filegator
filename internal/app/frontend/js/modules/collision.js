/**
 * Name-collision prompt.
 *
 * v7 silently renamed a colliding file to "name (1).ext". v8 refuses with 409
 * and asks, because renaming what the user asked for is surprising — but on a
 * batch the question must not be repeated for every file, so the answer can be
 * applied to the remaining collisions.
 */

import { closeDialog, dialogRoot } from "./dialog.js";
import { create } from "./dom.js";
import { t } from "./i18n.js";

/** Choices the prompt can return. */
export const REPLACE = "replace";
export const KEEP_BOTH = "upcount";
export const SKIP = "skip";
export const CANCEL = "cancel";

/**
 * Asks what to do about one conflicting name.
 *
 * options:
 *   name  - the entry that already exists
 *   batch - when true, offers "apply to the remaining files"
 *
 * Resolves to { action, applyToAll }, or { action: CANCEL } when dismissed.
 */
export function askCollision({ name, batch = false, allowReplace = true }) {
  const label = typeof name === "string" ? name : name && name.name ? name.name : String(name ?? "");
  // The shared container holds one dialog. With several uploads in flight, one
  // file's prompt can still be open when another file hits a collision; that
  // second prompt used to replace the first, and the first file's answer could
  // then never arrive — its upload sat at 0% forever.
  //
  // Instead of replacing, the newcomer waits for the open prompt to close and
  // then shows itself, so every collision is answered in turn.
  if (pending) {
    return pending.then(() => askCollision({ name, batch, allowReplace }));
  }
  return showCollisionPrompt({ label, batch, allowReplace });
}

/** A collision prompt that is currently open, if any. */
let pending = null;

/** Renders the prompt for one collision. */
function showCollisionPrompt({ label, batch, allowReplace }) {
  closeDialog();
  const node = dialogRoot();
  const restoreFocus = document.activeElement;

  return (pending = new Promise((resolve) => {
    let settled = false;

    const finish = (action) => {
      if (settled) {
        return;
      }
      settled = true;
      node.classList.add("hidden");
      node.onclick = null;
      node.onkeydown = null;
      node.replaceChildren();
      if (restoreFocus && document.contains(restoreFocus)) {
        restoreFocus.focus();
      }
      resolve({ action, applyToAll: Boolean(applyAll.checked) });
    };

    const applyAll = create("input", {
      className: "checkbox",
      attrs: { type: "checkbox", id: "collision-all" },
    });

    /** One choice button. */
    const choice = (label, action, className) => {
      const button = create("button", {
        className,
        text: label,
        attrs: { type: "button" },
      });
      button.addEventListener("click", () => finish(action));
      return button;
    };

    const close = create("button", {
      className: "dialog-close",
      text: "×",
      attrs: { type: "button", "aria-label": t("Close"), title: t("Close") },
    });
    close.addEventListener("click", () => finish(CANCEL));

    const form = create("form", { className: "dialog collision-dialog" }, [
      create("div", { className: "dialog-header" }, [
        create("h2", { className: "dialog-title", text: t("File exists") }),
        close,
      ]),
      create("p", {
        className: "dialog-message",
        text: t("A file with this name already exists: {0}", label),
      }),
      batch
        ? create("label", { className: "field field-inline", attrs: { for: "collision-all" } }, [
            applyAll,
            create("span", { className: "field-label", text: t("Apply to the remaining files") }),
          ])
        : null,
      create(
        "div",
        { className: "dialog-actions collision-actions" },
        [
          choice(t("Cancel"), CANCEL, "button"),
          choice(t("Skip"), SKIP, "button"),
          choice(t("Keep both"), KEEP_BOTH, "button button-primary-solid"),
          // Replace destroys the existing file, so it is styled and placed
          // where it cannot be hit by accident. It is not offered for folders,
          // where it would delete a whole tree.
          allowReplace ? choice(t("Replace"), REPLACE, "button button-danger") : null,
        ].filter(Boolean)
      ),
    ].filter(Boolean));

    // Enter submits the form by default; every action is an explicit button, so
    // keep Enter from doing nothing surprising.
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      finish(CANCEL);
    });

    // Dismissing with Escape or a backdrop tap cancels. Doing nothing is the
    // safe outcome; "keep both" would change names without being asked.
    node.onclick = (event) => {
      if (event.target === node) {
        finish(CANCEL);
      }
    };
    node.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        finish(CANCEL);
        return;
      }
      if (event.key !== "Tab") {
        return;
      }
      const focusable = node.querySelectorAll("button, input");
      if (!focusable.length) {
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    node.replaceChildren(form);
    node.classList.remove("hidden");
    // Focus "Keep both": it is the non-destructive choice that still keeps the
    // user's file.
    const preferred = form.querySelector(".button-primary-solid");
    (preferred || close).focus();
  }).finally(() => {
    pending = null;
  }));
}
