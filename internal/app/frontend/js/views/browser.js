/**
 * File browser: breadcrumb, listing, navigation and file actions.
 */

import { api, ApiError } from "../modules/api.js";
import { clear, create, el, hide, setVisible, show } from "../modules/dom.js";
import { confirmAction, promptText } from "../modules/dialog.js";
import {
  baseName,
  breadcrumbSegments,
  formatSize,
  iconFor,
} from "../modules/format.js";
import { DEFAULT_DATE_FORMAT, formatDate } from "../modules/datetime.js";
import { t } from "../modules/i18n.js";
import { applyLogo } from "../modules/logo.js";
import { createExpandableSearch, createHeaderSearch } from "../modules/header_search.js";
import { createUploadPanel } from "../modules/upload_panel.js";

/** Returns the folder containing a path. */
function parentOfPath(p) {
  const cut = (p || "").lastIndexOf("/");
  return cut <= 0 ? "/" : p.slice(0, cut);
}
import { askCollision, KEEP_BOTH, REPLACE } from "../modules/collision.js";
import { CollisionSession } from "../modules/collisions.js";
import { partition } from "../modules/filter.js";
import { nextDirection, sortEntries } from "../modules/sort.js";
import { createTreeView, openFolderTree } from "../modules/tree.js";
import { backEntry, isBack, parentOf } from "../modules/back_entry.js";
import { openPermissions } from "../modules/permissions_dialog.js";
import { nameAction, rowActions } from "../modules/row_actions.js";
import { closeRowMenu, onRowMenuChange, openRowMenu } from "../modules/row_menu.js";
import { openEditor } from "./editor.js";
import { isImage, openGallery } from "./gallery.js";

/** Rejects names that could escape the current folder. */
function invalidName(name) {
  const trimmed = (name || "").trim();
  if (!trimmed) {
    return t("Required");
  }
  if (trimmed === "." || trimmed === ".." || /[/\\]/.test(trimmed)) {
    return t("File name is not allowed");
  }
  return null;
}

/**
 * Creates the browser view.
 *
 * callbacks:
 *   onLogout(cancelled) - session ended
 */
export function createBrowser({ onLogout }) {
  const view = el("browser-view");
  const list = el("file-list");
  const crumb = el("breadcrumb");
  const empty = el("empty-message");
  const toast = el("file-toast");
  const userLabel = el("current-user");
  const btnNew = el("btn-new");
  const btnUpload = el("btn-upload");
  const selectionActions = el("selection-actions");
  const selectionCount = el("selection-count");
  const listingCount = el("listing-count");
  // Browsing controls, hidden while a selection is active so the toolbar
  // switches in place rather than growing.
  const browsingActions = [
    el("btn-new"),
    el("btn-upload"),
    el("btn-tree"),
    el("btn-refresh"),
  ];
  const btnBatchMove = el("btn-batch-move");
  const btnBatchCopy = el("btn-batch-copy");
  const btnZip = el("btn-zip");
  const btnUnzip = el("btn-unzip");
  const btnBatch = el("btn-batch-download");
  const btnBatchDelete = el("btn-batch-delete");
  const selectAll = el("select-all");
  let uploadPanel = null;

  let currentPath = "/";
  let user = null;
  let controller = null;
  let toastTimer = null;
  let config = {};

  /** The date pattern the administrator chose, or the default. */
  function dateFormat() {
    return (config && config.date_format) || DEFAULT_DATE_FORMAT;
  }
  // Selected paths for the batch actions, keyed by path.
  const selected = new Set();
  let entriesByPath = new Map();
  // The listing as the server sent it, before filtering, so the "show hidden"
  // toggle can redraw without another request.
  let lastListing = [];
  let hiddenEntriesCount = 0;
  // Remembered across navigation: a user who asked to see hidden files in one
  // folder usually wants them in the next.
  let showHidden = false;
  // The ".." row for the current listing, or null at the storage root.
  let backRow = null;
  // The left panel; created once and re-shown on navigation.
  let treeView = null;
  // The breakpoint at which the panel becomes a drawer. Queried on each call
  // rather than captured once, so resizing the window takes effect without a
  // reload and nothing depends on a MediaQueryList staying current.
  const DRAWER_QUERY = "(max-width: 720px)";
  // Whether the user wants the panel open. Remembered across sessions so a
  // user who hid it does not have to hide it again on every visit.
  const TREE_PREF_KEY = "filegator.tree";
  let treeOpen = readTreePreference();
  // The permissions column alternates between "644" and "[rw-r--r--]".
  // How many entries the listing shows, for the "Selected: n of m" line.
  let visibleCount = 0;
  // Column sorting, remembered while the user browses. The server sorts
  // folders first by name; this is the user's override of that.
  let sortKey = "name";
  let sortDesc = false;
  const DEFAULT_SORT = { key: "name", desc: false };

  function showToast(message, kind = "error") {
    toast.textContent = message;
    toast.classList.toggle("is-info", kind === "info");
    show(toast);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => hide(toast), 4000);
  }

  function can(permission) {
    return Boolean(user && user.permissions && user.permissions.includes(permission));
  }

  /** Runs an action, reporting API failures as a toast. */
  async function run(action, successMessage) {
    try {
      await action();
      if (successMessage) {
        showToast(successMessage, "info");
      }
      await navigate(currentPath, { push: false });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onLogout(true);
        return;
      }
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  function renderBreadcrumb() {
    clear(crumb);
    // The first crumb is the repository root; every other segment is a real
    // folder name and is shown as stored. "Home" is a label, so it comes from
    // the translations rather than from the path helper.
    const segments = breadcrumbSegments(currentPath);
    segments.forEach((segment, index) => {
      const isLast = index === segments.length - 1;
      const link = create("a", {
        text: segment.path === "/" ? t("Home") : segment.name,
        attrs: {
          href: segment.path === "/" ? "#/" : `#${segment.path}`,
          "data-path": segment.path,
          ...(isLast ? { "aria-current": "page" } : {}),
        },
      });
      link.addEventListener("click", (event) => {
        event.preventDefault();
        navigate(segment.path);
      });
      crumb.append(link);
      if (!isLast) {
        crumb.append(create("span", { className: "sep", text: "/" }));
      }
    });
  }

  /**
   * Builds the "⋯" button for a row.
   *
   * One menu per row replaced the inline action icons: the class differs so a
   * test can tell a menu button from the old icon buttons, and CSS styles it as
   * a menu trigger rather than an icon.
   */
  function rowAction(label, text, handler) {
    const button = create("button", {
      className: "row-menu-button",
      text,
      attrs: { type: "button", title: label, "aria-label": label },
    });
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      handler();
    });
    return button;
  }

  /** Updates the selection bar from the current selection. */
  function syncSelection() {
    const count = selected.size;
    const total = entriesByPath.size;
    selectionCount.textContent = t("Selected: {0}", count);
    // The same count sits under the table, where v7 showed it. It stays
    // visible with nothing selected, so the line never appears and disappears.
    listingCount.textContent = t("Selected: {0} of {1}", count, visibleCount);

    // The toolbar swaps to selection actions instead of gaining a row above the
    // table: selecting must not move anything, only change what the buttons do.
    const active = count > 0;
    selectionActions.classList.toggle("hidden", !active);
    for (const control of browsingActions) {
      setVisible(control, !active && control.dataset.allowed !== "false");
    }
    // With a selection, a row's menu would act on that row while the selection
    // bar suggests otherwise; v7 disabled the row buttons for the same reason.
    view.classList.toggle("is-selecting", active);

    // Unzip only makes sense for a single .zip file.
    const paths = [...selected];
    const only = paths.length === 1 ? entriesByPath.get(paths[0]) : null;
    const isZip = Boolean(only) && only.type === "file" &&
      only.name.toLowerCase().endsWith(".zip");
    if (can("zip")) {
      setVisible(btnUnzip, isZip);
    }

    // Row checkboxes update themselves on change; only "select all" needs
    // recalculating here.
    selectAll.checked = count > 0 && count === total;
    selectAll.indeterminate = count > 0 && count < total;
  }

  /** Toggles one path in the selection. */
  function toggleSelection(path, checked) {
    if (checked) {
      selected.add(path);
    } else {
      selected.delete(path);
    }
    syncSelection();
  }

  /** Creates an archive of the selection in the current folder. */
  async function zipSelection() {
    if (!selected.size) {
      return;
    }
    const name = await promptText({
      title: t("Zip"),
      label: t("New name"),
      value: config.default_archive_name || "archive.zip",
      confirmText: t("Create"),
      validate: (value) => invalidName(value),
    });
    if (name === null) {
      return;
    }
    try {
      await api.zip([...selected], currentPath, name.trim());
      showToast(t("Done"), "info");
      selected.clear();
      await navigate(currentPath, { push: false });
    } catch (err) {
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  /** Extracts the selected archive. */
  async function unzipSelection() {
    const path = [...selected][0];
    if (!path) {
      return;
    }
    try {
      await api.unzip(path, currentPath);
      showToast(t("Done"), "info");
      selected.clear();
      await navigate(currentPath, { push: false });
    } catch (err) {
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  /** Deletes every selected entry after one confirmation. */
  async function deleteSelection() {
    const paths = [...selected];
    if (!paths.length) {
      return;
    }

    const names = paths.map((p) => baseName(p));
    const summary = names.length <= 3
      ? names.join(", ")
      : `${names.slice(0, 3).join(", ")} …`;
    const confirmed = await confirmAction({
      title: t("Delete"),
      message: `${t("Are you sure you want to do this?")} (${paths.length}) ${summary}`,
      confirmText: t("Delete"),
      danger: true,
    });
    if (!confirmed) {
      return;
    }

    try {
      await api.remove(paths);
      selected.clear();
      showToast(t("Done"), "info");
      await navigate(currentPath, { push: false });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onLogout(true);
        return;
      }
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  /** Downloads the selection as one zip, streamed by the server. */
  function downloadSelection() {
    if (!selected.size) {
      return;
    }
    // The server streams the archive; it opens in a new tab so the listing
    // stays put, as every other download does.
    api.openDownload(api.batchUrl([...selected]));
  }

  function renderEntry(entry) {
    const back = isBack(entry);
    const row = create("tr", { className: back ? "is-back" : "" }, []);

    // The back row is a navigation control: nothing to select, nothing to
    // change, no size or time.
    const selectCell = create("td", { className: "col-select" }, []);
    if (!back) {
      const checkbox = create("input", {
        className: "checkbox row-check",
        attrs: { type: "checkbox", "data-path": entry.path, "aria-label": entry.name },
      });
      checkbox.checked = selected.has(entry.path);
      checkbox.addEventListener("change", () => toggleSelection(entry.path, checkbox.checked));
      selectCell.append(checkbox);
    }

    const nameCell = create("td", { className: "col-name" }, []);
    if (back) {
      // No text here: the name is a child, and passing both printed it twice
      // (".. 📁 ..").
      const link = create("a", {
        className: "file-name is-dir is-back",
        attrs: { href: `#${entry.path}`, title: t("Parent folder") },
      }, [
        create("span", { className: "file-icon", text: iconFor({ type: "dir", name: entry.name }) }),
        create("span", { text: entry.name }),
      ]);
      link.addEventListener("click", (event) => {
        event.preventDefault();
        navigate(entry.path);
      });
      nameCell.append(link);
    }
    const link = back ? null : create(
      "a",
      {
        className: entry.type === "dir" ? "file-name is-dir" : "file-name",
        attrs:
          entry.type === "dir"
            ? { href: `#${entry.path}` }
            : { href: api.downloadUrl(entry.path), download: entry.name },
      },
      [
        create("span", { className: "file-icon", text: iconFor(entry) }),
        create("span", { text: entry.name }),
      ]
    );
    // What the name does and where it points is decided once, in
    // row_actions.js, so the rules are testable and renderEntry only wires it.
    const action = back ? "none" : nameAction({
      entry,
      can,
      isEditable: isEditable,
      isImage,
    });
    const handlers = {
      browse: () => navigate(entry.path),
      edit: () => editEntry(entry),
      view: () => viewEntry(entry),
      download: () => {
        // A new tab, so the file manager is not replaced by the file.
        api.openDownload(api.downloadUrl(entry.path));
      },
    };
    if (back) {
      // Already handled above.
    } else if (action === "none") {
      // The name would answer with a JSON error instead of the file, which
      // looks like a broken app rather than a missing permission, so it stops
      // being a link and the reason moves into the tooltip.
      link.removeAttribute("href");
      link.removeAttribute("download");
      link.classList.add("is-locked");
      link.title = t("Downloading is not allowed");
    } else if (handlers[action]) {
      link.addEventListener("click", (event) => {
        event.preventDefault();
        handlers[action]();
      });
      if (action === "browse") {
        link.title = "";
      } else {
        link.title = t(action === "edit" ? "Edit" : action === "view" ? "View" : "Download");
      }
    }
    if (link) {
      nameCell.append(link);
    }

    // v7 printed the word "Folder" where a size would go, and showed no time
    // for the back row. The pattern is the administrator's date_format.
    const size = entry.type === "dir" || back ? t("Folder") : formatSize(entry.size);
    const date = back ? "" : formatDate(entry.modified, dateFormat());

    // One menu per row, as v7 had: the actions are named there instead of
    // filling every row with icons.
    const actions = create("td", { className: "col-actions" }, []);
    const specs = back ? [] : rowActions({ entry, can, isEditable: isEditable, isImage });
    if (specs.length) {
      const menuButton = rowAction(t("Actions"), "⋯", () => {});
      menuButton.setAttribute("aria-haspopup", "menu");
      menuButton.setAttribute("aria-expanded", "false");
      menuButton.addEventListener("click", (event) => {
        event.preventDefault();
        openRowMenu({
          anchor: menuButton,
          label: t("Actions for {0}", entry.name),
          actions: specs,
          onChoose: (id) => runRowAction(id, entry, specs),
        });
      });
      actions.append(menuButton);

      // Right-click anywhere in the row opens the same menu at the pointer,
      // as v7 did. The browser's own menu is replaced, which is what makes it
      // quick to reach Rename or Delete without aiming at the ⋯ button.
      row.addEventListener("contextmenu", (event) => {
        event.preventDefault();
        openRowMenu({
          anchor: menuButton,
          at: { x: event.clientX, y: event.clientY },
          label: t("Actions for {0}", entry.name),
          actions: specs,
          onChoose: (id) => runRowAction(id, entry, specs),
        });
      });
    }

    row.append(
      selectCell,
      nameCell,
      create("td", { className: "col-size", text: size }),
      create("td", { className: "col-time", text: date }),
      actions,
      create("td", {
        className: "col-meta",
        text: [size, date].filter(Boolean).join(" · "),
      })
    );

    // Only the name cell opens the entry.
    //
    // The whole row used to: aiming beside a short name did nothing, so the
    // handler covered every cell. That made the space around the checkbox open
    // the file, so missing the checkbox by a few pixels downloaded it instead
    // of selecting it — a destructive surprise next to a harmless action.
    // The name cell is the target now; the row keeps its right-click menu.
    const rowActionFn = back ? () => navigate(entry.path) : handlers[action];
    if (rowActionFn) {
      const open = rowActionFn;
      nameCell.classList.add("is-clickable");
      nameCell.addEventListener("click", (event) => {
        // The name link handles its own clicks and keeps its href, so
        // middle-click and copy-link still work.
        if (event.target.closest("a")) {
          return;
        }
        if (window.getSelection && String(window.getSelection())) {
          // The user is selecting text, not opening the row.
          return;
        }
        open();
      });
    }
    return row;
  }

  /** Reports whether a file may be opened in the editor. */
  function isEditable(name) {
    const ext = (name.includes(".") ? name.slice(name.lastIndexOf(".")) : "").toLowerCase();
    if (!ext) {
      return false;
    }
    const list = (config && config.editable) || [];
    return list.some((allowed) => String(allowed).toLowerCase() === ext);
  }

  /** Opens the editor for one file. */
  /**
   * Opens the image gallery on one entry.
   *
   * The strip holds the images in the current folder, so the user can flip
   * through them the way v7's gallery allowed.
   */
  function viewEntry(entry) {
    const images = [...entriesByPath.values()]
      .filter((e) => e.type === "file" && isImage(e.name))
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));
    openGallery(images, entry);
  }

  async function editEntry(entry) {
    try {
      await openEditor(entry.path, {
        onSaved: () => navigate(currentPath, { push: false }),
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onLogout(true);
        return;
      }
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  async function renameEntry(entry) {
    const name = await promptText({
      title: t("Rename"),
      label: t("New name"),
      value: entry.name,
      confirmText: t("Rename"),
      validate: invalidName,
    });
    if (name === null || name.trim() === entry.name) {
      return;
    }
    const target = `${currentPath === "/" ? "" : currentPath}/${name.trim()}`;
    // A rename onto a taken name asks rather than overwriting: Replace is
    // destructive, Keep both renames the incoming file.
    run(
      () =>
        new CollisionSession().run(entry.name, (policy) =>
          api.rename(entry.path, target, policy)
        ),
      t("Done")
    );
  }

  /**
   * Runs one action from a row's menu.
   *
   * Kept as a table rather than a chain of ifs so the menu's ids and the
   * handlers cannot drift apart unnoticed.
   */
  function runRowAction(id, entry, specs) {
    const handlers = {
      edit: () => editEntry(entry),
      view: () => viewEntry(entry),
      download: () => api.openDownload(api.downloadUrl(entry.path)),
      copy: () => transferEntry(entry, "copy"),
      move: () => transferEntry(entry, "move"),
      rename: () => renameEntry(entry),
      zip: () => zipEntry(entry),
      unzip: () => unzipEntry(entry),
      chmod: () => changePermissions(entry),
      delete: () => deleteEntry(entry),
    };
    if (handlers[id]) {
      handlers[id]();
    } else if (specs) {
      // A menu entry with no handler is a bug; say so rather than doing
      // nothing when the user clicks it.
      showToast(t("That action is not available"));
    }
  }

  /**
   * Copies or moves everything selected into a folder chosen from the tree.
   *
   * The toolbar actions work on the whole selection; a single row uses its own
   * menu, which arrives here with one path.
   */
  async function transferSelection(mode) {
    const paths = [...selected];
    if (!paths.length) {
      return;
    }
    const label = paths.length === 1 ? baseName(paths[0]) : `${paths.length} ${t("items")}`;
    await transfer(paths, label, mode);
  }

  /**
   * Copies or moves an entry into a folder chosen from the tree.
   *
   * v7 reused its folder picker for this, which is why the tree exists. A
   * collision is handled the same way as elsewhere: the server refuses and the
   * prompt asks.
   */
  async function transferEntry(entry, mode) {
    await transfer([entry.path], entry.name, mode);
  }

  /** Shared by the toolbar and the row menu. */
  async function transfer(paths, label, mode) {
    openFolderTree({
      rootPath: "/",
      current: currentPath,
      onSelect: async (node) => {
        try {
          await new CollisionSession().run(label, (policy) =>
            mode === "copy"
              ? api.copy(paths, node.path, policy)
              : api.move(paths, node.path, policy)
          );
          selected.clear();
          for (const box of list.querySelectorAll(".row-check")) {
            box.checked = false;
          }
          showToast(t("Done"), "info");
          await navigate(currentPath, { push: false });
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            onLogout(true);
            return;
          }
          showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
        }
      },
    });
  }

  /** Zips one entry in place. */
  async function zipEntry(entry) {
    const name = await promptText({
      title: t("Zip"),
      label: t("New name"),
      value: defaultArchiveName(entry),
      confirmText: t("Create"),
      validate: invalidName,
    });
    if (name === null) {
      return;
    }
    run(() => api.zip([entry.path], currentPath, name.trim()), t("Done"));
  }

  /** Extracts one archive into a subfolder. */
  async function unzipEntry(entry) {
    run(() => api.unzip(entry.path, currentPath), t("Done"));
  }

  /**
   * Suggests an archive name for one entry.
   *
   * "docs" becomes "docs.zip", and "report.txt" becomes "report.zip" — the
   * extension is replaced, not appended, so it does not read as report.txt.zip.
   */
  function defaultArchiveName(entry) {
    const base = entry.type === "dir" ? entry.name : entry.name.replace(/\.[^.]+$/, "");
    return `${base}.zip`;
  }

  /** Opens the permission dialog and saves the chosen mode. */
  async function changePermissions(entry) {
    const result = await openPermissions({
      name: entry.name,
      mode: entry.mode,
      isDir: entry.type === "dir",
    });
    if (!result) {
      return;
    }
    run(() => api.chmod([entry.path], result.mode, result.recursive), t("Done"));
  }

  async function deleteEntry(entry) {
    const confirmed = await confirmAction({
      title: t("Delete"),
      message: t("Are you sure you want to do this?"),
      confirmText: t("Delete"),
      danger: true,
    });
    if (!confirmed) {
      return;
    }
    run(() => api.remove([entry.path]), t("Done"));
  }

  async function newFolder() {
    const name = await promptText({
      title: `${t("New")} ${t("Folder")}`,
      label: t("New name"),
      confirmText: t("Create"),
      validate: invalidName,
    });
    if (name === null) {
      return;
    }
    run(
      () =>
        new CollisionSession({ allowReplace: false }).run(name.trim(), (policy) =>
          api.mkdir(currentPath, name.trim(), policy)
        ),
      t("Done")
    );
  }

  async function newFile() {
    const name = await promptText({
      title: `${t("New")} ${t("File")}`,
      label: t("New name"),
      confirmText: t("Create"),
      validate: invalidName,
    });
    if (name === null) {
      return;
    }
    run(
      () =>
        new CollisionSession().run(name.trim(), (policy) =>
          api.create(currentPath, name.trim(), policy)
        ),
      t("Done")
    );
  }

  function renderListing(listing) {
    // A menu anchored to a row disappears with it.
    closeRowMenu();
    clear(list);
    // ".." leads to the parent; v7 showed it at the top when a parent exists.
    backRow = backEntry(listing.parent ?? parentOf(listing.path ?? currentPath));
    // The administrator may hide entries by pattern; they appear only when the
    // user asks. Hidden entries still exist, so they are not selectable.
    lastListing = listing.entries;
    let entries = listing.entries;
    if (!showHidden) {
      entries = partition(listing.entries, config.filter_entries).visible;
    }
    // The count is always over the full listing, so the toggle can say how
    // many are hidden even while they are visible.
    const all = partition(lastListing, config.filter_entries);
    hiddenEntriesCount = all.hiddenCount;
    renderHiddenToggle();
    const visible = sortEntries(entries, sortKey, sortDesc);
    visibleCount = visible.length;
    renderSortIndicators();

    // Drop selections whose entries are no longer visible.
    entriesByPath = new Map();
    for (const entry of visible) {
      entriesByPath.set(entry.path, entry);
    }
    for (const path of [...selected]) {
      if (!entriesByPath.has(path)) {
        selected.delete(path);
      }
    }
    // "Select all" covers real entries only: the back row is navigation.

    if (!visible.length && !backRow) {
      show(empty);
      syncSelection();
      return;
    }
    hide(empty);
    const fragment = document.createDocumentFragment();
    // The back row is not part of the selection, the sort or the count.
    if (backRow) {
      fragment.append(renderEntry(backRow));
    }
    for (const entry of visible) {
      fragment.append(renderEntry(entry));
    }
    list.append(fragment);
    syncSelection();
  }

  /**
   * Marks the active sort column and direction.
   *
   * The arrows live in each header button so a screen reader announces both
   * the column and the order.
   */
  function renderSortIndicators() {
    for (const key of ["name", "size", "time"]) {
      const header = el(`sort-${key}`);
      if (!header) {
        continue;
      }
      const active = key === sortKey;
      header.classList.toggle("is-sorted", active);
      header.setAttribute(
        "aria-sort",
        active ? (sortDesc ? "descending" : "ascending") : "none"
      );
      const arrow = header.querySelector(".sort-arrow");
      if (arrow) {
        arrow.textContent = active ? (sortDesc ? "▼" : "▲") : "";
      }
    }
  }

  /** Applies a column sort, redrawing from the listing already loaded. */
  function setSort(key) {
    const next = nextDirection(key, sortKey, sortDesc);
    sortKey = key;
    sortDesc = next;
    renderListing({ entries: lastListing });
  }

  /**
   * Shows or hides the "show hidden" control.
   *
   * It appears only when a filter actually hides something, so ordinary
   * folders stay uncluttered.
   */
  function renderHiddenToggle() {
    const wrapper = el("hidden-toggle");
    const box = el("hidden-toggle-check");
    const label = el("hidden-toggle-label");
    const applicable = hiddenEntriesCount > 0;
    setVisible(wrapper, applicable || showHidden);
    label.textContent = applicable
      ? t("Show hidden ({0})", hiddenEntriesCount)
      : t("Show hidden");
    wrapper.title = label.textContent;
    box.checked = showHidden;
  }

  /**
   * Opens a folder.
   *
   * options:
   *   push - add a history entry, so the browser's Back button returns to the
   *          previous folder. False for the first render and for a hashchange,
   *          which are already history events.
   *
   * v7 pushed a route for every folder change, which is why Back walked
   * through folders instead of leaving the application.
   */
  async function navigate(path, options = {}) {
    const { push = true } = options;
    currentPath = path || "/";
    renderBreadcrumb();

    if (controller) {
      controller.abort();
    }
    controller = new AbortController();

    try {
      const listing = await api.list(currentPath, controller.signal);
      renderListing(listing);
      // The panel follows the listing. A failure here must not stop the
      // listing, which has already rendered.
      //
      // A closed panel stays closed: this runs on every navigation, and
      // opening it here is why hiding the panel never lasted a click.
      if (!treeOpen || isDrawer()) {
        // A closed panel stays closed, and a drawer never opens itself: both
        // must have the hidden class applied, not just rely on the CSS
        // transform. Leaving it to the transform meant the class and the
        // preference disagreed on a phone, so the first click "closed" a
        // drawer that was already invisible.
        closeTreePanel({ remember: false });
        if (treeOpen && treeView) {
          // A drawer opened before keeps its highlight current while closed.
          treeView.show(currentPath).catch(() => {});
        }
      } else {
        // On a wide screen the panel is part of the layout: build it with the
        // first folder, then keep its highlight on every navigation.
        openTreePanel();
        treeView.show(currentPath).catch(() => {});
      }
      const hash = currentPath === "/" ? "" : `#${currentPath}`;
      if (window.location.hash !== hash) {
        // push, not replace: replacing overwrote the entry, so Back skipped
        // the folder the user came from and left the app entirely.
        if (push) {
          history.pushState(null, "", hash || window.location.pathname);
        } else {
          history.replaceState(null, "", hash || window.location.pathname);
        }
      }
    } catch (err) {
      if (err.name === "AbortError") {
        return;
      }
      if (err instanceof ApiError && err.status === 401) {
        onLogout(true);
        return;
      }
      clear(list);
      hide(empty);
      showToast(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    }
  }

  /**
   * Uploads files into the current folder.
   *
   * The panel owns the queue, so files keep uploading while the user browses
   * elsewhere — and a second drop adds to the running queue rather than
   * replacing it, which is how v7 behaved.
   */
  function upload(fileList) {
    if (!can("upload") || !fileList || !fileList.length) {
      return;
    }
    if (!uploadPanel) {
      uploadPanel = createUploadPanel({
        chunkSize: config.upload_chunk_size,
        simultaneous: config.upload_simultaneous,
        askCollision: async (name) => {
          // Uploads may replace an existing file (the user is explicitly
          // asked), so "Replace" is offered here, unlike for folders.
          const answer = await askCollision({ name, batch: true });
          if (answer.action === REPLACE || answer.action === KEEP_BOTH) {
            if (answer.applyToAll) {
              uploadPanel.queue.collisionPolicy = answer.action;
            }
            return answer.action;
          }
          return null;
        },
        // Refresh the listing as each file lands, so it appears while the
        // rest are still uploading. Skipped when the user has navigated away.
        onStored: (_entry, stored) => {
          if (stored && parentOfPath(stored.path) === currentPath) {
            // A refresh, not a step: an upload must not fill the Back button
            // with one entry per file.
            navigate(currentPath, { push: false });
          }
        },
      });
    }
    uploadPanel.start([...fileList], currentPath);
  }

  function applyPermissions() {
    // Remembered so syncSelection can swap the toolbar without overriding what
    // the user is allowed to see.
    for (const [control, allowed] of [
      [el("btn-new"), can("write")],
      [el("btn-upload"), can("upload")],
      [el("btn-tree"), can("read")],
      [el("btn-refresh"), true],
    ]) {
      control.dataset.allowed = String(allowed);
    }
    setVisible(btnNew, can("write"));
    setVisible(btnUpload, can("upload"));
    // Searching needs the read permission; without it there is nothing to see.
    // The tree browses folders, so it needs the same permission as a listing.
    setVisible(el("btn-tree"), can("read"));
    if (!can("read")) {
      setVisible(el("tree-panel"), false);
      setVisible(el("tree-backdrop"), false);
    }
    // Zip/unzip need the zip permission; batch download needs its own.
    setVisible(btnZip, can("zip") && can("write"));
    setVisible(btnBatch, can("batchdownload"));
    setVisible(btnBatchCopy, can("write"));
    setVisible(btnBatchMove, can("write"));
    // Deleting in bulk is destructive, so it needs the same write permission
    // as the per-row delete.
    setVisible(btnBatchDelete, can("write"));
    if (!can("zip")) {
      setVisible(btnUnzip, false);
    }
  }

  /**
   * Labels the account menu button for the current session.
   *
   * A signed-in user sees their name; a guest sees "Log in", because that is
   * the only useful thing the menu can offer it. What the menu contains is
   * decided by js/modules/user_menu.js each time it opens.
   */
  function updateSessionButton() {
    const isGuest = Boolean(user && user.guest);
    const name = isGuest ? t("Log in") : user ? user.name || user.username : "";
    const button = el("btn-user-menu");
    userLabel.textContent = name;
    button.title = name;
    button.setAttribute("aria-label", `${name} — ${t("Account")}`);
  }

  /**
   * Applies the administrator's interface width.
   *
   * Set as a custom property rather than on each element, so the card and the
   * admin card both follow one value. 0 means no cap: `max-width: none`.
   */
  function applyMaxWidth(px) {
    const width = Number(px) || 0;
    document.documentElement.style.setProperty(
      "--max-width",
      width > 0 ? `${width}px` : "none"
    );
  }

  /** Applies the active language to static toolbar labels. */
  function applyLabels() {
    el("btn-new-label").textContent = t("New");
    // Upload had a hardcoded English label and no icon.
    el("btn-upload-label").textContent = t("Upload");
    btnNew.title = t("New");
    // The administration area is reached from here; "Admin" told the user
    // nothing about what the button does.
    el("btn-admin").textContent = t("Setup");
    const refresh = el("btn-refresh");
    refresh.title = t("Refresh");
    refresh.querySelector(".sr-only").textContent = t("Refresh");
    // Derived from the session rather than set here: a guest has no account to
    // sign out of, so the button opens the login form instead. Setting it in
    // two places meant this line silently overwrote the guest label.
    updateSessionButton();
    btnBatch.textContent = t("Download");
    btnBatchCopy.textContent = t("Copy");
    btnBatchMove.textContent = t("Move");
    const clear = el("btn-selection-clear");
    clear.title = t("Cancel");
    clear.querySelector(".sr-only").textContent = t("Cancel");
    btnZip.textContent = t("Zip");
    btnUnzip.textContent = t("Unzip");
    btnBatchDelete.textContent = t("Delete");
    el("empty-message").textContent = t("This folder is empty");
    el("btn-tree").title = t("Folder tree");
    el("btn-tree").querySelector(".sr-only").textContent = t("Folder tree");
    // The account menu's own labels are built each time it opens, so they pick
    // up the current language without anything to relabel here.
  }

  // Hidden file input used by the Upload button.
  const fileInput = create("input", { attrs: { type: "file", multiple: true } });
  fileInput.style.display = "none";
  document.body.append(fileInput);
  fileInput.addEventListener("change", () => {
    upload(fileInput.files);
    fileInput.value = "";
  });

  // Column headers sort the listing; clicking the active column flips it.
  for (const key of ["name", "size", "time"]) {
    const header = el(`sort-${key}`);
    if (header) {
      header.addEventListener("click", () => setSort(key));
    }
  }

  /**
   * Builds the left panel once, then reuses it.
   *
   * It follows every navigation, so the highlight always matches the listing.
   */
  function ensureTreePanel() {
    if (treeView) {
      return treeView;
    }
    const start = currentPath;
    treeView = createTreeView({
      rootPath: "/",
      current: start,
      onSelect: (node) => {
        closeTreeAfterPick();
        if (node.path !== currentPath) {
          navigate(node.path);
        }
      },
    });
    el("tree-panel-body").append(treeView.element);
    return treeView;
  }

  /**
   * Forgets everything that belonged to the previous account.
   *
   * The browser view is created once for the page and survives logout, so the
   * folder tree, the remembered listing and the remembered path would
   * otherwise be shown to whoever signs in next.
   */
  function resetSession() {
    currentPath = "/";
    lastListing = [];
    entriesByPath = new Map();
    selected.clear();
    hiddenEntriesCount = 0;
    visibleCount = 0;
    backRow = null;
    // Folders are per account, so the panel is rebuilt from the new root on
    // the next navigation. An open panel stays open: it is a layout
    // preference, not data.
    if (treeView) {
      treeView.element.remove();
      treeView = null;
    }
    clear(list);
  }

  /**
   * Reads the remembered panel preference.
   *
   * Defaults to **closed**: the listing is what the user came for, and the
   * folder panel is one click away. A stored value always wins, so someone who
   * opened it keeps it on every later load — including every navigation, which
   * used to reopen the panel because the navigation path called
   * openTreePanel unconditionally.
   */
  function readTreePreference() {
    try {
      // Closed unless the user opened it: a first-time visitor sees the
      // listing first, and the folder panel is there when they want it. A
      // stored "open" is honoured, so anyone who opened it keeps it.
      return localStorage.getItem(TREE_PREF_KEY) === "open";
    } catch {
      // Private browsing can throw on access; the panel then just follows the
      // default for this visit.
      return false;
    }
  }

  /** Records the preference. */
  function writeTreePreference(open) {
    treeOpen = open;
    try {
      localStorage.setItem(TREE_PREF_KEY, open ? "open" : "closed");
    } catch {
      // Remembering is a convenience; failing to store it must not break the
      // toggle.
    }
  }

  /** Reports whether the panel is in drawer mode (narrow screens). */
  function isDrawer() {
    return Boolean(window.matchMedia && window.matchMedia(DRAWER_QUERY).matches);
  }

  /**
   * Shows the panel.
   *
   * `remember` is false for the automatic re-open on navigation and on a
   * resize: those follow the stored preference rather than changing it.
   */
  function openTreePanel({ remember = false } = {}) {
    const started = !treeView;
    ensureTreePanel();
    setVisible(el("tree-panel"), true);
    setVisible(el("btn-tree-close"), isDrawer());
    if (isDrawer()) {
      setVisible(el("tree-backdrop"), true);
    }
    if (remember) {
      writeTreePreference(true);
    }
    // A panel built on demand has never been told where the user is: on a
    // phone the drawer is only built when it is opened, so without this it
    // showed the root alone and no subfolders.
    if (started) {
      treeView.show(currentPath).catch(() => {});
    }
  }

  /**
   * Hides the panel and remembers that.
   *
   * Also used for the drawer's own dismissal, so picking a folder from the
   * drawer does not re-open it on the next navigation: the user closed it.
   */
  function closeTreePanel({ remember = true } = {}) {
    setVisible(el("tree-panel"), false);
    setVisible(el("tree-backdrop"), false);
    if (remember) {
      writeTreePreference(false);
    }
  }

  /**
   * Hides the **drawer** after the user picks a folder in it.
   *
   * Only in drawer mode: on a wide screen the sidebar is part of the layout,
   * and hiding it here meant `navigate()` immediately showed it again — the
   * panel visibly closed and reopened on every click in the tree.
   *
   * This is navigation, not "I want the panel closed", so nothing is
   * remembered either.
   */
  function closeTreeAfterPick() {
    if (!isDrawer()) {
      return;
    }
    closeTreePanel({ remember: false });
  }

  /** The drawer's own × and backdrop: the user wants it hidden, so remember. */
  function closeTreeDrawer() {
    closeTreePanel();
  }

  // The toolbar button opens the drawer on a phone and toggles the sidebar
  // otherwise. Toggling is the user's decision, so it is remembered.
  el("btn-tree").addEventListener("click", () => {
    // No ensureTreePanel() here: it would build the view first, so
    // openTreePanel() would see `started` as false and never ask the tree to
    // expand to the current folder. The drawer then opened showing only the
    // root, with no subfolders.
    if (!el("tree-panel").classList.contains("hidden")) {
      closeTreePanel();
    } else {
      openTreePanel({ remember: true });
    }
  });
  el("btn-tree-close").addEventListener("click", () => closeTreeDrawer());

  // Crossing the breakpoint changes what the panel is: a sidebar that is part
  // of the layout, or a drawer that must be asked for. Without this, a window
  // narrowed after load keeps a drawer permanently open over the listing.
  if (window.matchMedia) {
    const query = window.matchMedia(DRAWER_QUERY);
    const onBreakpoint = () => {
      if (isDrawer()) {
        // A drawer over the listing would be in the way; it is reopened when
        // asked for. The preference is untouched.
        closeTreePanel({ remember: false });
      } else if (treeOpen) {
        openTreePanel();
      }
    };
    if (query.addEventListener) {
      query.addEventListener("change", onBreakpoint);
    } else if (query.addListener) {
      query.addListener(onBreakpoint);
    }
  }

  // While a row's menu is open that row stays highlighted, even when the
  // pointer moves into the menu: otherwise the menu looks detached from the
  // row it belongs to.
  onRowMenuChange((anchor) => {
    // Every row, not just the highlighted ones: a stale highlight from a
    // previous menu must be cleared even if the classes changed underneath.
    for (const row of list.querySelectorAll("tr")) {
      row.classList.remove("has-menu-open");
    }
    if (!anchor) {
      return;
    }
    // The anchor lives in the row's actions cell. Walking up by tag is what
    // closest("tr") does, but doing it explicitly keeps the relationship
    // visible and does not depend on the row being the nearest <tr> ancestor
    // in some future markup.
    let node = anchor;
    while (node) {
      if (node.tagName === "TR" || node.tag === "tr") {
        node.classList.add("has-menu-open");
        return;
      }
      node = node.parent;
    }
  });
  el("tree-backdrop").addEventListener("click", () => closeTreeDrawer());

  el("hidden-toggle-check").addEventListener("change", (event) => {
    showHidden = event.target.checked;
    // Redraw from the listing already in memory: no request, no flicker.
    renderListing({ entries: lastListing });
  });

  el("btn-refresh").addEventListener("click", () =>
    navigate(currentPath, { push: false })
  );
  btnUpload.addEventListener("click", () => fileInput.click());
  // Search lives in the header and is always visible. It starts at the user's
  // home folder, not wherever they happen to be: "/" is the root of their own
  // storage view (their home for a user, the published folder for a guest).
  // Searching from the current folder silently hid matches above it.
  const headerSearch = createHeaderSearch({
    field: el("header-search-input"),
    results: el("header-search-results"),
    onSelect: (target) => {
      if (target !== currentPath) {
        navigate(target);
      }
    },
    // The same list a listing row shows, from the same module: a result is an
    // entry, so it deserves the same actions and the same permission rules.
    actionsFor: (result) =>
      rowActions({ entry: result, can, isEditable, isImage }),
    onAction: (id, result) => {
      const specs = rowActions({ entry: result, can, isEditable, isImage });
      const act = specs.find((spec) => spec.id === id);
      if (!act) {
        return;
      }
      // The listing refresh in run() is enough: the menu closes on choose, and
      // a fresh search finds only what still exists. Re-running the search here
      // would also reorder the results under the user mid-way through.
      runRowAction(id, result, specs);
    },
  });
  // On a phone the field is an icon until tapped, then it takes the header.
  createExpandableSearch(
    el("btn-search-expand"),
    el("header-search-input"),
    el("header-search")
  );
  el("header-search-clear").addEventListener("click", () => {
    headerSearch.clear();
    el("header-search-clear").classList.add("hidden");
    el("header-search-input").focus();
  });
  el("header-search-input").addEventListener("input", () => {
    setVisible(el("header-search-clear"), Boolean(el("header-search-input").value));
  });
  // Escape in the field must not close the browser or navigate back; the module
  // handles it, and this keeps the clearing button in step.
  el("header-search-input").addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      setVisible(el("header-search-clear"), false);
    }
  });
  btnZip.addEventListener("click", zipSelection);
  btnUnzip.addEventListener("click", unzipSelection);
  btnBatch.addEventListener("click", downloadSelection);
  btnBatchDelete.addEventListener("click", deleteSelection);
  btnBatchCopy.addEventListener("click", () => transferSelection("copy"));
  btnBatchMove.addEventListener("click", () => transferSelection("move"));
  el("btn-selection-clear").addEventListener("click", () => {
    selected.clear();
    for (const box of list.querySelectorAll(".row-check")) {
      box.checked = false;
    }
    syncSelection();
  });
  selectAll.addEventListener("change", () => {
    if (selectAll.checked) {
      for (const path of entriesByPath.keys()) {
        selected.add(path);
      }
    } else {
      selected.clear();
    }
    // Reflect the change on every row without a full re-render.
    for (const box of list.querySelectorAll(".row-check")) {
      box.checked = selected.has(box.dataset.path);
    }
    syncSelection();
  });
  // The logo returns to the user's own home folder: for a signed-in user that
  // is their home, for a guest it is the folder they were given.
  el("app-home").addEventListener("click", (event) => {
    event.preventDefault();
    // Always navigate, even when already at the root: the action is "go home",
    // and a reload from the server is the useful result if nothing changes.
    navigate("/");
  });

  // Logging out is an item of the account menu (js/modules/user_menu.js),
  // built by app.js, which owns the session and reads the capability flags
  // itself.
  btnNew.setAttribute("aria-haspopup", "menu");
  btnNew.addEventListener("click", () => {
    // The same menu the rows use, so the two behave identically.
    openRowMenu({
      anchor: btnNew,
      label: t("New"),
      actions: [
        { id: "folder", label: "Folder", icon: "📁" },
        { id: "file", label: "File", icon: "📄" },
      ],
      onChoose: (id) => (id === "folder" ? newFolder() : newFile()),
    });
  });

  // Drag and drop uploads. Only active when the user may upload.
  const dropTarget = el("file-area") || document.body;
  let dragDepth = 0;

  window.addEventListener("dragenter", (event) => {
    if (!can("upload") || view.classList.contains("hidden")) {
      return;
    }
    if (!event.dataTransfer || ![...event.dataTransfer.types].includes("Files")) {
      return;
    }
    dragDepth += 1;
    document.body.classList.add("is-dragging");
  });
  window.addEventListener("dragover", (event) => {
    if (can("upload") && dragDepth > 0) {
      event.preventDefault();
    }
  });
  window.addEventListener("dragleave", () => {
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) {
      document.body.classList.remove("is-dragging");
    }
  });
  window.addEventListener("drop", (event) => {
    dragDepth = 0;
    document.body.classList.remove("is-dragging");
    if (!can("upload") || !event.dataTransfer) {
      return;
    }
    event.preventDefault();
    upload(event.dataTransfer.files);
  });
  void dropTarget;

  // Keep the view in sync with browser back/forward. The history entry
  // already exists, so this must not push another one.
  window.addEventListener("hashchange", () => {
    if (view.classList.contains("hidden")) {
      return;
    }
    const path = decodeURIComponent(window.location.hash.replace(/^#/, "")) || "/";
    if (path !== currentPath) {
      navigate(path, { push: false });
    }
  });

  // pushState does not fire hashchange, so Back and Forward are caught here.
  window.addEventListener("popstate", () => {
    if (view.classList.contains("hidden")) {
      return;
    }
    const path = decodeURIComponent(window.location.hash.replace(/^#/, "")) || "/";
    if (path !== currentPath) {
      navigate(path, { push: false });
    }
  });

  return {
    setConfig(next) {
      config = next || {};
      applyMaxWidth(config.max_width);
      // The date pattern may have changed under this session (an administrator
      // saving Settings); redraw with what is already loaded.
      if (lastListing) {
        renderListing({ entries: lastListing });
      }
    },
    setUser(nextUser) {
      // A different account starts with nothing carried over. The view lives as
      // long as the page does, so without this the previous user's folder tree
      // (and its loaded folders) stayed on screen, and a folder the next
      // account cannot open failed with "path not found" when clicked.
      if (!user || nextUser.username !== user.username) {
        resetSession();
      }
      user = nextUser;
      userLabel.title = `${nextUser.username} (${nextUser.role})`;
      applyPermissions();
      applyLabels();
      // Last, so the label reflects the session (a guest reads "Log in").
      updateSessionButton();
    },
    /**
     * Re-renders everything written in the interface language.
     *
     * `setLanguage` swaps the message table but cannot redraw the page: every
     * label built by t() was rendered in the old language and stays that way
     * until something asks again. The listing is redrawn because it carries
     * "Folder", the column headers and the formatted dates; the admin area
     * rebuilds itself the next time one of its pages is opened.
     */
    applyLanguage() {
      applyLabels();
      // An empty array is also "nothing loaded yet", so an unconditional redraw
      // would clear the table on a screen that had never fetched a listing.
      if (lastListing.length > 0) {
        renderListing({ entries: lastListing });
      }
    },
    setBrand(appName, logos) {
      // The title belongs in the browser tab; the header shows the logo only.
      document.title = appName || "FileGator";

      // The browser and the administration area each have their own pair of
      // images, and both show the same branding: there is one logo setting.
      applyLogo("app-logo", logos.light, logos.dark, t("Home"));
      applyLogo("admin-logo", logos.light, logos.dark, t("Home"));
      // The logo is a home button, so its label and tooltip follow the language.
      const home = el("app-home");
      home.title = t("Home");
      home.setAttribute("aria-label", t("Home"));
    },
    show() {
      applyLabels();
      selected.clear();
      view.classList.remove("hidden");
      // The first render replaces whatever entry the page was loaded with, so
      // Back leaves the application rather than bouncing inside it.
      const hashPath = decodeURIComponent(window.location.hash.replace(/^#/, ""));
      navigate(hashPath || "/", { push: false });
    },
    hide() {
      view.classList.add("hidden");
    },
    /**
     * Throws away the signed-in user's view of the repository.
     *
     * Called on logout: the view is shown again for the next account, and
     * leaving the previous user's listing on screen (or their folder tree
     * behind a hidden view) showed the next person folders they cannot open.
     */
    reset() {
      resetSession();
      // The previous account's path must not be the URL the next one lands on:
      // the hash would be restored on the next show() and 404 there.
      currentPath = "/";
      history.replaceState(null, "", window.location.pathname);
    },
    navigate,
    currentPath: () => currentPath,
    baseName,
  };
}
