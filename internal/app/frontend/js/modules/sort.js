/**
 * Column sorting for the file listing.
 *
 * v7 let the table be sorted by name, permission, size or time, with folders
 * always grouped before files and names compared case-insensitively. Keeping
 * this in a module means it can be exercised without a DOM.
 */

/** Sort keys the table offers. There is no "mode": the column was removed. */
export const SORT_KEYS = ["name", "size", "time"];

/**
 * Returns a comparator for a listing.
 *
 * key   - one of SORT_KEYS
 * desc  - descending when true
 *
 * Folders always come first whatever the key: sorting by size would otherwise
 * bury every folder below the files, which is not how a file manager reads.
 * Equal values fall back to the name, so the order is stable rather than
 * whatever the server happened to send.
 */
export function comparator(key, desc) {
  const direction = desc ? -1 : 1;

  return (a, b) => {
    // Folders first, independently of the sort direction.
    if (a.type !== b.type) {
      if (a.type === "dir") {
        return -1;
      }
      if (b.type === "dir") {
        return 1;
      }
    }

    let result = 0;
    switch (key) {
      case "size":
        result = compareNumbers(a.size || 0, b.size || 0);
        break;
      case "time":
        result = compareNumbers(Date.parse(a.modified) || 0, Date.parse(b.modified) || 0);
        break;
      case "name":
      default:
        result = compareNames(a.name, b.name);
        break;
    }

    if (result === 0) {
      // A stable tie-break, but never undo the folder grouping above.
      result = compareNames(a.name, b.name) * direction;
      return result;
    }
    return result * direction;
  };
}

/** Compares two numbers. */
function compareNumbers(a, b) {
  if (a === b) {
    return 0;
  }
  return a < b ? -1 : 1;
}

/** Compares names the way a file manager does: case-insensitively. */
function compareNames(a, b) {
  return String(a || "").localeCompare(String(b || ""), undefined, {
    sensitivity: "base",
    numeric: true,
  });
}

/**
 * Returns a sorted copy of entries.
 *
 * The input is not modified: the listing in memory is reused for "show hidden"
 * and must keep the server's order.
 */
export function sortEntries(entries, key, desc) {
  return [...entries].sort(comparator(key, desc));
}

/**
 * Returns the direction the next click should use for a column.
 *
 * Name starts ascending (what people expect); the numeric columns start
 * descending, so one click on Size shows the biggest first.
 */
export function nextDirection(key, currentKey, currentDesc) {
  if (key !== currentKey) {
    return key !== "name";
  }
  return !currentDesc;
}
