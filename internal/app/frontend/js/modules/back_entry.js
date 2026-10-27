/**
 * The ".." row that leads back to the parent folder.
 *
 * v7 put a synthetic "back" entry at the top of every listing except the
 * storage root. It is a navigation control, not a file: it has no size, no
 * time, no permissions and no actions, cannot be selected or sorted among the
 * real entries, and clicking it goes up one level.
 *
 * It is built here rather than by the server so the API keeps returning only
 * real entries.
 */

/** The identifier used for the back row's type. */
export const BACK = "back";

/**
 * Returns a back entry for a listing, or null at the root.
 *
 * parent - the parent path the API reported, or null at the root.
 */
export function backEntry(parent) {
  if (!parent) {
    return null;
  }
  return {
    name: "..",
    path: parent,
    type: BACK,
    size: 0,
    modified: "",
    mode: "",
  };
}

/** Reports whether an entry is the back row. */
export function isBack(entry) {
  return Boolean(entry) && entry.type === BACK;
}

/**
 * Returns the parent path of a folder path, or null for "/".
 *
 * Only used when a listing arrives without one: the server is authoritative.
 */
export function parentOf(path) {
  const trimmed = String(path || "").replace(/\/+$/, "");
  if (trimmed === "" || trimmed === "/") {
    return null;
  }
  const cut = trimmed.lastIndexOf("/");
  return cut <= 0 ? "/" : trimmed.slice(0, cut);
}
