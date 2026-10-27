/**
 * Upload panel: overall progress, per-file rows, pause, cancel and retry.
 *
 * The queue does the work (js/modules/upload.js); this module only renders its
 * state and forwards clicks. Splitting it this way keeps the upload logic
 * testable without a DOM.
 */

import { create, el, hide, show } from "./dom.js";
import { formatSize } from "./format.js";
import { t } from "./i18n.js";
import { UploadQueue, DONE, FAILED, PAUSED, QUEUED, UPLOADING } from "./upload.js";

/** Formats bytes per second, e.g. "820 KB/s" or "1.2 MB/s". */
export function formatSpeed(bytesPerSecond) {
  if (!bytesPerSecond || bytesPerSecond <= 0) {
    return "";
  }
  return `${formatSize(Math.round(bytesPerSecond))}/s`;
}

/** Creates the panel controller. */
export function createUploadPanel({ chunkSize, simultaneous, onStored, askCollision }) {
  const panel = el("upload-panel");
  const list = el("upload-list");
  const summaryText = el("upload-summary");
  const overallFill = el("upload-overall-fill");
  const btnPause = el("upload-pause");
  const pauseIcon = el("upload-pause-icon");
  const pauseLabel = el("upload-pause-label");
  const btnClose = el("upload-close");
  const toggle = el("upload-toggle");
  const caret = el("upload-caret");

  let expanded = true;
  let rows = new Map();

  const queue = new UploadQueue({
    chunkSize,
    simultaneous,
    // A colliding name asks once; the answer can cover the rest of the batch,
    // so ten duplicates do not mean ten prompts.
    // The entry, not just its name: the queue uses the answer to carry on from
    // the same chunk, and the caller needs the whole entry to apply it.
    // The prompt needs a name, not the entry: passing the object through made
    // the dialog read "A file with this name already exists: [object Object]".
    resolveCollision: (entry) => askCollision(entry.name),
    onStored: (entry, stored) => {
      onStored(entry, stored);
    },
    onChange: () => render(),
  });

  // Hide immediately once nothing is left to do; a finished summary would
  // otherwise sit on screen forever, and v7 dismissed it on close too.
  let hideTimer = null;

  /** Builds one row for an entry. */
  function row(entry) {
    const bar = create("span", { className: "upload-row-fill" });
    const barWrap = create("div", { className: "upload-row-bar" }, [bar]);

    const status = create("span", { className: "upload-row-status" });
    const action = create("button", {
      className: "upload-row-action",
      attrs: { type: "button" },
    });

    const name = create("span", { className: "upload-row-name", text: entry.name });
    const meta = create("span", { className: "upload-row-meta" });

    const node = create("li", { className: "upload-row" }, [
      create("div", { className: "upload-row-head" }, [name, status, action]),
      create("div", { className: "upload-row-detail" }, [meta]),
      barWrap,
    ]);

    action.addEventListener("click", () => {
      if (entry.state === FAILED) {
        queue.retry(entry);
      } else if (entry.state === DONE) {
        rows.get(entry.id)?.remove();
        queue.entries = queue.entries.filter((e) => e !== entry);
        render();
      } else {
        queue.cancel(entry);
      }
    });

    return { entry, node, bar, status, meta, action };
  }

  /** Updates one row from its entry. */
  function paint(view) {
    const { entry, bar, status, meta, action } = view;
    const pct = Math.round(entry.progress * 100);

    view.node.dataset.state = entry.state;
    bar.style.width = `${pct}%`;

    const speed = formatSpeed(entry.speed);
    if (entry.state === UPLOADING) {
      status.textContent = `${pct}%`;
      meta.textContent = speed
        ? `${formatSize(entry.sent)} / ${formatSize(entry.size)} · ${speed}`
        : `${formatSize(entry.sent)} / ${formatSize(entry.size)}`;
      action.textContent = "✕";
      action.title = t("Cancel");
      action.setAttribute("aria-label", `${t("Cancel")} ${entry.name}`);
    } else if (entry.state === PAUSED) {
      status.textContent = t("Paused");
      meta.textContent = formatSize(entry.size);
      action.textContent = "✕";
      action.title = t("Cancel");
      action.setAttribute("aria-label", `${t("Cancel")} ${entry.name}`);
    } else if (entry.state === QUEUED) {
      status.textContent = t("Waiting");
      meta.textContent = formatSize(entry.size);
      action.textContent = "✕";
      action.title = t("Cancel");
      action.setAttribute("aria-label", `${t("Cancel")} ${entry.name}`);
    } else if (entry.state === DONE) {
      status.textContent = "✓";
      meta.textContent = formatSize(entry.size);
      action.textContent = "✕";
      action.title = t("Close");
      action.setAttribute("aria-label", `${t("Close")} ${entry.name}`);
    } else {
      status.textContent = "✕";
      // "Upload failed" is v7's template "{0} failed to upload", so it is
      // filled with the file name here. Languages that still carry the
      // placeholder get the name; any that do not simply read as a sentence.
      meta.textContent = entry.error ? t(entry.error) : t("Upload failed", entry.name);
      action.textContent = "⟳";
      action.title = t("Retry");
      action.setAttribute("aria-label", `${t("Retry")} ${entry.name}`);
    }
  }

  /** Renders the whole panel from the queue. */
  function render() {
    const active = queue.active;

    if (!queue.entries.length) {
      clearTimeout(hideTimer);
      hide(panel);
      rows = new Map();
      list.replaceChildren();
      return;
    }

    // Starting to upload again must cancel a pending hide.
    if (active) {
      clearTimeout(hideTimer);
      hideTimer = null;
    }

    show(panel);
    list.classList.toggle("hidden", !expanded);
    caret.textContent = expanded ? "▾" : "▸";
    toggle.setAttribute("aria-expanded", String(expanded));

    const pct = Math.round(queue.progress * 100);
    overallFill.style.width = `${pct}%`;
    overallFill.classList.toggle("is-paused", queue.paused && active);
    const speed = formatSpeed(queue.speed);
    const { done, failed, total } = queue.summary;

    if (active) {
      const line = t("Uploading files", pct, formatSize(queue.entries.reduce((s, e) => s + e.size, 0)));
      summaryText.textContent = queue.paused ? `${line} (${t("Paused")})` : line;
    } else {
      // "Upload summary" is v7's phrasing: "{0} successful, {1} failed, {2} total".
      summaryText.textContent =
        t("Upload summary", done, failed, total) + (speed ? ` (${speed})` : "");
    }

    pauseIcon.textContent = queue.paused ? "▶" : "⏸";
    pauseLabel.textContent = queue.paused ? t("Resume") : t("Pause");
    btnPause.title = pauseLabel.textContent;
    btnPause.hidden = !active;
    btnClose.title = active ? t("Cancel all") : t("Close");
    el("upload-close-label").textContent = btnClose.title;

    // Rows are keyed by entry so a repaint does not rebuild the DOM and lose
    // focus or scroll position.
    const seen = new Set();
    for (const entry of queue.entries) {
      seen.add(entry.id);
      let view = rows.get(entry.id);
      if (!view) {
        view = row(entry);
        rows.set(entry.id, view);
        list.append(view.node);
      }
      paint(view);
    }
    for (const [id, view] of rows) {
      if (!seen.has(id)) {
        view.node.remove();
        rows.delete(id);
      }
    }
  }

  btnPause.addEventListener("click", () => queue.togglePause());

  btnClose.addEventListener("click", async () => {
    if (queue.active) {
      // Every other destructive action asks for confirmation; stopping a
      // batch of uploads is no different.
      const { confirmAction } = await import("./dialog.js");
      const ok = await confirmAction({
        title: t("Cancel all"),
        message: t("Are you sure you want to stop all uploads?"),
        confirmText: t("Confirm"),
        danger: true,
      });
      if (!ok) {
        return;
      }
      await queue.cancelAll();
    } else {
      queue.clearFinished();
    }
  });

  toggle.addEventListener("click", () => {
    expanded = !expanded;
    render();
  });

  return {
    /** Adds files to the queue and shows the panel. */
    start(files, destDir) {
      expanded = true;
      queue.add(files, destDir);
    },
    /** Cancels everything, used when the view is torn down. */
    stop() {
      return queue.cancelAll();
    },
    get queue() {
      return queue;
    },
  };
}
