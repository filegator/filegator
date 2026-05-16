<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Tmpfs\Adapters;

use Filegator\Services\Service;
use Filegator\Services\Tmpfs\TmpfsInterface;

class Tmpfs implements Service, TmpfsInterface
{
    protected $path;

    public function init(array $config = [])
    {
        $this->path = $config['path'];

        if (! is_dir($this->path)) {
            mkdir($this->path);
        }

        if (mt_rand(0, 99) < $config['gc_probability_perc']) {
            $this->clean($config['gc_older_than']);
        }
    }

    public function write(string $filename, $data, $append = false)
    {
        $filename = $this->sanitizeFilename($filename);

        $flags = 0;

        if ($append) {
            $flags = FILE_APPEND;
        }

        file_put_contents($this->getPath().$filename, $data, $flags);
    }

    public function getFileLocation(string $filename): string
    {
        $filename = $this->sanitizeFilename($filename);

        return $this->getPath().$filename;
    }

    public function read(string $filename): string
    {
        $filename = $this->sanitizeFilename($filename);

        return (string) file_get_contents($this->getPath().$filename);
    }

    public function readStream(string $filename): array
    {
        $filename = $this->sanitizeFilename($filename);

        $stream = fopen($this->getPath().$filename, 'r');
        $filesize = filesize($this->getPath().$filename);

        return [
            'filename' => $filename,
            'stream' => $stream,
            'filesize' => $filesize,
        ];
    }

    public function exists(string $filename): bool
    {
        $filename = $this->sanitizeFilename($filename);

        return file_exists($this->getPath().$filename);
    }

    public function findAll($pattern): array
    {
        $files = [];
        $matches = glob($this->getPath().$pattern);
        if (! empty($matches)) {
            foreach ($matches as $filename) {
                if (is_file($filename)) {
                    $files[] = [
                        'name' => basename($filename),
                        'size' => filesize($filename),
                        'time' => filemtime($filename),
                    ];
                }
            }
        }

        return $files;
    }

    public function remove(string $filename)
    {
        $filename = $this->sanitizeFilename($filename);

        unlink($this->getPath().$filename);
    }

    public function clean(int $older_than)
    {
        $files = $this->findAll('*');
        foreach ($files as $file) {
            if (time() - $file['time'] >= $older_than) {
                $this->remove($file['name']);
            }
        }
    }

    public function addIfAbsent(string $filename, string $data = '1'): bool
    {
        $filename = $this->sanitizeFilename($filename);
        $path = $this->getPath().$filename;

        // 'x' mode opens for write IFF the file does NOT already exist. The
        // check-and-create is a single atomic syscall (O_CREAT|O_EXCL), so
        // two concurrent callers cannot both observe absence and both create.
        $fh = @fopen($path, 'x');
        if ($fh === false) return false;
        try {
            fwrite($fh, $data);
            fflush($fh);
        } finally {
            fclose($fh);
        }
        return true;
    }

    public function incrementCounterIfBelow(string $filename, int $max): int
    {
        $filename = $this->sanitizeFilename($filename);
        $path = $this->getPath().$filename;

        $fh = @fopen($path, 'c+');
        if ($fh === false) {
            return -1; // cannot open; treat as limit-exceeded to fail closed
        }
        if (! flock($fh, LOCK_EX)) {
            fclose($fh);
            return -1;
        }
        try {
            rewind($fh);
            $contents = stream_get_contents($fh);
            $current = ($contents === false) ? 0 : strlen($contents);
            if ($current >= $max) {
                return -1;
            }
            // Append one byte while still holding the lock.
            fseek($fh, 0, SEEK_END);
            fwrite($fh, 'x');
            fflush($fh);
            return $current + 1;
        } finally {
            @flock($fh, LOCK_UN);
            @fclose($fh);
        }
    }

    private function getPath(): string
    {
        return $this->path;
    }

    private function sanitizeFilename($filename)
    {
        $filename = (string) preg_replace(
            '~
            [<>:"/\\|?*]|    # file system reserved https://en.wikipedia.org/wiki/Filename#Reserved_characters_and_words
            [\x00-\x1F]|     # control characters http://msdn.microsoft.com/en-us/library/windows/desktop/aa365247%28v=vs.85%29.aspx
            [\x7F\xA0\xAD]|  # non-printing characters DEL, NO-BREAK SPACE, SOFT HYPHEN
            [;\\\{}^\~`]     # other non-safe
            ~xu',
            '-',
            (string) $filename
        );

        // maximize filename length to 255 bytes http://serverfault.com/a/9548/44086
        return mb_substr($filename, 0, 255);
    }
}
