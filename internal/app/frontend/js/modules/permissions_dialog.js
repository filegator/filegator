/**
 * Permission dialog: change the mode of one entry.
 *
 * v7 showed a checkbox grid for owner/group/other with read/write/execute, a
 * numeric field, and — for folders — a choice of what to apply it to
 * recursively. The checkboxes and the number stay in sync in both directions.
 */

import { closeDialog, dialogRoot } from "./dialog.js";
import { create } from "./dom.js";
import { t } from "./i18n.js";
import {
  BITS,
  RECURSIVE,
  SCOPES,
  formatMode,
  hasBit,
  parseMode,
  setBit,
  symbolic,
} from "./permissions.js";

/** Human labels for the scopes, kept short for narrow screens. */
const SCOPE_LABEL = {
  owner: "Owner",
  group: "Group",
  other: "Other",
};

/**
 * Opens the permission dialog.
 *
 * options:
 *   name    - entry name shown in the title
 *   mode    - current mode as "644"
 *   isDir   - shows the recursive choices for a folder
 *
 * Resolves to { mode, recursive } or null when cancelled.
 */
export function openPermissions({ name, mode, isDir }) {
  closeDialog();
  const node = dialogRoot();
  const restoreFocus = document.activeElement;

  let current = parseMode(mode) ?? 0o644;

  return new Promise((resolve) => {
    let settled = false;

    const finish = (value) => {
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
      resolve(value);
    };

    // --- the numeric field -------------------------------------------------

    const numeric = create("input", {
      className: "input permission-number",
      attrs: {
        type: "text",
        inputmode: "numeric",
        maxlength: "3",
        value: formatMode(current),
        "aria-label": t("Permissions"),
      },
    });
    const message = create("p", { className: "form-error hidden" });

    // --- the checkbox grid -------------------------------------------------

    const boxes = new Map(); // "owner:read" -> input
    const grid = create("div", { className: "perm-matrix" });

    grid.append(create("div", { className: "perm-head" }, [
      create("span"),
      ...BITS.map((bit) => create("span", { text: bit.charAt(0).toUpperCase() + bit.slice(1) })),
    ]));
    for (const scope of SCOPES) {
      const row = create("div", { className: "perm-row" }, [
        create("span", { className: "perm-scope", text: SCOPE_LABEL[scope] }),
      ]);
      for (const bit of BITS) {
        // The id is kept in a local: create() sets attributes on the element
        // and keeps nothing on the object, so reading it back was undefined.
        const id = `perm-${scope}-${bit}`;
        const box = create("input", {
          className: "checkbox",
          attrs: {
            type: "checkbox",
            id,
            "aria-label": `${SCOPE_LABEL[scope]} ${bit}`,
          },
        });
        box.addEventListener("change", () => {
          current = setBit(current, scope, bit, box.checked);
          refresh();
        });
        boxes.set(`${scope}:${bit}`, box);
        row.append(create("label", { className: "perm-cell", attrs: { for: id } }, [box]));
      }
      grid.append(row);
    }

    // --- recursive (folders only) -----------------------------------------

    const recursive = create("select", { className: "select" });
    for (const [value, label] of [
      [RECURSIVE.none, t("No")],
      [RECURSIVE.all, t("Both folders and files")],
      [RECURSIVE.folders, t("Apply only for folders")],
      [RECURSIVE.files, t("Apply only for files")],
    ]) {
      recursive.append(
        Object.assign(document.createElement("option"), {
          value: value ?? "",
          textContent: label,
        })
      );
    }

    /** Pushes `current` into every control. */
    function refresh() {
      const octal = formatMode(current);
      if (numeric.value !== octal) {
        numeric.value = octal;
      }
      for (const [key, box] of boxes) {
        const [scope, bit] = key.split(":");
        box.checked = hasBit(current, scope, bit);
      }
      preview.textContent = `-${symbolic(current)}`;
    }

    // Typing a number moves the checkboxes; an invalid value is reported but
    // does not reset what is typed, so a half-typed "7" is not fought.
    numeric.addEventListener("input", () => {
      const parsed = parseMode(numeric.value);
      if (parsed === null) {
        message.textContent = t("Must be a number between 0 and 777");
        message.classList.remove("hidden");
        return;
      }
      message.classList.add("hidden");
      current = parsed;
      // Refresh the boxes without overwriting the field being typed in.
      for (const [key, box] of boxes) {
        const [scope, bit] = key.split(":");
        box.checked = hasBit(current, scope, bit);
      }
      preview.textContent = `-${symbolic(current)}`;
    });

    const preview = create("p", { className: "perm-preview" });
    refresh();

    const close = create("button", {
      className: "dialog-close",
      text: "×",
      attrs: { type: "button", "aria-label": t("Close"), title: t("Close") },
    });
    close.addEventListener("click", () => finish(null));

    const cancel = create("button", {
      className: "button",
      text: t("Cancel"),
      attrs: { type: "button" },
    });
    cancel.addEventListener("click", () => finish(null));

    const save = create("button", {
      className: "button button-primary-solid",
      text: t("Save"),
      attrs: { type: "submit" },
    });

    const form = create("form", { className: "dialog permissions-dialog" }, [
      create("div", { className: "dialog-header" }, [
        create("h2", {
          className: "dialog-title",
          text: t("Change mode for {0}", name),
        }),
        close,
      ]),
      grid,
      create("div", { className: "perm-number-row" }, [
        create("label", { className: "field field-inline", attrs: { for: "perm-number" } }, [
          create("span", { className: "field-label", text: t("Mode") }),
          numeric,
        ]),
        preview,
      ]),
      isDir
        ? create("label", { className: "field", attrs: { for: "perm-recursive" } }, [
            create("span", { className: "field-label", text: t("Recursive") }),
            recursive,
          ])
        : null,
      message,
      create("p", {
        className: "admin-hint",
        text: isDir
          ? t("Setuid, setgid and sticky bits are not supported.")
          : t("Only the three permission digits are supported."),
      }),
      create("div", { className: "dialog-actions" }, [cancel, save]),
    ].filter(Boolean));

    form.addEventListener("submit", (event) => {
      event.preventDefault();
      const parsed = parseMode(numeric.value);
      if (parsed === null) {
        message.textContent = t("Must be a number between 0 and 777");
        message.classList.remove("hidden");
        numeric.focus();
        return;
      }
      finish({
        mode: formatMode(parsed),
        recursive: isDir ? recursive.value || null : null,
      });
    });

    numeric.id = "perm-number";
    recursive.id = "perm-recursive";

    node.onclick = (event) => {
      if (event.target === node) {
        finish(null);
      }
    };
    node.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        finish(null);
      }
    };

    node.replaceChildren(form);
    node.classList.remove("hidden");
    numeric.focus();
    numeric.select();
  });
}
