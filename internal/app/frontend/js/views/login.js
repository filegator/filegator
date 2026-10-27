/**
 * Login screen.
 */

import { api, ApiError } from "../modules/api.js";
import { el, hide, setVisible, show } from "../modules/dom.js";
import { languages, canSelectLanguage, t } from "../modules/i18n.js";
import { applyLogo } from "../modules/logo.js";

/**
 * Wires up the login form.
 * onSuccess is called with the authenticated user.
 */
/**
 * Returns a URL safe to redirect to, or "" when there is none.
 *
 * Only absolute http/https URLs are accepted. A relative value would point
 * back into the application (normally a loop), and any other scheme is a
 * scripting risk.
 */
export function redirectTarget(value) {
  const raw = (value || "").trim();
  if (!raw) {
    return "";
  }
  // A relative value is almost always a mistake: the app resolves it against
  // itself, so "login" would redirect the login page to the login page.
  if (!/^https?:\/\//i.test(raw)) {
    return "";
  }
  let parsed;
  try {
    parsed = new URL(raw, window.location.href);
  } catch {
    return "";
  }
  return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : "";
}

export function createLogin({ onSuccess, onLanguageChange }) {
  const view = el("login-view");
  const form = el("login-form");
  const username = el("login-username");
  const password = el("login-password");
  const error = el("login-error");
  const submit = el("login-submit");
  const langButton = el("login-language");
  // Set from /api/config; empty means the form is shown as usual.
  let config = {};

  /** Shows the language link only when there is a choice to make. */
  function renderLanguage() {
    setVisible(langButton, canSelectLanguage() && languages().length > 1);
  }

  langButton.addEventListener("click", async () => {
    // Opens the same dialog the account menu uses. The caller applies the
    // choice and relabels this screen, so nothing is done here beyond keeping
    // the focus where the reader left it.
    await onLanguageChange();
  });

  function showError(key) {
    error.dataset.key = key;
    error.textContent = t(key);
    show(error);
  }

  function clearError() {
    delete error.dataset.key;
    error.textContent = "";
    hide(error);
  }

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearError();

    const name = username.value.trim();
    const pass = password.value;
    if (!name || !pass) {
      showError("Please enter username and password");
      return;
    }

    submit.disabled = true;
    submit.textContent = t("Signing in");
    try {
      const user = await api.login(name, pass);
      password.value = "";
      onSuccess(user);
    } catch (err) {
      showError(err instanceof ApiError ? err.message : "Unknown error");
      password.value = "";
      password.focus();
    } finally {
      submit.disabled = false;
      submit.textContent = t("Login");
    }
  });

  /** Re-applies translations after a language change. */
  function render() {
    el("label-username").textContent = t("Username");
    el("label-password").textContent = t("Password");
    submit.textContent = t("Login");
    el("login-language-label").textContent = t("Language");
    renderLanguage();
    if (error.dataset.key) {
      error.textContent = t(error.dataset.key);
    }
  }

  return {
    /**
     * Shows the login screen, or leaves for the configured guest redirection.
     *
     * v7 sent a logged-out visitor to that URL instead of showing a form,
     * which is how an external login (WordPress, SSO) is reached. It applies
     * only to http(s) URLs: a value like "javascript:" would execute in this
     * origin, and the setting is stored by an administrator but read by
     * everyone.
     */
    show() {
      const target = redirectTarget(config.guest_redirection);
      if (target) {
        window.location.assign(target);
        return;
      }
      render();
      view.classList.remove("hidden");
      username.focus();
    },
    hide() {
      view.classList.add("hidden");
    },
    setConfig(next) {
      config = next || {};
    },
    // The login card shows the logo only; the app name is the page title. It
    // is the same logo as the header, in the theme's variant.
    setBrand(_appName, logos) {
      render();
      applyLogo("login-logo", logos.light, logos.dark, "");
    },
  };
}
