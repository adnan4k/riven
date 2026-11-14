<?php

namespace Riven\Tests;

/**
 * Boots the app with the dashboard on and the fake engine
 * (tests/Fixtures/fake-engine) in place of riven-engine.
 */
abstract class DashboardTestCase extends TestCase
{
    protected string $dir;

    protected function defineEnvironment($app): void
    {
        $this->dir = sys_get_temp_dir().'/riven-dashboard-'.uniqid();
        mkdir($this->dir);

        $app['config']->set('app.url', 'http://app.test:8080');
        $app['config']->set('riven.dashboard.enabled', true);
        $app['config']->set('riven.engine.path', __DIR__.'/Fixtures/fake-engine');
        $app['config']->set('riven.dashboard.storage', $this->dir.'/storage');
        $app['config']->set('riven.dashboard.reports', $this->dir.'/reports');
    }

    protected function tearDown(): void
    {
        exec('rm -rf '.escapeshellarg($this->dir));
        parent::tearDown();
    }
}
