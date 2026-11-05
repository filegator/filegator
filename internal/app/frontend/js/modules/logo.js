/**
 * The logo, in two themes.
 *
 * The branding comes from /api/config as a pair — `logo` for light mode and
 * `logo_dark` for dark mode. Both are empty when the administrator removed the
 * logo; the header and the login card then show `app_name` instead. One pair
 * serves every surface: the file browser header, the administration header and
 * the login card.
 *
 * Each logo in the markup is TWO images distinguished by class rather than by
 * id: a `.logo-light` for the light theme and a `.logo-dark` for the dark one,
 * inside a group whose ids are `<group>-light` and `<group>-dark`. The visible
 * one is chosen by CSS (`--logo-light`/`--logo-dark` in app.css), before any
 * script runs, and `display: none` keeps the browser from downloading the
 * other. This module only fills in the src of both.
 */

/**
 * The artwork embedded in the binary.
 *
 * These are config.DefaultLogo and config.DefaultLogoDark, repeated because the
 * browser cannot import Go. The admin page uses them as the placeholder hints
 * on an empty logo field, so the built-in artwork stays reachable after it has
 * been removed; an empty field itself shows no logo.
 */
export const BUILT_IN_LOGO = "/img/filegator-logo.svg";
export const BUILT_IN_LOGO_DARK = "/img/filegator-logo-dark.svg";

/**
 * Points the tab icon at one of the logos, or removes it when there is none.
 *
 * The favicon is the logo, as on the docs site: one SVG, no raster copy to keep
 * in step. It is applied once /api/config answers, so an installation without
 * a logo keeps the browser's own tab icon rather than falling back to the
 * built-in FileGator drawing.
 *
 * A non-SVG file still works: a browser that does not read it as an icon falls
 * back to its own, which is the same loss as having no favicon.
 */
export function applyFavicon(src) {
  let link = document.querySelector('link[rel="icon"]');
  if (!src) {
    if (link) {
      link.remove();
    }
    return;
  }
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    document.head.appendChild(link);
  }
  if (/\.svg(?:[?#]|$)/i.test(src)) {
    link.setAttribute("type", "image/svg+xml");
  } else {
    link.removeAttribute("type");
  }
  if (link.getAttribute("href") !== src) {
    link.setAttribute("href", src);
  }
}

/**
 * Points one group at a light/dark pair and shows it.
 *
 * `light` is required; `dark` falls back to it, so a picture supplied without
 * a dark counterpart is shown in both themes rather than disappearing on a
 * dark page. Passing no `light` hides the group.
 */
export function applyLogo(group, light, dark, alt) {
  const images = {
    light: document.getElementById(group + "-light"),
    dark: document.getElementById(group + "-dark"),
  };
  if (!light) {
    setGroupVisible(images, false);
    return;
  }
  setSrc(images.light, light, alt);
  setSrc(images.dark, dark || light, alt);
  setGroupVisible(images, true);
}

/**
 * Shows the application title where a logo would have been.
 *
 * The title is the fallback branding, not a caption: when a logo is configured
 * it is hidden, and when the logo is removed it takes the logo's place in the
 * header and on the login card. `visible` is false when a logo is set, so the
 * two never appear side by side.
 */
export function applyAppName(id, title, visible) {
  const element = document.getElementById(id);
  if (!element) {
    return;
  }
  element.textContent = title || "";
  element.classList.toggle("hidden", !visible || !title);
}

/**
 * The pair the page should show, read from the public configuration. Kept here
 * so app.js does not have to remember which key belongs to which theme, and so
 * a missing key means "no logo" in one place.
 */
export function logosFromConfig(config) {
  const c = config || {};
  return { light: c.logo || "", dark: c.logo_dark || "" };
}

/**
 * The logo the tab icon follows.
 *
 * The dark variant is not used here: a tab icon sits on the browser's own
 * chrome, not on our page background, so it has one picture rather than a
 * themed pair. No configured logo means no tab icon at all, rather than the
 * built-in artwork reappearing after it was removed.
 */
export function faviconFromConfig(config) {
  return logosFromConfig(config).light || "";
}

/**
 * What to show before /api/config has answered, and if it never does.
 *
 * The artwork embedded in the binary, so the first paint is branded even before
 * /api/config answers. A configuration with empty logo fields replaces it with
 * the application title once it arrives.
 */
export function defaultLogos() {
  return { light: BUILT_IN_LOGO, dark: BUILT_IN_LOGO_DARK };
}

function setSrc(image, src, alt) {
  if (!image) {
    return;
  }
  if (image.getAttribute("src") !== src) {
    image.setAttribute("src", src);
  }
  image.setAttribute("alt", alt || "");
}

/**
 * Shows or hides a group. The class goes on both images, each of which keeps
 * its own theme rule: hiding must not turn into "show one of them".
 */
function setGroupVisible(images, visible) {
  for (const image of [images.light, images.dark]) {
    if (!image) {
      continue;
    }
    image.classList.toggle("hidden", !visible);
  }
}
