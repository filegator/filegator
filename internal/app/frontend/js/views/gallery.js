/**
 * Image gallery: one large picture with a strip of the folder's other images.
 *
 * v7 opened a modal with the clicked image and a thumbnail column. This keeps
 * the same idea but adds keyboard navigation (arrow keys, Escape), a position
 * counter and touch swipe, because the main reason to open a gallery on a
 * phone is to flick through pictures.
 */

import { api } from "../modules/api.js";
import { closeDialog, dialogRoot } from "../modules/dialog.js";
import { create } from "../modules/dom.js";
import { formatSize } from "../modules/format.js";
import { t } from "../modules/i18n.js";

/** Extensions the browser can display directly. */
export const IMAGE_EXTENSIONS = ["jpg", "jpeg", "png", "gif", "webp", "bmp", "avif"];

/**
 * Reports whether a name is an image the gallery can show.
 *
 * SVG is deliberately excluded: it can carry script, so the download endpoint
 * serves it as a download rather than a picture, and an <img> pointing at it
 * would fail silently. TIFF is excluded too — browsers do not render it.
 */
export function isImage(name) {
  const dot = (name || "").lastIndexOf(".");
  if (dot < 0) {
    return false;
  }
  return IMAGE_EXTENSIONS.includes(name.slice(dot + 1).toLowerCase());
}

/**
 * Opens the gallery.
 *
 * images - the folder's image entries, in display order
 * start  - the entry to show first
 */
export function openGallery(images, start) {
  closeDialog();
  const node = dialogRoot();

  // Only the current image is loaded eagerly; the rest are thumbnails. A
  // folder of 500 photos should not start 500 full-size downloads.
  const list = [...images];
  // The clicked entry must be in the strip. If the caller's list is stale or
  // empty, add it rather than silently showing whatever happens to be first.
  let index = list.findIndex((item) => item.path === start.path);
  if (index < 0) {
    list.push(start);
    index = list.length - 1;
  }
  const restoreFocus = document.activeElement;

  const title = create("h2", { className: "dialog-title gallery-title", text: start.name });
  const info = create("span", { className: "gallery-count" });

  const closeBtn = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": t("Close"), title: t("Close") },
  });

  const picture = create("img", {
    className: "gallery-image",
    attrs: { alt: "", draggable: "false" },
  });

  const prev = create("button", {
    className: "gallery-nav gallery-prev",
    text: "‹",
    attrs: { type: "button", "aria-label": t("Previous") },
  });
  const next = create("button", {
    className: "gallery-nav gallery-next",
    text: "›",
    attrs: { type: "button", "aria-label": t("Next") },
  });

  const stage = create("div", { className: "gallery-stage" }, [prev, picture, next]);
  const strip = create("div", { className: "gallery-strip" });
  const status = create("p", { className: "gallery-status", attrs: { role: "status" } });

  // The href stays a real link (copyable, middle-clickable), but it opens in a
  // new tab: downloading the picture must not replace the gallery or the file
  // manager behind it. No "download" attribute, or the link would save the file
  // in place instead of opening the image.
  const download = create("a", {
    className: "gallery-download button button-ghost",
    text: t("Download"),
    attrs: { target: "_blank", rel: "noopener" },
  });

  const form = create("div", {
    className: "gallery",
    attrs: { role: "document" },
  }, [
    create("div", { className: "dialog-header" }, [title, info, closeBtn]),
    stage,
    list.length > 1 ? strip : create("div"),
    create("div", { className: "gallery-footer" }, [status, download]),
  ]);

  /** Shows one image and syncs every control to it. */
  function show(i) {
    index = (i + list.length) % list.length;
    const item = list[index];
    const url = api.downloadUrl(item.path);

    // The file name is shown in the header, so a screen reader announcing the
    // image again would only repeat it.
    picture.src = url;
    picture.alt = "";
    title.textContent = item.name;
    info.textContent = list.length > 1 ? `${index + 1} / ${list.length}` : "";
    download.href = url;
    status.textContent = formatSize(item.size);

    for (const [n, thumb] of [...strip.children].entries()) {
      thumb.classList.toggle("is-current", n === index);
    }
    const current = strip.children[index];
    if (current) {
      current.scrollIntoView({ block: "nearest", inline: "center" });
    }
    prev.hidden = list.length < 2;
    next.hidden = list.length < 2;
  }

  // Thumbnails are built once; only the highlighted one changes.
  if (list.length > 1) {
    for (const [i, item] of list.entries()) {
      const thumb = create("img", {
        className: "gallery-thumb",
        attrs: {
          src: api.downloadUrl(item.path),
          alt: item.name,
          loading: "lazy",
          draggable: "false",
        },
      });
      thumb.addEventListener("click", () => show(i));
      // Thumbnails are clickable, so they must be reachable without a mouse.
      thumb.setAttribute("role", "button");
      thumb.setAttribute("tabindex", "0");
      thumb.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          show(i);
        }
      });
      strip.append(thumb);
    }
  }

  let settled = false;
  function finish() {
    if (settled) {
      return;
    }
    settled = true;
    node.classList.add("hidden");
    node.onclick = null;
    node.onkeydown = null;
    node.replaceChildren();
    picture.src = "";
    if (restoreFocus && document.contains(restoreFocus)) {
      restoreFocus.focus();
    }
  }

  closeBtn.addEventListener("click", finish);
  prev.addEventListener("click", () => show(index - 1));
  next.addEventListener("click", () => show(index + 1));

  // Tab order follows the strip, not the DOM: the header's controls come
  // first, then previous/next, then the thumbnails.
  for (const el of [title, info, strip]) {
    el.tabIndex = -1;
  }

  // Tapping the picture area (not a control) advances, which is what a photo
  // viewer should do; the controls stop their own clicks bubbling.
  picture.addEventListener("click", () => {
    if (list.length > 1) {
      show(index + 1);
    }
  });
  for (const control of [prev, next, download]) {
    control.addEventListener("click", (event) => event.stopPropagation());
  }

  // Touch swipe: 40px horizontal is enough to mean "next", and the same
  // gesture the photo apps use.
  let touchStartX = null;
  stage.addEventListener(
    "touchstart",
    (event) => {
      touchStartX = event.touches[0].clientX;
    },
    { passive: true }
  );
  stage.addEventListener(
    "touchend",
    (event) => {
      if (touchStartX === null) {
        return;
      }
      const delta = event.changedTouches[0].clientX - touchStartX;
      touchStartX = null;
      if (Math.abs(delta) < 40 || list.length < 2) {
        return;
      }
      show(delta < 0 ? index + 1 : index - 1);
    },
    { passive: true }
  );

  node.onclick = (event) => {
    if (event.target === node) {
      finish();
    }
  };
  node.onkeydown = (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      finish();
      return;
    }
    if (event.key === "ArrowRight" || event.key === " ") {
      event.preventDefault();
      show(index + 1);
      return;
    }
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      show(index - 1);
      return;
    }
    if (event.key !== "Tab") {
      return;
    }
    // Keep focus inside the gallery.
    const focusable = node.querySelectorAll("button, a[href]");
    if (!focusable.length) {
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  node.replaceChildren(form);
  node.classList.remove("hidden");
  show(index);
  closeBtn.focus();
}
