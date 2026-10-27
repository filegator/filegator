/**
 * Date and time formatting for the listing.
 *
 * The pattern is the administrator's `date_format`, written with the moment.js
 * tokens v7 used, so an upgraded installation keeps the format it always had
 * (`YY/MM/DD hh:mm:ss`). It is rendered here rather than through moment (which
 * v7 bundled and v8 may not) or through Intl (whose tokens are a different
 * language), so a pattern means the same thing on every browser.
 */

/** Default pattern: the one v7 shipped with, widened to four-digit years. */
export const DEFAULT_DATE_FORMAT = "YYYY/MM/DD HH:mm:ss";

/**
 * Longest alternatives first, so "YYYY" is not read as "YY" plus a leftover
 * "YY" and "MMMM" is not read as "M" four times. Matching is case-sensitive:
 * "MM" is the month, "mm" the minutes.
 */
const TOKEN =
  /YYYY|YY|MMMM|MMM|MM|M|DDDD|DDD|Do|DD|D|dddd|ddd|dd|d|HH|H|hh|h|mm|m|ss|s|SSS|A|a|ZZ|Z/y;

const MONTHS_LONG = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];
const MONTHS_SHORT = MONTHS_LONG.map((name) => name.slice(0, 3));
const DAYS_LONG = [
  "Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
];
const DAYS_SHORT = DAYS_LONG.map((name) => name.slice(0, 3));

/** Zero-pads a number to `width` digits. */
function pad(value, width) {
  return String(value).padStart(width, "0");
}

/** Returns the day-of-month with its ordinal suffix, e.g. "1st". */
function ordinal(day) {
  const mod100 = day % 100;
  if (mod100 >= 11 && mod100 <= 13) {
    return `${day}th`;
  }
  switch (day % 10) {
    case 1:
      return `${day}st`;
    case 2:
      return `${day}nd`;
    case 3:
      return `${day}rd`;
    default:
      return `${day}th`;
  }
}

/** Renders one token for a local Date. */
function renderToken(token, date) {
  const hours12 = date.getHours() % 12 || 12;
  switch (token) {
    case "YYYY":
      return String(date.getFullYear()).padStart(4, "0");
    case "YY":
      return pad(date.getFullYear() % 100, 2);
    case "M":
      return String(date.getMonth() + 1);
    case "MM":
      return pad(date.getMonth() + 1, 2);
    case "MMM":
      return MONTHS_SHORT[date.getMonth()];
    case "MMMM":
      return MONTHS_LONG[date.getMonth()];
    case "D":
      return String(date.getDate());
    case "DD":
      return pad(date.getDate(), 2);
    case "Do":
      return ordinal(date.getDate());
    // Moment's DDD/DDDD are the day of the year counted from ZERO, which is
    // odd but is what a v7 pattern means; DDDD pads to three digits.
    case "DDD":
      return String(dayOfYear(date) - 1);
    case "DDDD":
      return pad(dayOfYear(date) - 1, 3);
    case "H":
      return String(date.getHours());
    case "HH":
      return pad(date.getHours(), 2);
    case "h":
      return String(hours12);
    case "hh":
      return pad(hours12, 2);
    case "m":
      return String(date.getMinutes());
    case "mm":
      return pad(date.getMinutes(), 2);
    case "s":
      return String(date.getSeconds());
    case "ss":
      return pad(date.getSeconds(), 2);
    case "SSS":
      return pad(date.getMilliseconds(), 3);
    case "A":
      return date.getHours() < 12 ? "AM" : "PM";
    case "a":
      return date.getHours() < 12 ? "am" : "pm";
    case "Z":
      return zoneOffset(date, false);
    case "ZZ":
      return zoneOffset(date, true);
    case "dddd":
      return DAYS_LONG[date.getDay()];
    case "ddd":
      return DAYS_SHORT[date.getDay()];
    case "dd":
      return DAYS_LONG[date.getDay()].slice(0, 2);
    case "d":
      return String(date.getDay());
    default:
      return token;
  }
}

/** Returns the day of the year, 1-based. */
function dayOfYear(date) {
  const start = new Date(date.getFullYear(), 0, 1);
  // Local midnights, so a daylight-saving change does not add or lose a day.
  const days = Math.round((new Date(date.getFullYear(), date.getMonth(), date.getDate()) - start) / 86400000);
  return days + 1;
}

/** Returns the offset from UTC as "+0200" (or "+02:00" when colon is set). */
function zoneOffset(date, colon) {
  const minutes = -date.getTimezoneOffset();
  const sign = minutes < 0 ? "-" : "+";
  const abs = Math.abs(minutes);
  return `${sign}${pad(Math.floor(abs / 60), 2)}${colon ? ":" : ""}${pad(abs % 60, 2)}`;
}

/**
 * Formats a date with a moment-style pattern.
 *
 * Text inside square brackets is printed literally (`[at]`), and a backslash
 * escapes the next character, so a pattern can carry words that would
 * otherwise be read as tokens.
 */
export function formatDatePattern(date, pattern) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    return "";
  }
  const source = pattern || DEFAULT_DATE_FORMAT;
  let out = "";
  for (let i = 0; i < source.length; i++) {
    const char = source[i];
    if (char === "[") {
      const close = source.indexOf("]", i + 1);
      if (close !== -1) {
        out += source.slice(i + 1, close);
        i = close;
        continue;
      }
    }
    if (char === "\\" && i + 1 < source.length) {
      out += source[i + 1];
      i++;
      continue;
    }
    // Sticky, so a match must start exactly here; a letter that is not a
    // token prints as itself.
    TOKEN.lastIndex = i;
    const match = TOKEN.exec(source);
    if (match) {
      out += renderToken(match[0], date);
      i += match[0].length - 1;
      continue;
    }
    out += char;
  }
  return out;
}

/**
 * Formats an RFC 3339 timestamp for display, e.g. "2026/10/26 09:41:00".
 *
 * The pattern comes from the administrator (`frontend_config.date_format`); an
 * empty or unusable one falls back to the default rather than printing blank
 * dates.
 */
export function formatDate(iso, pattern = DEFAULT_DATE_FORMAT) {
  const date = new Date(iso);
  return formatDatePattern(date, pattern);
}
