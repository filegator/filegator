/**
 * Translation loading and lookup.
 *
 * Messages come from the server as {"Key": "translated"}. Placeholders use the
 * {0}, {1} style inherited from v7. Missing keys fall back to English, then to
 * the key itself, so the UI never shows blank labels.
 */

import { api } from "./api.js";

const STORAGE_KEY = "filegator.language";

let current = "english";
let messages = {};
let fallback = null;
let available = [];
let selectable = true;

/** Returns the messages for a language, cached in memory. */
const cache = new Map();

async function load(lang) {
  if (cache.has(lang)) {
    return cache.get(lang);
  }
  const loaded = await api.messages(lang);
  cache.set(lang, loaded);
  return loaded;
}

/** Initialises the i18n system and returns the active language code. */
export async function init() {
  const info = await api.languages();
  available = info.languages || [];
  selectable = info.selectable !== false;

  const stored = localStorage.getItem(STORAGE_KEY);
  const preferred = selectable && stored ? stored : info.default;
  current = available.includes(preferred) ? preferred : info.default;

  messages = await load(current);
  if (current !== "english") {
    fallback = await load("english");
  }
  return current;
}

/** Returns the languages the user may choose from. */
export function languages() {
  return available;
}

/** Reports whether users may override the default language. */
export function canSelectLanguage() {
  return selectable;
}

/** Returns the active language code. */
export function language() {
  return current;
}

/** Switches language and remembers the choice. */
export async function setLanguage(lang) {
  if (!available.includes(lang)) {
    return;
  }
  current = lang;
  messages = await load(lang);
  localStorage.setItem(STORAGE_KEY, lang);
}

/**
 * Translates key, substituting positional arguments.
 * Returns the key itself when no translation exists.
 */
export function t(key, ...args) {
  let template = messages[key];
  if (template === undefined && fallback) {
    template = fallback[key];
  }
  if (template === undefined) {
    template = key;
  }
  return template.replace(/\{(\d+)\}/g, (match, index) => {
    const value = args[Number(index)];
    return value === undefined ? match : String(value);
  });
}

/** Forces a re-render of every element carrying data-i18n attributes. */
export function applyTo(root = document) {
  for (const el of root.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.dataset.i18n);
  }
  for (const el of root.querySelectorAll("[data-i18n-placeholder]")) {
    el.placeholder = t(el.dataset.i18nPlaceholder);
  }
  for (const el of root.querySelectorAll("[data-i18n-title]")) {
    el.title = t(el.dataset.i18nTitle);
  }
}
