<?php

$finder = PhpCsFixer\Finder::create()
    ->exclude('vendor')
    ->in(__DIR__)
;

return PhpCsFixer\Config::create()
    ->setUsingCache(false)
    ->setRules([
        '@PhpCsFixer' => true,
        'yoda_style' => null,
        'not_operator_with_successor_space' => true,
        'php_unit_test_class_requires_covers' => false,
    ])
    ->setFinder($finder)
;
