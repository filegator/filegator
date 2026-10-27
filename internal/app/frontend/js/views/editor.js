/**
 * Text editor for editable files.
 *
 * Opens in its own full-screen overlay. Ctrl/Cmd+S saves, Escape asks before
 * discarding unsaved changes.
 */

import { api, ApiError } from "../modules/api.js";
import { create } from "../modules/dom.js";
import { t } from "../modules/i18n.js";

/**
 * Opens the editor for a path.
 *
 * options:
 *   onSaved() - called after a successful save so the browser can refresh
 */
export async function openEditor(path, { onSaved } = {}) {
  // Failures propagate so the caller can show a toast.
  const file = await api.content(path);

  const overlay = create("div", {
    className: "editor-view",
    attrs: { role: "dialog", "aria-modal": "true" },
  });

  const title = create("span", { className: "editor-title", text: file.name });
  const status = create("span", { className: "editor-status" });

  const textarea = create("textarea", {
    className: "editor-textarea",
    attrs: { spellcheck: "false", autocapitalize: "none", wrap: "off" },
  });
  // textarea.value preserves whitespace and avoids HTML escaping issues.
  textarea.value = file.content;

  const closeButton = create("button", {
    className: "button button-ghost",
    text: "×",
    attrs: { type: "button", "aria-label": t("Cancel"), title: t("Cancel") },
  });
  const saveButton = create("button", {
    className: "button button-primary-solid",
    text: t("Save"),
    attrs: { type: "button" },
  });

  const overlayEl = create("div", { className: "editor-header" }, [
    title,
    create("div", { className: "editor-actions" }, [status, saveButton, closeButton]),
  ]);

  overlay.append(overlayEl, textarea);
  document.body.append(overlay);
  textarea.focus();

  function setStatus(text, kind) {
    status.textContent = text;
    status.className = `editor-status${kind ? ` is-${kind}` : ""}`;
  }

  function markDirty() {
    const dirty = textarea.value !== file.content;
    setStatus(dirty ? t("Unsaved changes") : "", dirty ? "warning" : "");
  }

  async function save() {
    saveButton.disabled = true;
    setStatus(t("Saving"), "");
    try {
      await api.save(path, textarea.value);
      setStatus(t("Saved"), "ok");
      // The saved content becomes the baseline for dirty detection.
      file.content = textarea.value;
      setStatus(t("Saved"), "ok");
      if (onSaved) {
        onSaved();
      }
    } catch (err) {
      setStatus(err instanceof ApiError ? t(err.message) : t("Unknown error"), "error");
    } finally {
      saveButton.disabled = false;
    }
  }

  function close(force) {
    if (!force && textarea.value !== file.content) {
      // Confirm before discarding edits.
      if (!window.confirm(t("Discard unsaved changes?"))) {
        return;
      }
    }
    overlay.remove();
    document.removeEventListener("keydown", onKey);
  }

  function onKey(event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
      event.preventDefault();
      save();
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      close(false);
    }
  }

  textarea.addEventListener("input", markDirty);
  saveButton.addEventListener("click", save);
  closeButton.addEventListener("click", () => close(false));
  document.addEventListener("keydown", onKey);
}

