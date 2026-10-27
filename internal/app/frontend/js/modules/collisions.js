/**
 * Resolves a name collision by asking the user and retrying.
 *
 * The server refuses with 409 and never renames on its own. This wraps that:
 * ask once, remember the answer for the rest of the batch, and retry with the
 * policy the server understands.
 */

import { ApiError } from "./api.js";
import { askCollision, CANCEL, KEEP_BOTH, REPLACE, SKIP } from "./collision.js";

/** Applies one answer to the remaining collisions in this batch. */
export class CollisionSession {
  /**
   * options:
   *   allowReplace - offer "Replace" at all (false for folders)
   */
  constructor({ allowReplace = true } = {}) {
    this.allowReplace = allowReplace;
    /** "replace" | "upcount" | null (ask each time) */
    this.policy = null;
    /** True once the user chose Skip for all. */
    this.skipAll = false;
  }

  /**
   * Runs an operation that may collide, asking when it does.
   *
   * label - the name shown in the prompt
   * run   - (policy) => promise; policy is one of the server's values
   *
   * Resolves to the operation's result, or null when skipped or cancelled.
   */
  async run(label, run) {
    if (this.skipAll) {
      return null;
    }
    if (this.policy) {
      return run(this.policy);
    }

    try {
      return await run(undefined);
    } catch (err) {
      if (!(err instanceof ApiError) || err.status !== 409) {
        throw err;
      }
    }

    const answer = await askCollision({
      name: label,
      batch: true,
      allowReplace: this.allowReplace,
    });
    if (answer.applyToAll) {
      if (answer.action === SKIP || answer.action === CANCEL) {
        this.skipAll = true;
        return null;
      }
      this.policy = answer.action;
    }

    switch (answer.action) {
      case REPLACE:
        if (!this.allowReplace) {
          return null;
        }
        return run(answer.action);
      case KEEP_BOTH:
        return run(answer.action);
      case SKIP:
      case CANCEL:
      default:
        return null;
    }
  }
}

/** Re-exported so callers do not import from two modules. */
export { REPLACE, KEEP_BOTH, SKIP, CANCEL };
