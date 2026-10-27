/**
 * Permission bits: reading a mode as checkboxes and building one back.
 *
 * A mode is three octal digits — owner, group, other — each the sum of read
 * (4), write (2) and execute (1). v7 showed these as a checkbox grid and let
 * the number be typed directly; both are kept.
 */

/** One permission triplet, in display order. */
export const SCOPES = ["owner", "group", "other"];

/** The bits within a triplet. */
export const BITS = ["read", "write", "execute"];

/** Bit values. */
const BIT_VALUE = { read: 4, write: 2, execute: 1 };

/**
 * Parses a mode string into a number.
 *
 * Accepts what v7's API sent ("755") and what this API returns ("644"). The
 * input is restricted to three octal digits: anything above 777 would mean
 * setuid, setgid or sticky bits, which v7 refused and this refuses too.
 */
export function parseMode(value) {
  const text = String(value ?? "").trim();
  // Leading zeros are allowed ("0644", v7's style) but "000" is a real mode —
  // nobody may access it — so the digits are matched before trimming.
  if (!/^[0-7]{1,4}$/.test(text)) {
    return null;
  }
  const parsed = parseInt(text, 8);
  return parsed <= 0o777 ? parsed : null;
}

/** Renders a number as three octal digits. */
export function formatMode(mode) {
  if (!Number.isInteger(mode) || mode < 0 || mode > 0o777) {
    return "";
  }
  return mode.toString(8).padStart(3, "0");
}

/**
 * Reports whether a bit is set.
 *
 * scope - "owner" | "group" | "other"
 * bit   - "read" | "write" | "execute"
 */
export function hasBit(mode, scope, bit) {
  const shift = { owner: 6, group: 3, other: 0 }[scope];
  if (shift === undefined || !(bit in BIT_VALUE)) {
    return false;
  }
  return ((mode >> shift) & BIT_VALUE[bit]) !== 0;
}

/** Returns a new mode with one bit set or cleared. */
export function setBit(mode, scope, bit, on) {
  const shift = { owner: 6, group: 3, other: 0 }[scope];
  if (shift === undefined || !(bit in BIT_VALUE)) {
    return mode;
  }
  const mask = BIT_VALUE[bit] << shift;
  return on ? mode | mask : mode & ~mask & 0o777;
}

/**
 * Describes a mode the way `ls -l` does: "rwxr-xr--".
 *
 * Needed because the listing shows the octal number, and the two are easy to
 * confuse: 644 is rw-r--r--, not rwxr-xr-x.
 */
export function symbolic(mode) {
  let out = "";
  for (const scope of SCOPES) {
    for (const bit of BITS) {
      out += hasBit(mode, scope, bit) ? "rwx"[BITS.indexOf(bit)] : "-";
    }
  }
  return out;
}

/** The two "recursive" choices: what a directory change applies to. */
export const RECURSIVE = {
  none: null,
  all: "all",
  folders: "folders",
  files: "files",
};
