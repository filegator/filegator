/**
 * Formatting helpers shared by the views.
 */

/** Formats a byte count using binary units, e.g. "1.5 MB". */
export function formatSize(bytes) {
  if (bytes === 0) {
    return "0 B";
  }
  if (!bytes || bytes < 0) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, exp);
  const digits = exp === 0 || value >= 100 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[exp]}`;
}

/** Returns the final segment of a path, or "/" for the root. */
export function baseName(path) {
  if (!path || path === "/") {
    return "/";
  }
  const trimmed = path.replace(/\/+$/, "");
  const index = trimmed.lastIndexOf("/");
  return index === -1 ? trimmed : trimmed.slice(index + 1);
}

/**
 * Splits a path into breadcrumb segments: [{name, path}, ...].
 *
 * The root segment has no name: "Home" is a translated label, added by the
 * view that renders the breadcrumb, so this helper stays free of i18n.
 */
export function breadcrumbSegments(path) {
  const segments = [{ name: "/", path: "/" }];
  const parts = (path || "/").split("/").filter(Boolean);
  let current = "";
  for (const part of parts) {
    current += `/${part}`;
    segments.push({ name: part, path: current });
  }
  return segments;
}

/** Chooses an icon for an entry. */
export function iconFor(entry) {
  if (entry.type === "dir") {
    return "📁";
  }
  const ext = entry.name.toLowerCase().split(".").pop();
  switch (ext) {
    case "png":
    case "jpg":
    case "jpeg":
    case "gif":
    case "webp":
    case "svg":
      return "🖼";
    case "pdf":
      return "📕";
    case "zip":
    case "gz":
    case "tar":
    case "7z":
    case "rar":
      return "🗜";
    case "mp4":
    case "mkv":
    case "webm":
    case "avi":
    case "mov":
      return "🎬";
    case "mp3":
    case "wav":
    case "flac":
    case "ogg":
      return "🎵";
    case "txt":
    case "md":
    case "log":
      return "📄";
    case "json":
    case "js":
    case "ts":
    case "go":
    case "php":
    case "css":
    case "html":
      return "⌨";
    default:
      return "📄";
  }
}
