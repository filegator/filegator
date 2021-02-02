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
