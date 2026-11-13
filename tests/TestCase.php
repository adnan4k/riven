<?php

namespace Riven\Tests;

use Orchestra\Testbench\TestCase as BaseTestCase;
use Riven\RivenServiceProvider;

abstract class TestCase extends BaseTestCase
{
    protected function getPackageProviders($app): array
    {
        return [RivenServiceProvider::class];
    }
}
