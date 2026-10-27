/**
 * Folder tree: browse the folder hierarchy without leaving the current view.
 *
 * v7 loaded a subtree per expansion and cached it in the store; this fetches on
 * expansion too, but keeps the state in the tree object so it can be tested
 * without a store. Only folders are listed, since files have nothing to expand.
 */

import { api } from "./api.js";
import { dialogRoot } from "./dialog.js";
import { create } from "./dom.js";
import { iconFor } from "./format.js";
import { t } from "./i18n.js";

/** One folder in the tree. */
export class TreeNode {
  constructor(entry, parent) {
    this.name = entry.name;
    this.path = entry.path;
    this.parent = parent || null;
    this.children = null; // null = never loaded
    this.expanded = false;
    this.loading = false;
    this.error = "";
  }

  get level() {
    return this.parent ? this.parent.level + 1 : 0;
  }
}

/**
 * Expands or collapses a node, loading its children the first time.
 *
 * Returns the node so callers can await the load and then re-render.
 */
export async function toggleNode(node) {
  if (node.expanded) {
    collapseSubtree(node);
    return node;
  }
  node.expanded = true;
  if (node.children !== null) {
    return node;
  }

  node.loading = true;
  node.error = "";
  try {
    const listing = await api.list(node.path);
    node.children = (listing.entries || [])
      .filter((entry) => entry.type === "dir")
      .map((entry) => new TreeNode(entry, node));
  } catch (err) {
    // A folder the user may not open is not an error worth breaking the tree
    // for: show it as empty and say why.
    node.children = [];
    node.error = err && err.message ? err.message : t("Unknown error");
    node.expanded = false;
  } finally {
    node.loading = false;
  }
  return node;
}

/**
 * Renders a folder tree into an element.
 *
 * Shared by the modal picker and the sidebar so the two cannot drift apart.
 *
 * options:
 *   rootPath - the folder the tree starts at
 *   current  - the folder to highlight
 *   onSelect - (node) => void when a folder name is clicked
 *   compact  - hides the status line (the sidebar has its own)
 */
export function createTreeView({ rootPath = "/", current = "/", onSelect, compact = false }) {
  const list = create("ul", { className: "tree-list" });
  const status = create("p", { className: "tree-status", attrs: { role: "status" } });
  const root = new TreeNode({ name: t("Home"), path: rootPath }, null);
  let highlight = current;
  // What is on screen now, so a navigation that changes nothing structural
  // does not rebuild it.
  let renderedSignature = null;

  /**
   * Renders the whole tree from its nodes.
   *
   * Only when the structure changed. Navigation usually expands nothing —
   * the branches are already loaded — so rebuilding every row tore the panel
   * down and repainted it for a highlight move, which flashed on every click.
   */
  function render() {
    const signature = describe(root);
    if (signature === renderedSignature) {
      paintHighlight();
      return;
    }
    renderedSignature = signature;
    list.replaceChildren();
    renderNode(root, list);
  }

  /**
   * The rendered shape of the tree: which nodes exist, which are open, and
   * which are loading.
   *
   * Cheaper than comparing DOM, and it changes exactly when a rebuild is
   * needed.
   */
  function describe(item) {
    let out = `${item.path}|${item.expanded ? 1 : 0}|${item.loading ? 1 : 0}|${item.error ? 1 : 0}`;
    for (const child of item.children || []) {
      out += "\n" + describe(child);
    }
    return out;
  }

  /** Moves the "current folder" highlight without touching the rows. */
  function paintHighlight() {
    for (const name of list.querySelectorAll(".tree-name")) {
      // getAttribute rather than dataset.path: the path is written with
      // setAttribute, and reading it back the same way cannot disagree.
      const path = name.getAttribute("data-path");
      const active = path === highlight;
      name.classList.toggle("is-current", active);
      if (active) {
        name.setAttribute("aria-current", "true");
      } else {
        name.removeAttribute("aria-current");
      }
    }
  }

  /** Renders one node and, when expanded, its children. */
  function renderNode(item, container) {
    const row = create("div", {
      className: "tree-row",
      attrs: { style: `padding-left: ${0.5 + item.level * 1.1}rem` },
    });

    const toggle = create("button", {
      className: "tree-toggle",
      text: item.loading ? "…" : item.expanded ? "▾" : "▸",
      attrs: {
        type: "button",
        "aria-expanded": String(item.expanded),
        "aria-label": item.expanded ? t("Collapse") : t("Expand"),
      },
    });
    toggle.addEventListener("click", async (event) => {
      event.stopPropagation();
      await toggleNode(item);
      status.textContent = item.error || "";
      render();
    });

    const name = create("button", {
      className: `tree-name${item.path === highlight ? " is-current" : ""}`,
      attrs: {
        type: "button",
        title: item.path,
        // The highlight is moved by paintHighlight without a rebuild, so it
        // needs the path on the element to find.
        "data-path": item.path,
        ...(item.path === highlight ? { "aria-current": "true" } : {}),
      },
    }, [
      create("span", { className: "tree-icon", text: iconFor({ type: "dir", name: item.name }) }),
      create("span", { text: item.name }),
    ]);
    name.addEventListener("click", () => onSelect && onSelect(item));

    row.append(toggle, name);
    // The list item is held directly rather than reached through lastChild:
    // relying on the last appended child breaks as soon as the rendering order
    // changes.
    const node = create("li", { className: "tree-node" }, [row]);
    container.append(node);

    if (item.expanded) {
      if (item.children === null) {
        node.append(create("div", { className: "tree-row tree-loading", text: t("Loading…") }));
        return;
      }
      // An empty folder simply shows nothing below it. The "No subfolders"
      // line read as a folder-looking row that could not be clicked, which is
      // more confusing than the absence of rows.
      if (item.children.length === 0) {
        return;
      }
      const sub = create("ul", { className: "tree-children" });
      for (const child of item.children) {
        renderNode(child, sub);
      }
      node.append(sub);
    }
  }

  /**
   * Opens the path to a folder and highlights it.
   *
   * Called on every navigation so the panel follows the listing; the branches
   * already loaded are reused, so this costs one request per level at most.
   *
   * The tree is rendered **once**, after every level has loaded. Rendering per
   * level made the panel flash: each intermediate state was painted, so a
   * click redrew the whole tree two or three times in a row.
   */
  async function show(path) {
    highlight = path;
    await expandTo(root, path);
    render();
    scrollToActive();
  }

  /**
   * Throws the loaded folders away and loads the root again.
   *
   * The tree belongs to the session, not to the page: after a logout the
   * folders one user could see were still on screen for the next one, and a
   * folder from the previous account failed to open ("path not found"). The
   * sidebar is built once and reused, so nothing else rebuilds it.
   */
  async function reset() {
    root.children = null;
    root.error = "";
    collapseSubtree(root);
    highlight = rootPath;
    renderedSignature = null;
    list.replaceChildren();
    status.textContent = "";
    await expandNode(root);
    render();
  }

  /**
   * Brings the highlighted folder into view without moving the page.
   *
   * scrollIntoView walks up to the nearest scrollable ancestor, and on a wide
   * screen the document is one: the call nudged the whole page, which is the
   * jiggle when picking a folder in the tree. Scrolling the list by hand only
   * moves the tree.
   */
  function scrollToActive() {
    const active = list.querySelector(".tree-name.is-current");
    if (!active || !list.scrollHeight) {
      return;
    }
    const top = active.offsetTop;
    const bottom = top + active.offsetHeight;
    if (top < list.scrollTop) {
      list.scrollTop = top;
    } else if (bottom > list.scrollTop + list.clientHeight) {
      list.scrollTop = bottom - list.clientHeight;
    }
  }

  /** Re-loads the children of a folder, for after a create or delete. */
  async function refresh(path) {
    const node = find(root, path);
    if (!node) {
      return;
    }
    node.children = null;
    if (node.expanded) {
      await toggleNode(node);
    }
    render();
  }

  /** Finds a node by path. */
  function find(item, path) {
    if (item.path === path) {
      return item;
    }
    for (const child of item.children || []) {
      const found = find(child, path);
      if (found) {
        return found;
      }
    }
    return null;
  }

  render();
  const parts = [status, list];
  if (compact) {
    status.remove?.();
  }
  return {
    element: compact ? list : create("div", { className: "tree-body" }, parts),
    show,
    refresh,
    reset,
    /** Opens the root immediately, so a sidebar is not empty on first paint. */
    async open() {
      if (!root.expanded) {
        await toggleNode(root);
        render();
      }
    },
  };
}

/**
 * Opens the folder picker as a modal.
 *
 * onSelect is called with the chosen node and the dialog closes.
 */
export function openFolderTree({ rootPath = "/", current = "/", onSelect }) {
  const node = dialogRoot();
  const restoreFocus = document.activeElement;
  let settled = false;

  const close = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": t("Close"), title: t("Close") },
  });

  const tree = createTreeView({
    rootPath,
    current,
    onSelect: (item) => finish(item),
  });

  const footer = create("div", { className: "dialog-actions" }, [
    create("button", { className: "button", text: t("Close"), attrs: { type: "button" } }),
  ]);
  footer.querySelector("button").addEventListener("click", () => finish(null));

  const form = create("div", { className: "dialog tree-dialog" }, [
    create("div", { className: "dialog-header" }, [
      create("h2", { className: "dialog-title", text: t("Select Folder") }),
      close,
    ]),
    tree.element,
    footer,
  ]);

  function finish(value) {
    if (settled) {
      return;
    }
    settled = true;
    node.classList.add("hidden");
    node.onclick = null;
    node.onkeydown = null;
    node.replaceChildren();
    if (restoreFocus && document.contains(restoreFocus)) {
      restoreFocus.focus();
    }
    if (value) {
      onSelect(value);
    }
  }

  close.addEventListener("click", () => finish(null));
  node.onclick = (event) => {
    if (event.target === node) {
      finish(null);
    }
  };
  node.onkeydown = (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      finish(null);
    }
  };

  node.replaceChildren(form);
  node.classList.remove("hidden");
  tree.show(current);
  return node;
}

/**
 * Collapses a node and everything inside it.
 *
 * Collapsing only the node it was clicked on left its children marked
 * expanded, so reopening the parent showed the whole subtree already open.
 * v7 behaved the same way: closing a folder closed what was inside it, which
 * is what makes collapsing useful as "start again from here".
 */
export function collapseSubtree(node) {
  node.expanded = false;
  for (const child of node.children || []) {
    collapseSubtree(child);
  }
  return node;
}

/**
 * Expands a node, doing nothing when it is already expanded.
 *
 * expandTo used toggleNode here, which collapsed the root every time a folder
 * other than the root was shown — so the tree flickered shut and the target
 * was never highlighted.
 */
export async function expandNode(node) {
  // A node can be marked expanded with its children still unloaded (null):
  // returning early here left the root permanently bare, because the first
  // show() found it "already expanded" and never fetched anything.
  if (node.expanded && node.children !== null) {
    return node;
  }
  if (node.expanded) {
    // Expanded but never loaded: load it in place rather than toggling, which
    // would collapse it.
    node.expanded = false;
  }
  return toggleNode(node);
}

/**
 * Expands the nodes leading to path, loading each level.
 *
 * Only the ancestors of the target are opened: expanding everything would make
 * a large tree useless.
 */
export async function expandTo(root, path) {
  if (path === root.path) {
    await expandNode(root);
    return;
  }
  const prefix = root.path === "/" ? "/" : root.path + "/";
  if (!path.startsWith(prefix)) {
    await expandNode(root);
    return;
  }

  await expandNode(root);
  let current = root;
  const segments = path.slice(prefix.length).split("/").filter(Boolean);
  for (const segment of segments) {
    const children = current.children || [];
    const next = children.find((child) => child.name === segment);
    if (!next) {
      return;
    }
    await expandNode(next);
    current = next;
  }
}
