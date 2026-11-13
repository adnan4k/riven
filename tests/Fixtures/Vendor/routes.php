<?php

use Illuminate\Support\Facades\Route;
use Riven\Tests\Fixtures\Vendor\VendorController;

// Routes a package provider would register. Their actions are defined in this
// directory, which the tests pass to RouteScanner as the vendor path.
Route::get('vendor-closure', fn () => 'vendor');
Route::get('vendor-controller', [VendorController::class, 'show']);
