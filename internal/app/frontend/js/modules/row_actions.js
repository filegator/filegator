/**
 * What a user may do with one entry, and what clicking its name does.
 *
 * Which controls appear depends on the entry type, the user's permissions and
 * the file extension. Those rules were tangled into the row renderer, so this
 * module returns plain data that can be checked without a DOM.
 */

/** Archive extensions v7 offered to extract. */
const ARCHIVE_EXTENSIONS = [".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar"];

/** Reports whether a name looks like an archive that can be extracted. */
export function isArchive(name) {
  const lower = String(name || "").toLowerCase();
  return ARCHIVE_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

/**
 * Returns the actions shown at the end of a row, in display order.
 *
 * options:
 *   entry      - the listing entry
 *   can        - (permission) => boolean
 *   isEditable - (name) => boolean, from frontend_config.editable
 *   isImage    - (name) => boolean
 *
 * Each action is { id, label, icon }. An empty list means the user may only
 * look at the entry.
 */
export function rowActions({ entry, can, isEditable, isImage }) {
  const actions = [];
  const isFile = entry.type === "file";

  // Edit and View are alternatives: a text file is edited, a picture viewed.
  if (isFile && isEditable(entry.name)) {
    actions.push({ id: "edit", label: "Edit", icon: "📝" });
  } else if (isFile && isImage(entry.name) && can("download")) {
    actions.push({ id: "view", label: "View", icon: "👁" });
  }

  // The rest follows v7's menu order: Download, View, Copy, Move, Rename,
  // Unzip/Zip, Permissions, Delete.
  if (isFile && can("download")) {
    actions.push({ id: "download", label: "Download", icon: "⬇" });
  }
  if (can("write")) {
    actions.push({ id: "copy", label: "Copy", icon: "⧉" });
    actions.push({ id: "move", label: "Move", icon: "↪" });
    actions.push({ id: "rename", label: "Rename", icon: "✎" });
  }
  if (can("write") && can("zip")) {
    // One entry, two jobs: an archive is extracted, anything else compressed.
    actions.push(
      isArchive(entry.name)
        ? { id: "unzip", label: "Unzip", icon: "🗜" }
        : { id: "zip", label: "Zip", icon: "🗜" }
    );
  }
  if (can("write") && can("chmod")) {
    // The octal mode appears here rather than in a table column: it is only
    // useful next to the action that changes it, and a whole column for three
    // digits cost width the name needed. "Mode (644)" reads as one item, so
    // the style is not a separate thing to discover.
    const mode = String(entry.mode || "").trim();
    actions.push({
      id: "chmod",
      label: mode ? `Mode (${mode})` : "Mode",
      icon: "🔒",
    });
  }
  if (can("write")) {
    actions.push({ id: "delete", label: "Delete", icon: "🗑", danger: true });
  }
  return actions;
}

/**
 * Returns what clicking the entry's name should do.
 *
 *   "browse"   - a folder: navigate into it
 *   "edit"     - an editable text file: open the editor
 *   "view"     - a picture: open the gallery
 *   "download" - anything else the user may download
 *   "none"     - the user may not open it; the name is plain text
 *
 * The order matters and matches v7's expectations: editing beats viewing
 * beats downloading, because a file manager's job is to open the thing.
 */
export function nameAction({ entry, can, isEditable, isImage }) {
  if (entry.type === "dir") {
    return "browse";
  }
  if (isEditable(entry.name)) {
    // Editing needs the editor endpoint, which requires read and write. A
    // read-only user must not be offered a link that will be refused.
    return can("read") && can("write") ? "edit" : can("download") ? "download" : "none";
  }
  if (isImage(entry.name) && can("download")) {
    return "view";
  }
  return can("download") ? "download" : "none";
}
