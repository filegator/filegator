/**
 * Entry filtering: the administrator's `filter_entries` hide matching names
 * from a listing until the user asks to see them.
 *
 * Ported from v7, where a pattern is matched against either the file name or
 * its full path:
 *   ".git/"        a folder called .git, anywhere
 *   "Thumbs.db"    that file, anywhere
 *   "/archive/*"   anything directly inside /archive
 * A leading "/" means the pattern is a full path; a trailing "/" means the
 * entry must be a folder. `*` and `?` are wildcards, everything else is
 * literal.
 */

/** Builds a matcher from one configured pattern, or null when unusable. */
function compile(pattern) {
  const raw = String(pattern || "").trim();
  if (!raw) {
    return null;
  }
  const wantsDir = raw.endsWith("/");
  const isPath = raw.startsWith("/");
  const body = raw.replace(/\/$/, "");

  // Wildcards are rewritten, everything else is escaped, so a name containing
  // a dot or a bracket cannot turn into a regex.
  const source = body
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")
    .replace(/\*/g, "[^/]*")
    .replace(/\?/g, "[^/]");
  let regex;
  try {
    regex = new RegExp(`^${source}$`, "iu");
  } catch {
    return null;
  }
  return { wantsDir, isPath, regex, body };
}

/**
 * Returns a predicate that reports whether an entry should be hidden.
 *
 * patterns - the configured filter_entries
 */
export function hiddenMatcher(patterns) {
  const rules = (patterns || []).map(compile).filter(Boolean);
  if (!rules.length) {
    return () => false;
  }
  return (entry) => {
    const isDir = entry.type === "dir";
    const name = entry.name || "";
    const path = entry.path || "";
    for (const rule of rules) {
      if (rule.wantsDir && !isDir) {
        continue;
      }
      // A full-path pattern is compared against the absolute path; a plain one
      // against the entry's own name, which is how v7 behaved.
      const subject = rule.isPath ? path : name;
      if (rule.regex.test(subject)) {
        return true;
      }
    }
    return false;
  };
}

/**
 * Splits a listing's entries into visible and hidden ones.
 *
 * Returns { visible, hiddenCount } so the caller can offer "show hidden"
 * without recomputing anything.
 */
export function partition(entries, patterns) {
  const isHidden = hiddenMatcher(patterns);
  const visible = [];
  let hiddenCount = 0;
  for (const entry of entries) {
    if (isHidden(entry)) {
      hiddenCount += 1;
    } else {
      visible.push(entry);
    }
  }
  return { visible, hiddenCount };
}
