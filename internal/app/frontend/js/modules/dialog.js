/**
 * Modal dialogs and prompts.
 *
 * One dialog is shown at a time. It closes on Escape, on a backdrop tap and on
 * the cancel button. Focus is trapped inside while open and restored on close,
 * which keep keyboard and mobile users from tabbing into the page behind it.
 */

import { create } from "./dom.js";

let active = null;

// Escape closes whatever dialog is open, wherever the focus is.
//
// Each dialog already handles Escape on its own container, but only the
// container *with the focus* sees the key: click a button inside a dialog and
// the event still bubbles from that button, yet a keydown with the focus
// nowhere — or on a detached node — reached no handler at all. On `document` it
// cannot be missed. Dialogs that need to ask first (the editor, which warns
// about unsaved changes) keep their own listener and are not routed here.
document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape" || !active) {
    return;
  }
  // Another handler already acted on this key.
  if (event.defaultPrevented) {
    return;
  }
  event.preventDefault();
  closeDialog();
});

/** Returns the dialog root, creating it on first use. */
export function dialogRoot() {
  let node = document.getElementById("dialog-root");
  if (!node) {
    node = create("div", {
      className: "dialog-root hidden",
      attrs: { id: "dialog-root", role: "dialog", "aria-modal": "true" },
    });
    document.body.append(node);
  }
  return node;
}

/** Closes the open dialog, resolving its promise with null. */
export function closeDialog() {
  if (!active) {
    return;
  }
  const { node, resolve, restoreFocus } = active;
  active = null;
  node.classList.add("hidden");
  node.replaceChildren();
  if (restoreFocus && document.contains(restoreFocus)) {
    restoreFocus.focus();
  }
  resolve(null);
}

/**
 * Shows a modal.
 *
 * options:
 *   title       - heading text
 *   message     - optional body text
 *   fields      - [{ id, label, type, value, placeholder }]
 *   confirmText - label of the primary button
 *   danger      - style the primary button as destructive
 *   validate    - (values) => error string or null
 *
 * Resolves to a map of field values, or null when cancelled.
 */
export function openDialog(options) {
  closeDialog();

  const node = dialogRoot();
  const fields = options.fields || [];
  const restoreFocus = document.activeElement;

  const form = create("form", { className: "dialog" }, []);

  // A visible close button so the dialog can always be dismissed, including
  // on touch devices where Escape is unavailable.
  const closeButton = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": "Close", title: "Close" },
  });
  form.append(
    create("div", { className: "dialog-header" }, [
      create("h2", { className: "dialog-title", text: options.title || "" }),
      closeButton,
    ])
  );

  if (options.message) {
    form.append(create("p", { className: "dialog-message", text: options.message }));
  }

  const inputs = new Map();
  for (const field of fields) {
    const id = `dialog-field-${field.id}`;
    const input = create("input", {
      className: "input",
      attrs: {
        id,
        name: field.id,
        type: field.type || "text",
        value: field.value || "",
        placeholder: field.placeholder || "",
        autocomplete: "off",
        spellcheck: "false",
        autocapitalize: "none",
      },
    });
    inputs.set(field.id, input);
    form.append(
      create("label", { className: "field", attrs: { for: id } }, [
        create("span", { className: "field-label", text: field.label }),
        input,
      ])
    );
  }

  const error = create("p", { className: "form-error hidden" });
  form.append(error);

  const cancel = create("button", {
    className: "button",
    text: options.cancelText || "Cancel",
    attrs: { type: "button" },
  });
  const confirm = create("button", {
    className: options.danger ? "button button-danger" : "button button-primary-solid",
    text: options.confirmText || "OK",
    attrs: { type: "submit" },
  });
  form.append(create("div", { className: "dialog-actions" }, [cancel, confirm]));

  return new Promise((resolve) => {
    active = { node, resolve, restoreFocus };

    function cancelDialog() {
      closeDialog();
    }

    function submit(event) {
      event.preventDefault();
      const values = {};
      for (const [id, input] of inputs) {
        values[id] = input.value;
      }
      if (options.validate) {
        const message = options.validate(values);
        if (message) {
          error.textContent = message;
          error.classList.remove("hidden");
          for (const input of inputs.values()) {
            input.focus();
            break;
          }
          return;
        }
      }
      const { resolve: done } = active;
      active = null;
      node.classList.add("hidden");
      node.replaceChildren();
      if (restoreFocus && document.contains(restoreFocus)) {
        restoreFocus.focus();
      }
      done(values);
    }

    form.addEventListener("submit", submit);
    cancel.addEventListener("click", cancelDialog);
    closeButton.addEventListener("click", cancelDialog);

    // A tap outside the dialog closes it, matching every other dialog.
    node.onclick = (event) => {
      if (event.target === node) {
        cancelDialog();
      }
    };

    node.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        cancelDialog();
        return;
      }
      if (event.key !== "Tab") {
        return;
      }
      // Keep focus inside the dialog.
      const focusable = node.querySelectorAll(
        'button, input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
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
    const firstInput = inputs.values().next().value;
    (firstInput || confirm).focus();
    if (firstInput && firstInput.select) {
      firstInput.select();
    }
  });
}

/** Asks for confirmation. Resolves true when confirmed. */
export async function confirmAction({ title, message, confirmText, danger }) {
  const result = await openDialog({
    title,
    message,
    confirmText,
    danger,
    fields: [],
  });
  return result !== null;
}

/** Prompts for a single line of text. Resolves to the value or null. */
export async function promptText({ title, label, value, confirmText, validate }) {
  const result = await openDialog({
    title,
    confirmText,
    fields: [{ id: "value", label, value }],
    validate: validate ? (values) => validate(values.value) : undefined,
  });
  return result ? result.value : null;
}

