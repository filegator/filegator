/**
 * Profile panel: change your own password.
 */

import { api, ApiError } from "../modules/api.js";
import { create } from "../modules/dom.js";
import { dialogRoot } from "../modules/dialog.js";
import { t } from "../modules/i18n.js";

/**
 * Opens the change-password dialog.
 *
 * Resolves true when the password was changed.
 */
export async function openChangePassword() {
  const form = create("form", { className: "dialog" }, []);
  const error = create("p", { className: "form-error hidden" });

  const closeButton = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": t("Cancel"), title: t("Cancel") },
  });
  form.append(
    create("div", { className: "dialog-header" }, [
      create("h2", { className: "dialog-title", text: t("Change password") }),
      closeButton,
    ])
  );

  const current = inputField(form, t("Current password"), "current-password");
  const next = inputField(form, t("New password"), "new-password");
  const confirm = inputField(form, t("Confirm new password"), "new-password");

  form.append(
    error,
    create("div", { className: "dialog-actions" }, [
      submitButton(t("Cancel"), "button"),
      submitButton(t("Save"), "button button-primary-solid", "submit"),
    ])
  );

  const root = dialogRoot();

  return new Promise((resolve) => {
    function close(result) {
      root.classList.add("hidden");
      root.onclick = null;
      root.replaceChildren();
      resolve(result);
    }

    function fail(message) {
      error.textContent = message;
      error.classList.remove("hidden");
    }

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      if (!current.value) {
        fail(t("Current password is incorrect"));
        current.focus();
        return;
      }
      if (!next.value) {
        fail(t("New password"));
        next.focus();
        return;
      }
      if (next.value !== confirm.value) {
        fail(t("Passwords do not match"));
        confirm.focus();
        return;
      }

      try {
        await api.changePassword(current.value, next.value);
        close(true);
      } catch (err) {
        fail(err instanceof ApiError ? t(err.message) : t("Unknown error"));
      }
    });

    root.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close(false);
      }
    };
    root.onclick = (event) => {
      if (event.target === root) {
        close(false);
      }
    };
    closeButton.addEventListener("click", () => close(false));
    form.querySelectorAll(".button")[0].addEventListener("click", () => close(false));

    root.replaceChildren(form);
    root.classList.remove("hidden");
    current.focus();
  });
}

/** Adds a password input and returns it. */
function inputField(form, labelText, autocomplete) {
  const input = create("input", {
    className: "input",
    attrs: { type: "password", autocomplete, spellcheck: "false" },
  });
  form.append(
    create("label", { className: "field" }, [
      create("span", { className: "field-label", text: labelText }),
      input,
    ])
  );
  return input;
}

/** Creates a dialog button. */
function submitButton(label, className, type = "button") {
  return create("button", {
    className,
    text: label,
    attrs: { type },
  });
}

