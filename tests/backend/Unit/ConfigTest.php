<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Config\Config;
use Tests\TestCase;

/**
 * @internal
 */
class ConfigTest extends TestCase
{
    public function testGettingAnItemFromConfigUsingDotNotation()
    {
        $sample = [
            'test' => 'something',
            'test2' => [
                'deep' => 123,
            ],
            'test3' => [
                'sub' => [
                    'subsub' => 2,
                ],
            ],
        ];

        $config = new Config($sample);

        $this->assertEquals($config->get(), $sample);
        $this->assertEquals($config->get('test'), 'something');
        $this->assertEquals($config->get('test2.deep'), 123);
        $this->assertEquals($config->get('test3.sub.subsub'), 2);
        $this->assertEquals($config->get('not-found'), null);
        $this->assertEquals($config->get('not-found', 'default'), 'default');
        $this->assertEquals($config->get('not.found', 'default'), 'default');
    }
}
