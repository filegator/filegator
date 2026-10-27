/**
 * The logo, in two themes.
 *
 * The branding comes from /api/config as a pair — `logo` for light mode and
 * `logo_dark` for dark mode — and the server has already replaced an empty
 * value with the FileGator artwork embedded in the binary. One pair serves
 * every surface: the file browser header, the administration header and the
 * login card.
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
 * browser cannot import Go. The admin page's live preview shows them for an
 * empty field, which is what the server resolves an empty logo to.
 */
export const BUILT_IN_LOGO = "/img/filegator-logo.svg";
export const BUILT_IN_LOGO_DARK = "/img/filegator-logo-dark.svg";

/**
 * Points the tab icon at one of the logos.
 *
 * The favicon is the logo, as on the docs site: one SVG, no raster copy to keep
 * in step. index.html sets it to the built-in file for the active theme before
 * the first paint, and this replaces it when the administrator configured a
 * logo of their own — a browser tab showing FileGator's drawing beside a
 * customised product name would be the same drift the docs fixed.
 *
 * A non-SVG file still works: a browser that does not read it as an icon falls
 * back to its own, which is the same loss as having no favicon.
 */
export function applyFavicon(src) {
  if (!src) {
    return;
  }
  let link = document.querySelector('link[rel="icon"]');
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
 * themed pair. With no configured logo the built-in artwork stays.
 */
export function faviconFromConfig(config) {
  return logosFromConfig(config).light || BUILT_IN_LOGO;
}

/**
 * What to show before /api/config has answered, and if it never does.
 *
 * The same artwork the server substitutes for an unset logo: a page that
 * already has its branding in the first paint, which is what "embedded in the
 * binary" is for.
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
