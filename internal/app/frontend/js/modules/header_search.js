/**
 * The search box in the header.
 *
 * Always visible, centred between the logo and the account actions. Typing
 * searches the user's home folder and results drop down under the field, so the
 * listing and the folder tree stay visible while results appear. This replaced
 * a toolbar button that opened a modal: a field you can see is one you use, and
 * a dropdown does not hide where you are the way a modal does.
 *
 * The search itself is the same server-side walk (GET /api/files/search) the
 * modal used, with a 250 ms debounce and arrow-key navigation.
 */

import { api, ApiError } from "./api.js";
import { create } from "./dom.js";
import { formatSize, iconFor } from "./format.js";
import { t } from "./i18n.js";
import { openRowMenu } from "./row_menu.js";

/** How long to wait after the last keystroke before searching. */
const DEBOUNCE_MS = 250;

/**
 * Wires a header search field.
 *
 * field      - the input element
 * results    - the container the dropdown is rendered into
 * onSelect   - (targetPath) => void, called when a result is chosen
 * rootPath   - the folder searches start at, "/" by default (the home folder)
 * actionsFor - (result) => [{ id, label, icon, danger }], the same list the
 *              row menu shows. Deciding it here would mean this module knowing
 *              about permissions and file extensions; the browser already
 *              decides it for a listing row, so it decides it for a result too.
 * onAction   - (id, result) => void, performed without closing the dialog
 */
// Actions that open a dialog, an overlay or a prompt, and so must close the
// dropdown first. On a phone, a modal opened over an open dropdown leaves the
// dropdown painted under the dialog's backdrop: the folder picker appeared
// "behind the search results" exactly like that.
//
// `download` and `unzip` are deliberately absent: they act in place, and the
// remaining results are meant to stay usable for the next one.
const ACTIONS_OPEN_A_DIALOG = new Set([
  "copy",
  "move",
  "edit",
  "view",
  "rename",
  "zip",
  "chmod",
  "delete",
]);

export function createHeaderSearch({
  field,
  results,
  onSelect,
  rootPath = "/",
  actionsFor,
  onAction,
}) {
  let timer = null;
  let controller = null;
  let items = [];
  let active = 0;
  let open = false;

  const setOpen = (value) => {
    open = value;
    results.classList.toggle("hidden", !value);
    field.setAttribute("aria-expanded", String(value));
  };

  const setActive = (index) => {
    if (!items.length) {
      return;
    }
    active = Math.max(0, Math.min(index, items.length - 1));
    const buttons = results.querySelectorAll(".search-result-button");
    buttons.forEach((button, i) => {
      button.classList.toggle("is-active", i === active);
    });
    const current = buttons[active];
    if (current && current.scrollIntoView) {
      current.scrollIntoView({ block: "nearest" });
    }
  };

  /** The folder to open for one result. */
  const targetOf = (result) => (result.type === "dir" ? result.path : result.parent);

  const render = () => {
    results.replaceChildren();
    for (const [index, result] of items.entries()) {
      const button = create(
        "button",
        {
          className: "search-result-button",
          attrs: { type: "button" },
        },
        [
          create("span", { className: "search-result-icon", text: iconFor(result) }),
          create("span", { className: "search-result-text" }, [
            create("span", { className: "search-result-name", text: result.name }),
            create("span", { className: "search-result-path", text: result.path }),
          ]),
          create("span", {
            className: "search-result-meta",
            text: result.type === "dir" ? "" : formatSize(result.size),
          }),
        ]
      );
      button.addEventListener("click", () => choose(result));

      // A result's actions, without navigating to it first: finding a file and
      // deleting it should not cost a trip to its folder.
      const row = create("li", { className: "search-result" }, [button]);
      const specs = actionsFor ? actionsFor(result) : [];
      if (specs && specs.length) {
        const menuButton = create("button", {
          className: "row-menu-button search-result-menu",
          text: "⋯",
          attrs: {
            type: "button",
            "aria-haspopup": "menu",
            "aria-expanded": "false",
            "aria-label": t("Actions for {0}", result.name),
            title: t("Actions"),
          },
        });

        /** Opens the actions for this result, at the button or at the pointer. */
        const open = (at) => {
          openRowMenu({
            anchor: menuButton,
            ...(at ? { at } : {}),
            label: t("Actions for {0}", result.name),
            actions: specs,
            onChoose: (id) => {
              // An action that opens a dialog closes the results first; the
              // others act in place, so the list stays put and a delete removes
              // one row while the rest of the results are still there to work
              // through.
              if (ACTIONS_OPEN_A_DIALOG.has(id)) {
                setOpen(false);
              }
              if (onAction) {
                onAction(id, result);
              }
            },
          });
        };

        menuButton.addEventListener("click", (event) => {
          // The button sits inside the row; without this the row's own click
          // would also fire and navigate away from the result.
          event.preventDefault();
          event.stopPropagation();
          open();
        });

        // Right-click anywhere in the result opens the same menu at the
        // pointer, as a listing row does. The browser's own menu is replaced,
        // which is what makes it quick to reach Delete without aiming.
        row.addEventListener("contextmenu", (event) => {
          event.preventDefault();
          open({ x: event.clientX, y: event.clientY });
        });

        row.append(menuButton);
      }
      results.append(row);
    }
    setActive(active);
  };

  /** Leaves the dropdown open with a single line of text. */
  const note = (text) => {
    results.replaceChildren(
      create("li", { className: "search-note", attrs: { role: "status" }, text })
    );
    setOpen(true);
  };

  const choose = (result) => {
    if (!result) {
      return;
    }
    clear();
    setOpen(false);
    onSelect(targetOf(result));
  };

  /** Empties the field and cancels any walk in progress. */
  function clear() {
    clearTimeout(timer);
    if (controller) {
      controller.abort();
      controller = null;
    }
    items = [];
    active = 0;
    field.value = "";
    results.replaceChildren();
  }

  const run = async () => {
    const term = field.value.trim();
    if (controller) {
      controller.abort();
      controller = null;
    }
    if (!term) {
      items = [];
      results.replaceChildren();
      setOpen(false);
      return;
    }

    const current = new AbortController();
    controller = current;
    note(t("Searching…"));
    try {
      const data = await api.search(term, rootPath, current.signal);
      // A newer keystroke may have started another search already.
      if (current.signal.aborted) {
        return;
      }
      items = data.results || [];
      active = 0;
      render();
      setOpen(true);
      if (!items.length) {
        note(t("No results"));
      } else if (data.limited) {
        note(t("Too many results, refine your search"));
      }
    } catch (err) {
      if (err && err.name === "AbortError") {
        return;
      }
      items = [];
      note(err instanceof ApiError ? t(err.message) : t("Unknown error"));
    } finally {
      if (controller === current) {
        controller = null;
      }
    }
  };

  field.addEventListener("input", () => {
    clearTimeout(timer);
    // Debounce so typing does not start a walk per keystroke.
    timer = setTimeout(run, DEBOUNCE_MS);
  });

  // Escape works wherever the focus is, not only inside the field: clicking a
  // result leaves the focus on the list or on the page, and a modal that can
  // only be dismissed from one element is a trap on a keyboard.
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !open) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (field.value) {
      // Escape clears the term first, so typing can resume without the field
      // being emptied and closed in one keystroke.
      clear();
      setOpen(false);
      field.focus();
    } else {
      field.blur();
      setOpen(false);
    }
  });

  field.addEventListener("keydown", (event) => {
    if (!open) {
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive(active + 1);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive(active - 1);
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(items[active]);
    }
  });

  // The field keeps its own focus ring; closing on a click elsewhere avoids a
  // dropdown left hanging over the listing.
  document.addEventListener("click", (event) => {
    if (!open) {
      return;
    }
    if (results.contains(event.target) || event.target === field) {
      return;
    }
    setOpen(false);
  });

  // Refocusing a field that still holds a term shows its results again.
  field.addEventListener("focus", () => {
    if (field.value.trim()) {
      run();
    }
  });

  return {
    /** Empties the field, for when the listing navigates away. */
    clear,
    /** Closes the dropdown without touching the typed term. */
    close: () => setOpen(false),
    field,
  };
}

/**
 * Wires the phone-sized expand button.
 *
 * On a narrow screen the field is a search icon until it is tapped; it then
 * takes the whole header width. This keeps a usable input on a phone without
 * spending a row of the screen on it.
 */
export function createExpandableSearch(expandButton, field, wrapper) {
  expandButton.addEventListener("click", () => {
    wrapper.classList.add("is-expanded");
    field.focus();
  });

  const collapse = () => {
    if (field.value.trim()) {
      return;
    }
    wrapper.classList.remove("is-expanded");
  };
  field.addEventListener("blur", () => {
    // Defer so a tap on a result is delivered before the collapse.
    setTimeout(collapse, 120);
  });
  return { collapse };
}
