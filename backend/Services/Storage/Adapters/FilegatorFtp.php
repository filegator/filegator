<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Storage\Adapters;

use League\Flysystem\Adapter\Ftp;
use League\Flysystem\NotSupportedException;

class FilegatorFtp extends Ftp
{

    /**
     * Normalize a file entry.
     *
     * @param string $item
     * @param string $base
     *
     * @return array normalized file array
     *
     * @throws NotSupportedException
     */
    protected function normalizeObject($item, $base)
    {
        $systemType = $this->systemType ?: $this->detectSystemType($item);

        if ($systemType === 'unix') {
            $result = $this->normalizeUnixObject($item, $base);
            return $this->afterNormalizeUnixObject($result, $item, $base);
        } elseif ($systemType === 'windows') {
            $result = $this->normalizeWindowsObject($item, $base);
            return $this->afterNormalizeWindowsObject($result, $item, $base);
        }

        throw NotSupportedException::forFtpSystemType($systemType);
    }

    /**
     * Normalize a Unix file entry, with permissions.
     *
     * Given $item contains:
     *    '-rw-r--r--   1 ftp      ftp           409 Aug 19 09:01 file1.txt'
     *
     * This function will return:
     * [
     *   'type' => 'file',
     *   'path' => 'file1.txt',
     *   'visibility' => 'public',
     *   'size' => 409,
     *   'timestamp' => 1566205260,
     *   'permissions' => 644
     * ]
     *
     * @param array $result original normalized file array
     * @param string $item
     * @param string $base
     *
     * @return array normalized file array
     */
    protected function afterNormalizeUnixObject($result, $item, $base)
    {
        $item = preg_replace('#\s+#', ' ', trim($item), 7);
        list($permissions, /* $number */, /* $owner */, /* $group */, $size, $month, $day, $timeOrYear, $name) = explode(' ', $item, 9);
        $permissions = $this->normalizePermissions($permissions);

        $result['permissions'] = decoct($permissions);
        return $result;
    }

    /**
     * Normalize a Windows/DOS file entry, with permissions.
     *
     * @param array $result original normalized file array
     * @param string $item
     * @param string $base
     *
     * @return array normalized file array
     */
    protected function afterNormalizeWindowsObject($result, $item, $base)
    {
        $result['permissions'] = 777;
        return $result;
    }

}