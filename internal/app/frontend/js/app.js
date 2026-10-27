/**
 * Application entry point: boot, routing between login and browser views.
 */

import { api, ApiError } from "./modules/api.js";
import { el, hide, show } from "./modules/dom.js";
import { init as initI18n, t, canSelectLanguage, languages, language, setLanguage } from "./modules/i18n.js";
import { openLanguagePicker } from "./modules/language_picker.js";
import { applyFavicon, defaultLogos, faviconFromConfig, logosFromConfig } from "./modules/logo.js";
import { createNotices } from "./modules/notices.js";
import { createTheme } from "./modules/theme.js";
import { createUserMenu, userMenuItems } from "./modules/user_menu.js";
import { createAdmin } from "./views/admin.js";
import { createBrowser } from "./views/browser.js";
import { createLogin } from "./views/login.js";
import { openChangePassword } from "./views/profile.js";

let login;
let browser;
let admin;
let userMenu = null;
let currentUser = null;
let notices = null;
let capabilities = {};
// An empty appName means "logo only": the interface shows no title text.
//
// The logo is a light/dark pair serving the header, the admin header and the
// login card (js/modules/logo.js). Until /api/config answers, and if it never
// does, the page already shows the artwork embedded in the binary: the same
// files the server substitutes for an unset logo, so nothing flashes.
let branding = { appName: "FileGator", logos: defaultLogos() };
// What this start of the server created or could not resolve, from
// /api/config. Shown in the notice bar to administrators (see modules/notices.js).
let startupNotes = [];

function showBoot() {
  show(el("boot"));
}

function hideBoot() {
  hide(el("boot"));
}

function enterApp(user) {
  currentUser = user;
  // The status endpoint is administrator-only, so asking as anybody else is a
  // guaranteed 403 in the browser console. Only administrators are asked.
  notices.setUser(user);
  if (user && user.role === "admin") {
    notices.refresh();
  } else {
    notices.clear();
  }
  login.hide();
  admin.hide();
  updateAdminButton(user);
  browser.setUser(user);
  browser.setBrand(branding.appName, branding.logos);
  browser.show();
  hideBoot();
}

/** Shows the admin area (administrators only). */
function enterAdmin() {
  browser.hide();
  admin.show();
}

/** Re-enables the admin button only for administrators. */
function updateAdminButton(user) {
  const button = el("btn-admin");
  button.classList.toggle("hidden", user.role !== "admin");
}



function showLogin() {
  currentUser = null;
  if (notices) {
    notices.setUser(null);
    notices.clear();
  }
  // The next account must not inherit the previous one's folders, listing or
  // current path, including through the URL hash.
  browser.reset();
  browser.hide();
  admin.hide();
  login.setBrand(branding.appName, branding.logos);
  login.show();
  hideBoot();
}

async function logout(cancelled) {
  // `cancelled` means the guest's "Log in" button: there is no session to end.
  // Logging out can also be switched off (usual with SSO); the server refuses
  // it then, so it is not called.
  if (!cancelled && capabilities.auth_logout !== false) {
    try {
      await api.logout();
    } catch {
      // The cookie is cleared server-side; ignore network errors.
    }
  }
  showLogin();
}

/** Loads the frontend configuration, when the server provides it. */
async function loadBranding() {
  try {
    const config = await api.config();
    branding = {
      appName: config.app_name || "",
      logos: logosFromConfig(config),
    };
    // What this start of the server created, or could not resolve. Kept for
    // the notice bar, which rebuilds from it whenever an administrator signs in.
    startupNotes = config.startup_notes || [];
    if (browser) {
      browser.setConfig(config);
    }
    if (login) {
      // guest_redirection decides whether visitors see a form at all.
      login.setConfig(config);
    }
    // The tab icon follows the logo, so a customised install is branded in the
    // browser tab too. An earlier page sets the current theme's built-in file,
    // which faviconFromConfig leaves alone when nothing was configured.
    applyFavicon(faviconFromConfig(config));
  } catch {
    // Configuration is optional during boot; defaults are fine.
  }
}

/**
 * The account menu's choices that end or start a session, wired by main().
 * Kept as a hook so the menu's handler does not have to know about views.
 */
let onLogoutChoice = () => {};

async function main() {
  showBoot();

  notices = createNotices({
    onChangePassword: async () => {
      if (await openChangePassword()) {
        notices.refresh();
      }
    },
    // What this start of the server created (private/, users file, storage
    // folder, config.json) or could not resolve. A function because the
    // configuration is fetched in loadBranding(), after this object exists.
    startup: () => startupNotes,
  });

  el("btn-admin").addEventListener("click", enterAdmin);

  // "Log out" ends the session; a guest's "Log in" shows the login form
  // instead. `logout(true)` is the cancelled form: nothing to end.
  onLogoutChoice = (cancelled) => {
    logout(cancelled);
  };

  /**
   * Reports whether the signed-in user may change their own password.
   *
   * Both the provider (LDAP cannot) and the administrator's setting (which can
   * switch self-service off) decide this. Read on every click rather than
   * captured once, so a settings change takes effect without a reload.
   */
  function canChangePassword() {
    return capabilities.auth_change_password !== false;
  }

  /**
   * Reports whether the signed-in user may log out.
   *
   * With SSO the identity provider keeps the session, so a local logout only
   * looks broken; the administrator turns it off and the server refuses too.
   */
  function canLogout() {
    return capabilities.auth_logout !== false;
  }

  /**
   * Re-reads the capability flags.
   *
   * The defaults are permissive: a failed request must not strip controls the
   * user actually has. The server still enforces permissions, so showing a
   * button is never what grants access.
   */
  async function refreshCapabilities() {
    try {
      capabilities = (await api.capabilities()) || {};
    } catch {
      capabilities = {};
    }
    return capabilities;
  }

  /**
   * Reports whether the account menu should offer the language picker.
   *
   * Two conditions: the administrator allows users to choose
   * (`frontend_config.language_selection`), and there is more than one language
   * to choose from. A picker offering a single entry is a dead control.
   *
   * Read from the i18n module, which loaded the list at boot: it is the same
   * source the login form uses, so the two cannot disagree.
   */
  function canPickLanguage() {
    return canSelectLanguage() && languages().length > 1;
  }

  /**
   * Opens the language picker and applies the choice.
   *
   * `setLanguage` swaps the message table but cannot redraw anything, so the
   * browser view is asked to relabel itself and the open menu is rebuilt
   * afterwards. The login form builds its own labels from the same table, so it
   * picks the change up the next time it renders.
   */
  async function chooseLanguage() {
    const chosen = await openLanguagePicker({
      languages: languages(),
      current: language(),
      onChoose: async (code) => {
        await setLanguage(code);
        if (browser) {
          browser.applyLanguage();
        }
        if (userMenu) {
          userMenu.refresh();
        }
      },
    });
    return chosen;
  }

  const leaveAdmin = () => {
    if (currentUser) {
      enterApp(currentUser);
    }
  };
  // "← Files" is rendered by admin.js as the first section, so it is not bound
  // here. The logo is also a way back, since there is no folder browsing in
  // the administration area to "go home" to.
  el("admin-home").addEventListener("click", (event) => {
    event.preventDefault();
    leaveAdmin();
  });

  await loadBranding();
  // The browser tab still needs a name even when the page shows a logo only.
  document.title = branding.appName || "FileGator";

  // Capabilities decide which controls are shown; fetched at startup and
  // again whenever the administrator saves settings, because a setting can
  // change what the header offers.
  await refreshCapabilities();

  try {
    await initI18n();
  } catch (err) {
    // Without translations the UI still works with raw keys.
    console.error("i18n init failed", err);
  }

  // The theme was applied by the inline script in index.html before the first
  // paint; this owns the choice and the menu item that changes it. A browser
  // without matchMedia is treated as light.
  const theme = createTheme({
    storage: window.localStorage,
    query: window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)"),
    root: document.documentElement,
    // The menu names the theme the next click produces, so it is rebuilt when
    // the theme changes — including when the system preference flips while the
    // user has not chosen.
    onChange: () => userMenu && userMenu.refresh(),
  });

  /**
   * The account menu: theme, password, logout.
   *
   * Everything is decided on the click, not when the menu was built, because
   * an administrator can switch password changes or logout off while this page
   * is open — binding once at startup left an item either dead after enabling
   * the setting or gone until a reload after disabling it.
   */
  userMenu = createUserMenu({
    button: el("btn-user-menu"),
    element: el("user-menu"),
    items: () =>
      userMenuItems({
        user: currentUser,
        canChangePassword: canChangePassword(),
        canLogout: canLogout(),
        canPickLanguage: canPickLanguage(),
        theme: theme.get(),
      }),
    onChoose: async (id) => {
      if (id === "theme") {
        theme.toggle();
        return;
      }
      if (id === "language") {
        await chooseLanguage();
        return;
      }
      if (id === "password") {
        if (!canChangePassword()) {
          return;
        }
        const changed = await openChangePassword();
        if (changed) {
          // The session cookie was reissued; refresh the notice bar so the
          // default-password warning disappears immediately.
          notices.refresh();
        }
        return;
      }
      if (id === "logout" || id === "login") {
        onLogoutChoice(id === "login");
      }
    },
  });

  browser = createBrowser({ onLogout: logout });

  admin = createAdmin({
    capabilities,
    onSettingsChange: async () => {
      // Appearance changes should be visible immediately, so reload branding.
      loadBranding().then(() => {
        if (currentUser) {
          browser.setBrand(branding.appName, branding.logos);
        }
      });
      // Settings can change what the header offers (self-service password
      // changes). Re-read the flags so the profile button follows without a
      // reload.
      await refreshCapabilities();
    },
    onClose: () => {
      if (currentUser) {
        enterApp(currentUser);
      } else {
        showLogin();
      }
    },
    onRootChange: () => {
      // The storage root moved; reload the listing from the browser.
      browser.navigate("/");
    },
  });

  login = createLogin({
    // enterApp -> browser.setUser labels the header for the session, so
    // signing in after browsing as a guest switches the button automatically.
    onSuccess: enterApp,
    // The login screen's language link opens the same dialog as the account
    // menu, so a visitor meets one picker, not a second design.
    onLanguageChange: async () => {
      await chooseLanguage();
    },
  });

  try {
    const config = await api.config();
    browser.setConfig(config);
    // Also hand it to the login view: loadBranding() ran before it existed.
    login.setConfig(config);
  } catch {
    /* configuration is optional for uploads */
  }

  try {
    const user = await api.me();
    enterApp(user);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      showLogin();
      return;
    }
    hideBoot();
    const error = el("login-error");
    error.textContent = t("Unknown error");
    error.classList.remove("hidden");
  }
}

main();
