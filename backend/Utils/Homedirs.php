<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Utils;

/**
 * Shared normalisation for the multi-folder homedirs list.
 *
 * Before this helper existed, the same "trim string entries, drop blanks
 * and non-strings, re-index" loop and the same "read either `homedirs`
 * array or legacy `homedir` scalar from an array-shaped row" extractor
 * were duplicated in five places: User::setHomedirs, the two JsonFile +
 * Database adapter row extractors, AuditMailer's snapshot extractor, and
 * AdminController's request normaliser. The AuditMailer copy silently
 * drifted — it forgot to trim — which would have produced misleading
 * subject lines for any homedir that ever picked up leading/trailing
 * whitespace. Centralising removes the drift surface.
 */
class Homedirs
{
    /**
     * Trim string entries, drop blanks and non-strings, re-index.
     * Returns $default when the cleaned list is empty.
     */
    public static function clean(array $list, array $default = []): array
    {
        $out = [];
        foreach ($list as $h) {
            if (! is_string($h)) continue;
            $t = trim($h);
            if ($t === '') continue;
            $out[] = $t;
        }
        return $out ?: $default;
    }

    /**
     * Read homedirs from an array-shaped row (users.json row,
     * jsonSerialize snapshot). Prefers the new `homedirs` array key;
     * falls back to wrapping the legacy `homedir` scalar; returns
     * $default when neither key has usable data.
     */
    public static function fromArrayRow(array $row, array $default = []): array
    {
        if (isset($row['homedirs']) && is_array($row['homedirs'])) {
            return self::clean($row['homedirs'], $default);
        }
        if (isset($row['homedir']) && is_string($row['homedir']) && trim($row['homedir']) !== '') {
            return [trim($row['homedir'])];
        }
        return $default;
    }
}
