<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Config;

class Config
{
    protected $config;

    public function __construct(array $config = [])
    {
        $this->config = $config;

        $timezone = isset($this->config['timezone']) ? $this->config['timezone'] : 'UTC';
        date_default_timezone_set($timezone);
    }

    public function get($key = null, $default = null)
    {
        if (is_null($key)) {
            return $this->config;
        }

        $key = is_array($key) ? $key : explode('.', $key);

        $target = $this->config;

        while (! is_null($segment = array_shift($key))) {
            if (is_array($target) && array_key_exists($segment, $target)) {
                $target = $target[$segment];
            } else {
                return $default;
            }
        }

        return $target;
    }
}
