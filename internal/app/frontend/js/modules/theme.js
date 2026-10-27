/**
 * Light/dark theme.
 *
 * The theme is selected by `data-theme` on <html>: no attribute means "follow
 * the system". That attribute is set by a small inline script in index.html
 * before the first paint, so a user whose choice is dark never sees a flash of
 * light — which is why the reading of the stored key lives here and the
 * applying happens in the page, not on DOMContentLoaded.
 *
 * LIGHT and DARK are the two choices; SYSTEM is the absence of both.
 */

export const LIGHT = "light";
export const DARK = "dark";

/** The localStorage key. Namespaced like the other filegator keys. */
export const STORAGE_KEY = "filegator.theme";

/** What the user chose, or "" when they never chose and follow the system. */
export function storedTheme(storage) {
  try {
    const value = storage && storage.getItem(STORAGE_KEY);
    return value === LIGHT || value === DARK ? value : "";
  } catch (e) {
    // A browser with storage disabled (private mode, blocked cookies) throws
    // on access. Following the system is the right answer there.
    return "";
  }
}

/** Remembers a choice. A failure is not worth reporting: the theme applies now. */
export function storeTheme(storage, theme) {
  try {
    storage && storage.setItem(STORAGE_KEY, theme);
  } catch (e) {
    // Nothing to do; the choice lasts for this page view.
  }
}

/**
 * Resolves the theme actually in effect: the stored choice, or the system's
 * preference when there is none. `query` is a matchMedia result, and a browser
 * without one is treated as light.
 */
export function resolveTheme(stored, query) {
  if (stored === LIGHT || stored === DARK) {
    return stored;
  }
  return query && query.matches ? DARK : LIGHT;
}

/** The theme a click on the toggle produces. */
export function nextTheme(current) {
  return current === DARK ? LIGHT : DARK;
}

/**
 * Holds the current theme and reports changes.
 *
 * `onChange(theme)` runs once immediately — so a listener never has to know
 * the starting value — and again on every change, whether from a click or
 * from the system preference changing while the page is open and the user has
 * not chosen.
 */
export function createTheme({ storage, query, root, onChange }) {
  let stored = storedTheme(storage);
  let current = resolveTheme(stored, query);

  function apply(theme) {
    current = theme;
    // "Following the system" is the absence of the attribute, not a third
    // value: the stylesheet only knows light and dark, and leaving it unset
    // lets the media query decide.
    if (root) {
      if (stored) {
        root.setAttribute("data-theme", theme);
      } else {
        root.removeAttribute("data-theme");
      }
    }
    onChange(current);
  }

  function toggle() {
    stored = nextTheme(current);
    storeTheme(storage, stored);
    apply(stored);
    return current;
  }

  // A user who has not chosen follows the system, including when it changes.
  // A user who has chosen is not overridden.
  if (query && query.addEventListener) {
    query.addEventListener("change", () => {
      if (!stored) {
        apply(resolveTheme("", query));
      }
    });
  }

  apply(current);

  return {
    get: () => current,
    toggle,
  };
}
