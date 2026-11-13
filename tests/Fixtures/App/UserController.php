<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Http\Request;

class UserController
{
    public function index(Request $request, int $page) {}

    public function store(StoreUserRequest $request) {}

    public function update(int $id, ?StoreUserRequest $request) {}
}
