<?php

use Illuminate\Support\Facades\Route;
use Riven\Http\ApiController;
use Riven\Http\DashboardController;
use Riven\Http\Middleware\GuardDashboardWrites;

// The dashboard at APP_URL/riven. Loaded by RivenServiceProvider only when
// riven.dashboard.enabled (by default: APP_ENV=local).
Route::prefix(config('riven.dashboard.path'))
    ->middleware(config('riven.dashboard.middleware', []))
    ->name('riven.')
    ->group(function () {
        Route::prefix('api')->name('api.')->group(function () {
            Route::get('config', [ApiController::class, 'config'])->name('config');
            Route::get('manifest', [ApiController::class, 'manifest'])->name('manifest');
            Route::get('openapi', [ApiController::class, 'openapi'])->name('openapi');
            Route::get('reports', [ApiController::class, 'reports'])->name('reports');
            Route::get('reports/{name}', [ApiController::class, 'report'])->name('report');
            Route::get('stress', [ApiController::class, 'stressState'])->name('stress');

            Route::middleware(GuardDashboardWrites::class)->group(function () {
                Route::post('try', [ApiController::class, 'try'])->name('try');
                Route::post('fake', [ApiController::class, 'fake'])->name('fake');
                Route::post('rescan', [ApiController::class, 'rescan'])->name('rescan');
                Route::post('stress', [ApiController::class, 'startStress'])->name('stress.start');
                Route::post('stress/stop', [ApiController::class, 'stopStress'])->name('stress.stop');
            });

            Route::any('{any}', [ApiController::class, 'notFound'])->where('any', '.*');
        });

        Route::get('{path?}', DashboardController::class)->where('path', '.*')->name('dashboard');
    });
