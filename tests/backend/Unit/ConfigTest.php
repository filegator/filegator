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

        $this->assertEquals($sample, $config->get());
        $this->assertEquals('something', $config->get('test'));
        $this->assertEquals(123, $config->get('test2.deep'));
        $this->assertEquals(2, $config->get('test3.sub.subsub'));
        $this->assertNull($config->get('not-found'));
        $this->assertEquals('default', $config->get('not-found', 'default'));
        $this->assertEquals('default', $config->get('not.found', 'default'));
    }
}
