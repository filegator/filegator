/**
 * JSON API client for FileGator v8.
 *
 * Responses use the envelope {"data": ...} on success and
 * {"error": {"message", "code"}} on failure. This module unwraps the data and
 * throws ApiError with the message, code and HTTP status.
 */

export class ApiError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** Header carrying the double-submit CSRF token. */
const CSRF_HEADER = "X-CSRF-Token";
const CSRF_COOKIE = "filegator_csrf";

/** Returns the CSRF token from its cookie, or an empty string. */
export function csrfToken() {
  for (const part of document.cookie.split(";")) {
    const [name, ...rest] = part.trim().split("=");
    if (name === CSRF_COOKIE) {
      return decodeURIComponent(rest.join("="));
    }
  }
  return "";
}

/**
 * Turns an application path into a URL under the mount point.
 *
 * The app may be served at "/browser" rather than "/", so every request must
 * carry the prefix. It is read once from the <base> element the server fills
 * in, which means one place knows where the app lives. An absolute URL (one
 * with a scheme) is returned untouched.
 */
export function url(path) {
  if (/^[a-z]+:\/\//i.test(path) || path.startsWith("//")) {
    return path;
  }
  const base = document.baseURI.replace(/\/$/, "");
  return `${base}/${String(path).replace(/^\//, "")}`;
}

async function request(path, { method = "GET", body, signal } = {}) {
  const target = url(path);
  const headers = { Accept: "application/json" };
  const init = {
    method,
    credentials: "same-origin",
    headers,
    signal,
  };

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  if (method !== "GET") {
    const token = csrfToken();
    if (token) {
      headers[CSRF_HEADER] = token;
    }
  }

  let response;
  try {
    response = await fetch(target, init);
  } catch (err) {
    if (err.name === "AbortError") {
      throw err;
    }
    throw new ApiError("Cannot reach the server", 0, "network");
  }

  if (response.status === 204) {
    return null;
  }

  let payload = null;
  const text = await response.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = null;
    }
  }

  if (!response.ok) {
    const error = payload && payload.error;
    throw new ApiError(
      (error && error.message) || response.statusText || "Request failed",
      response.status,
      (error && error.code) || "unknown"
    );
  }

  return payload ? payload.data : null;
}

export const api = {
  /** Returns the current user, or throws 401. */
  me: (signal) => request(url("/api/auth/me"), { signal }),

  /** Logs in and returns the user. */
  login: (username, password) =>
    request(url("/api/auth/login"), {
      method: "POST",
      body: { username, password },
    }),

  /** Clears the session. */
  logout: () => request(url("/api/auth/logout"), { method: "POST" }),

  /** Changes the caller's own password. */
  changePassword: (currentPassword, newPassword) =>
    request(url("/api/auth/password"), {
      method: "POST",
      body: { current_password: currentPassword, new_password: newPassword },
    }),

  /** Lists a directory. */
  list: (path, signal) =>
    request(url(`/api/files?path=${encodeURIComponent(path)}`), { signal }),

  /** Returns one entry. */
  stat: (path) =>
    request(url(`/api/files/stat?path=${encodeURIComponent(path)}`)),

  /** Creates a directory. */
  mkdir: (parent, name, onCollision) =>
    request(url("/api/files/mkdir"), { method: "POST", body: { parent, name, on_collision: onCollision } }),

  /** Creates an empty file. */
  create: (parent, name, onCollision) =>
    request(url("/api/files/create"), { method: "POST", body: { parent, name, on_collision: onCollision } }),

  /** Reports how many chunks of an upload the server already has. */
  uploadStatus: ({ filename, identifier, totalChunks }) => {
    const params = new URLSearchParams({
      resumableFilename: filename,
      resumableIdentifier: identifier,
      resumableTotalChunks: String(totalChunks),
      resumableChunkNumber: "1",
    });
    return request(url(`/api/files/upload?${params}`));
  },

  /** Discards the staged chunks of an abandoned upload. */
  cancelUpload: ({ filename, identifier }) =>
    request(url("/api/files/upload/cancel"), {
      method: "POST",
      body: { filename, identifier },
    }),

  /** Renames or moves a single entry. */
  rename: (from, to, onCollision) =>
    request(url("/api/files/rename"), { method: "POST", body: { from, to, on_collision: onCollision } }),

  /** Returns the text of an editable file. */
  content: (path) =>
    request(url(`/api/files/content?path=${encodeURIComponent(path)}`)),

  /** Writes text into an editable file. */
  save: (path, content) =>
    request(url("/api/files/save"), { method: "POST", body: { path, content } }),

  /** Moves entries into a target folder. */
  move: (paths, target, onCollision) =>
    request(url("/api/files/move"), {
      method: "POST",
      body: { paths, target, on_collision: onCollision },
    }),

  /** Copies entries into a target folder. */
  copy: (paths, target, onCollision) =>
    request(url("/api/files/copy"), {
      method: "POST",
      body: { paths, target, on_collision: onCollision },
    }),

  /** Changes permission bits, optionally through a folder. */
  chmod: (paths, mode, recursive) =>
    request(url("/api/files/chmod"), {
      method: "POST",
      body: { paths, mode, recursive: recursive || "" },
    }),

  /** Deletes entries permanently. */
  remove: (paths) =>
    request(url("/api/files/delete"), { method: "POST", body: { paths } }),

  /** Searches below path for entries whose name contains q. */
  search: (q, path, signal) => {
    const params = new URLSearchParams({ q, path: path || "/" });
    return request(url(`/api/files/search?${params}`), { signal });
  },

  /** Creates a zip of the given paths inside destination. */
  zip: (paths, destination, name) =>
    request(url("/api/files/zip"), { method: "POST", body: { paths, destination, name } }),

  /** Extracts an archive into destination, in a folder named after it. */
  unzip: (path, destination, folder) =>
    request(url("/api/files/unzip"), {
      method: "POST",
      body: { path, destination, folder: folder || "" },
    }),

  /** Returns the URL for downloading several paths as one zip. */
  batchUrl: (paths) => {
    const params = new URLSearchParams();
    for (const p of paths) {
      params.append("path", p);
    }
    return url(`/api/files/batch?${params}`);
  },

  /** Returns the URL for downloading a file. */
  downloadUrl: (path) => url(`/api/files/download?path=${encodeURIComponent(path)}`),

  /**
   * Opens a download URL in a new tab, leaving the file manager where it is.
   *
   * v7 opened every download with window.open(..., '_blank') and v8 follows it
   * (2026-10-26). Navigating in place replaced the application with the file:
   * an inline PDF took over the tab and Back could re-trigger the download,
   * and an attachment left a blank page behind. A named tab ("filegator-
   * download") is reused, so choosing five files in a row does not leave five
   * tabs; its odd relative name is browser-standard for "the same detached
   * tab".
   */
  openDownload: (target) => {
    const tab = window.open(target, "filegator-download");
    if (tab) {
      tab.focus();
    }
  },

  /** Returns available languages and the configured default. */
  languages: () => request(url("/api/i18n")),

  /** Returns the messages for one language. */
  messages: (lang) => request(url(`/api/i18n/${encodeURIComponent(lang)}`)),

  /** Returns the public frontend configuration. */
  config: () => request(url("/api/config")),

  /** Returns what the auth provider and storage backend support. */
  capabilities: () => request(url("/api/capabilities")),

  // --- administration ---

  /** Lists directories anywhere on the server (storage root picker). */
  adminBrowse: (path) =>
    request(url(`/api/admin/browse?path=${encodeURIComponent(path)}`)),

  /** Lists directories inside the storage root only (home folder picker). */
  adminBrowseStorage: (path) =>
    request(url(`/api/admin/storage/browse?path=${encodeURIComponent(path)}`)),

  /** Returns the active storage root. */
  adminRoot: () => request(url("/api/admin/root")),

  /** Returns the auth provider settings. */
  adminAuthConfig: () => request(url("/api/admin/auth")),

  /** Switches the auth provider. */
  adminSaveAuthConfig: (cfg) =>
    request(url("/api/admin/auth"), { method: "POST", body: cfg }),

  /** Returns the storage backend settings. */
  adminStorageConfig: () => request(url("/api/admin/storage")),

  /** Switches the storage backend. */
  adminSaveStorageConfig: (cfg) =>
    request(url("/api/admin/storage"), { method: "POST", body: cfg }),

  /** Returns conditions the administrator should know about. */
  adminStatus: () => request(url("/api/admin/status")),

  /** Returns the editable settings. */
  adminSettings: () => request(url("/api/admin/settings")),

  /** Saves the editable settings. */
  adminSaveSettings: (settings) =>
    request(url("/api/admin/settings"), { method: "POST", body: settings }),

  /** Changes the storage root. */
  adminSetRoot: (path) =>
    request(url("/api/admin/root"), { method: "POST", body: { path } }),

  /** Lists all users. */
  adminUsers: () => request(url("/api/admin/users")),

  /** Creates a user. */
  adminCreateUser: (user) =>
    request(url("/api/admin/users"), { method: "POST", body: user }),

  /** Updates a user; an empty password leaves it unchanged. */
  adminUpdateUser: (username, user) =>
    request(url(`/api/admin/users/${username}`), { method: "POST", body: user }),

  /** Deletes a user. */
  adminDeleteUser: (username) =>
    request(url(`/api/admin/users/${username}/delete`), { method: "POST" }),

  /**
   * Sets another user's password.
   *
   * The administrator's override: no current password is needed, and it works
   * even when users may not change their own.
   */
  adminSetPassword: (username, password) =>
    request(url(`/api/admin/users/${username}/password`), {
      method: "POST",
      body: { password },
    }),
};
