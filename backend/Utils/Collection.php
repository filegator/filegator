<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Utils;

trait Collection
{
    protected $items = [];

    public function add($obj)
    {
        return $this->items[] = $obj;
    }

    public function delete($obj)
    {
        foreach ($this->items as $key => $item) {
            if ($item === $obj) {
                unset($this->items[$key]);
            }
        }
    }

    public function all()
    {
        return $this->items;
    }

    public function length()
    {
        return count($this->items);
    }

    public function jsonSerialize()
    {
        return $this->items;
    }

    public function sortByValue($value, $desc = false)
    {
        usort($this->items, function ($a, $b) use ($value) {
            return $a[$value] <=> $b[$value];
        });

        if ($desc) {
            $this->items = array_reverse($this->items);
        }

        return $this;
    }
}
