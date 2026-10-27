/**
 * Chunked upload client with per-file control.
 *
 * Files are split into chunks and sent in order, so a large upload survives a
 * dropped connection and can resume: the server reports how many chunks it
 * already holds.
 *
 * v7 did this with Resumable.js, which offered pause/resume, per-file cancel
 * and retry, per-file progress and speed, and concurrent uploads. This module
 * provides the same behaviour without a dependency: a queue where each entry
 * owns its own state, so one file can be cancelled while another continues.
 */

import { api, ApiError, csrfToken, url } from "./api.js";

/** Per-file states. */
export const QUEUED = "queued";
export const UPLOADING = "uploading";
export const PAUSED = "paused";
export const DONE = "done";
export const FAILED = "failed";
export const CANCELLED = "cancelled";

/** Generates an identifier stable for a given file and destination. */
export function identifierFor(file, destDir) {
  const raw = `${destDir}/${file.name}/${file.size}/${file.lastModified}`;
  let hash = 0;
  for (let i = 0; i < raw.length; i += 1) {
    hash = (hash * 31 + raw.charCodeAt(i)) | 0;
  }
  return `up${Math.abs(hash).toString(36)}${file.size.toString(36)}`;
}

/** Sleeps, resolving early when the signal aborts. */
function sleep(ms, signal) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      cleanup();
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(timer);
      cleanup();
      reject(new DOMException("aborted", "AbortError"));
    };
    const cleanup = () => signal?.removeEventListener("abort", onAbort);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/** One upload in the queue. */
export class UploadEntry {
  constructor(file, destDir, onCollision) {
    this.id = `${file.name}:${file.size}:${file.lastModified}`;
    this.file = file;
    this.name = file.name;
    this.destDir = destDir;
    this.size = file.size;
    this.identifier = identifierFor(file, destDir);
    // Empty means "refuse": the server answers 409 and the queue asks.
    this.onCollision = onCollision || "";
    this.state = QUEUED;
    this.sent = 0;
    this.error = "";
    // Speed is measured over a short window so a slow first chunk does not
    // make the rest of the upload look fast.
    this.speedSample = { bytes: 0, at: 0, speed: 0 };
  }

  /** Fraction complete, 0 to 1. */
  get progress() {
    return this.size === 0 ? 1 : Math.min(1, this.sent / this.size);
  }

  /** Bytes per second, or 0 before the first sample. */
  get speed() {
    return this.state === UPLOADING ? this.speedSample.speed : 0;
  }

  /** Recomputes the speed from the bytes sent since the last sample. */
  sample() {
    const now = Date.now();
    const last = this.speedSample;
    if (!last.at) {
      // First sample only sets the baseline.
      this.speedSample = { bytes: this.sent, at: now, speed: 0 };
      return;
    }
    const elapsed = now - last.at;
    if (elapsed < 500) {
      return;
    }
    const delta = Math.max(0, this.sent - last.bytes);
    // Exponential smoothing keeps the number readable instead of jumping.
    const instant = delta / (elapsed / 1000);
    const speed = last.speed ? last.speed * 0.6 + instant * 0.4 : instant;
    this.speedSample = { bytes: this.sent, at: now, speed };
  }

  /** Clears the speed baseline after a pause or a retry. */
  resetSpeed() {
    this.speedSample = { bytes: this.sent, at: 0, speed: 0 };
  }
}

/**
 * Runs an upload queue.
 *
 * options:
 *   chunkSize   - bytes per chunk (from the config)
 *   simultaneous - how many files upload at once (from the config)
 *   onChange    - called whenever any entry changes state or progress
 *   onStored    - called with entry and the stored file entry
 */
export class UploadQueue {
  /**
   * options:
   *   resolveCollision - (entry) => Promise<"replace"|"upcount"|null>. Called
   *                      when the server refuses a name; null skips the file.
   */
  constructor({ chunkSize, simultaneous = 3, onChange, onStored, resolveCollision } = {}) {
    this.chunkSize = chunkSize > 0 ? chunkSize : 1024 * 1024;
    this.simultaneous = Math.max(1, simultaneous || 1);
    this.onChange = onChange || (() => {});
    this.onStored = onStored || (() => {});
    this.resolveCollision = resolveCollision || (() => Promise.resolve(null));
    // Set once the user chooses "apply to all": later collisions skip the
    // prompt instead of asking again per file.
    this.collisionPolicy = "";
    /** @type {UploadEntry[]} */
    this.entries = [];
    this.paused = false;
    this.running = new Set();
    // One controller per entry, so cancelling one does not touch the others.
    this.controllers = new Map();
  }

  /** Adds files, ignoring ones already in the queue. */
  add(files, destDir) {
    const known = new Set(this.entries.map((e) => e.id));
    for (const file of files) {
      const entry = new UploadEntry(file, destDir);
      if (!known.has(entry.id)) {
        this.entries.push(entry);
        known.add(entry.id);
      }
    }
    this.pump();
    this.onChange();
  }

  /** Entries that still have work to do. */
  get active() {
    return this.entries.some(
      (e) => e.state === UPLOADING || e.state === QUEUED || e.state === PAUSED
    );
  }

  /** Totals used by the summary line. */
  get summary() {
    let done = 0;
    let failed = 0;
    for (const entry of this.entries) {
      if (entry.state === DONE) {
        done += 1;
      } else if (entry.state === FAILED) {
        failed += 1;
      }
    }
    return { done, failed, total: this.entries.length };
  }

  /** Overall progress across every entry, 0 to 1. */
  get progress() {
    const total = this.entries.reduce((sum, e) => sum + e.size, 0);
    if (total === 0) {
      return 1;
    }
    const sent = this.entries.reduce((sum, e) => sum + Math.min(e.sent, e.size), 0);
    return sent / total;
  }

  /** Combined speed of the entries currently uploading. */
  get speed() {
    return this.entries.reduce((sum, e) => sum + e.speed, 0);
  }

  /** Starts as many queued entries as the concurrency allows. */
  pump() {
    if (this.paused) {
      return;
    }
    for (const entry of this.entries) {
      if (this.running.size >= this.simultaneous) {
        return;
      }
      if (entry.state === QUEUED && !this.running.has(entry.id)) {
        // Marked as running before run() begins, so a pump() triggered while
        // the first file is still awaiting its collision answer cannot start
        // the same entry twice.
        this.running.add(entry.id);
        this.run(entry);
      }
    }
  }

  /** Pauses the queue: in-flight chunks finish, nothing new starts. */
  pause() {
    this.paused = true;
    for (const entry of this.entries) {
      if (entry.state === UPLOADING || entry.state === QUEUED) {
        entry.state = PAUSED;
        entry.resetSpeed();
      }
    }
    this.onChange();
  }

  /** Resumes a paused queue. */
  resume() {
    this.paused = false;
    for (const entry of this.entries) {
      if (entry.state === PAUSED) {
        entry.state = QUEUED;
      }
    }
    this.pump();
    this.onChange();
  }

  /** Pauses or resumes, whichever applies. */
  togglePause() {
    if (this.paused) {
      this.resume();
    } else {
      this.pause();
    }
  }

  /** Cancels one entry and asks the server to discard its chunks. */
  async cancel(entry) {
    const controller = this.controllers.get(entry.id);
    if (controller) {
      controller.abort();
    }
    const wasActive = entry.state === UPLOADING || entry.state === QUEUED || entry.state === PAUSED;
    entry.state = CANCELLED;
    entry.error = "";
    this.entries = this.entries.filter((e) => e !== entry);
    this.onChange();

    if (wasActive) {
      try {
        await api.cancelUpload({
          filename: entry.name,
          identifier: entry.identifier,
        });
      } catch {
        // The housekeeping sweep will clear it; never block the UI on this.
      }
    }
    this.pump();
  }

  /** Cancels every entry, in parallel. */
  async cancelAll() {
    const entries = [...this.entries];
    await Promise.all(entries.map((entry) => this.cancel(entry)));
  }

  /** Retries the failed entries. */
  retryFailed() {
    for (const entry of this.entries) {
      if (entry.state === FAILED) {
        entry.state = QUEUED;
        entry.error = "";
        entry.resetSpeed();
      }
    }
    this.pump();
    this.onChange();
  }

  /** Retries one entry. */
  retry(entry) {
    if (entry.state !== FAILED) {
      return;
    }
    entry.state = QUEUED;
    entry.error = "";
    entry.resetSpeed();
    this.pump();
    this.onChange();
  }

  /** Removes finished and cancelled rows from the list. */
  clearFinished() {
    this.entries = this.entries.filter(
      (e) => e.state === UPLOADING || e.state === QUEUED || e.state === PAUSED
    );
    this.onChange();
  }

  /**
   * Uploads one entry, chunk by chunk.
   *
   * The server is asked first how many chunks it already holds, so an upload
   * interrupted by a reload or a pause continues where it stopped instead of
   * starting over.
   */
  async run(entry) {
    const controller = new AbortController();
    this.controllers.set(entry.id, controller);
    this.running.add(entry.id);
    entry.state = UPLOADING;
    entry.error = "";
    entry.resetSpeed();
    this.onChange();

    const step = this.chunkSize;
    const totalChunks = Math.max(1, Math.ceil(entry.size / step));

    try {
      let received = 0;
      try {
        const status = await api.uploadStatus({
          filename: entry.name,
          identifier: entry.identifier,
          totalChunks,
        });
        received = Math.min(status.received || 0, totalChunks);
      } catch {
        received = 0;
      }
      entry.sent = Math.min(received * step, entry.size);
      this.onChange();

      for (let chunk = received + 1; chunk <= totalChunks; chunk += 1) {
        // Pausing lets the current chunk finish, then stops before the next.
        // Cancelling aborts the request in flight.
        while (this.paused && entry.state === PAUSED) {
          await sleep(150, controller.signal);
        }
        if (controller.signal.aborted || entry.state === CANCELLED) {
          return;
        }

        const start = (chunk - 1) * step;
        const end = Math.min(start + step, entry.size);
        const blob = entry.file.slice(start, end);

        const form = new FormData();
        form.append("resumableFilename", entry.name);
        form.append("resumableRelativePath", entry.destDir);
        form.append("resumableIdentifier", entry.identifier);
        form.append("resumableChunkNumber", String(chunk));
        form.append("resumableTotalChunks", String(totalChunks));
        form.append("resumableTotalSize", String(entry.size));
        form.append("file", blob, entry.name);
        if (entry.onCollision) {
          form.append("resumableOnCollision", entry.onCollision);
        }

        const res = await fetch(url("/api/files/upload"), {
          method: "POST",
          credentials: "same-origin",
          headers: { "X-CSRF-Token": csrfToken() },
          body: form,
          signal: controller.signal,
        });

        if (!res.ok) {
          const failure = await errorFrom(res);
          // A name that is taken is a question, not a failure: ask once,
          // apply the answer to the rest of the file, and carry on from the
          // same chunk because nothing was stored.
          if (failure.status === 409 && !entry.onCollision) {
            const policy = this.collisionPolicy || (await this.resolveCollision(entry));
            if (!policy) {
              entry.state = CANCELLED;
              return;
            }
            entry.onCollision = policy;
            chunk -= 1;
            continue;
          }
          throw failure;
        }

        const body = await res.json().catch(() => null);
        entry.sent = end;
        entry.sample();
        if (body && body.data && body.data.entry) {
          this.onStored(entry, body.data.entry);
        }
        this.onChange();
      }

      entry.state = DONE;
      entry.sent = entry.size;
    } catch (err) {
      if (err && err.name === "AbortError") {
        // Cancel already set the state; a pause leaves it PAUSED.
        if (entry.state !== CANCELLED && entry.state !== PAUSED) {
          entry.state = FAILED;
          entry.error = "";
        }
        return;
      }
      entry.state = FAILED;
      entry.error = err instanceof ApiError ? err.message : "Upload failed";
    } finally {
      this.controllers.delete(entry.id);
      this.running.delete(entry.id);
      entry.resetSpeed();
      this.onChange();
      // Deferred: pump() is re-entrant through run(), and calling it while the
      // finishing entry is still in `running` was what left the next file
      // stuck at 0% behind an open collision dialog.
      queueMicrotask(() => this.pump());
    }
  }
}

/** Reads the server's error message from a failed response. */
async function errorFrom(res) {
  let message = res.statusText || "Upload failed";
  let body = null;
  try {
    body = await res.json();
  } catch {
    /* keep the status text */
  }
  if (body && body.error) {
    message = body.error.message;
  }
  return new ApiError(message, res.status, (body && body.error && body.error.code) || "upload_failed");
}
