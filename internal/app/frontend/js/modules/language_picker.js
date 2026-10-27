/**
 * Language picker.
 *
 * Reached from the account menu. A small dialog holding a `<select>` (user
 * decision 2026-10-27, replacing the radio list this first shipped with): the
 * native control is what a phone already knows how to handle — it opens the
 * platform's own picker, is searchable on Android and scrolls on its own — and
 * it is one row on screen instead of 33.
 *
 * The codes are shown as the application names them — `english`, `german`,
 * `portuguese_br` — with the first letter upcased. That is what the language
 * files are keyed by, so a code always matches the file it loads.
 */

import { create } from "./dom.js";
import { dialogRoot } from "./dialog.js";
import { t } from "./i18n.js";

/** Turns a language code into its label: "portuguese_br" -> "Portuguese_br". */
export function languageLabel(code) {
  const text = String(code || "");
  if (!text) {
    return "";
  }
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/**
 * Opens the picker.
 *
 * options:
 *   languages - codes to offer
 *   current   - the language in effect
 *   onChoose  - (code) => Promise|void, applied when the reader confirms
 *
 * Resolves with the chosen code when a different one was confirmed, or null
 * when the dialog was dismissed or the same language was picked. Cancelling
 * must change nothing: the reader opened the menu to look, and shutting it
 * should not switch the interface to a language they cannot read.
 */
export function openLanguagePicker({ languages, current, onChoose }) {
  const root = dialogRoot();
  const restoreFocus = document.activeElement;

  const select = create("select", {
    className: "select",
    attrs: { id: "language-select", "aria-label": t("Language") },
  });
  for (const code of languages) {
    const option = create("option", { text: languageLabel(code), attrs: { value: code } });
    option.selected = code === current;
    select.append(option);
  }

  const closeButton = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": t("Cancel"), title: t("Cancel") },
  });

  const dialog = create("div", { className: "dialog language-dialog" }, [
    create("div", { className: "dialog-header" }, [
      create("h2", { className: "dialog-title", text: t("Language") }),
      closeButton,
    ]),
    create("label", { className: "field", attrs: { for: "language-select" } }, [
      select,
    ]),
    create("div", { className: "dialog-actions" }, [
      create("button", { className: "button", text: t("Cancel"), attrs: { type: "button" } }),
      create("button", {
        className: "button button-primary-solid",
        text: t("Save"),
        attrs: { type: "button" },
      }),
    ]),
  ]);

  const [cancel, save] = dialog.querySelectorAll(".dialog-actions .button");

  return new Promise((resolve) => {
    function finish(result) {
      root.classList.add("hidden");
      root.onclick = null;
      root.onkeydown = null;
      root.replaceChildren();
      if (restoreFocus && document.contains(restoreFocus)) {
        restoreFocus.focus();
      }
      resolve(result);
    }

    closeButton.addEventListener("click", () => finish(null));
    cancel.addEventListener("click", () => finish(null));
    save.addEventListener("click", async () => {
      const code = select.value;
      if (code === current) {
        finish(null);
        return;
      }
      if (onChoose) {
        await onChoose(code);
      }
      finish(code);
    });

    // Escape and a backdrop tap close without changing anything, the same two
    // ways every other dialog closes (profile.js does the same). A dismissed
    // picker must not switch the interface to a language the reader cannot
    // read: they opened the menu to look, not necessarily to choose.
    root.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        finish(null);
      }
    };
    root.onclick = (event) => {
      if (event.target === root) {
        finish(null);
      }
    };

    root.replaceChildren(dialog);
    root.classList.remove("hidden");
    // Focus the field rather than the heading, so a keyboard user can open the
    // list straight away and Enter still means "save" here.
    select.focus();
  });
}
