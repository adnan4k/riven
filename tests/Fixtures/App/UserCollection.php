<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Http\Resources\Json\ResourceCollection;

// No $collects: Laravel infers UserResource from the name, and so should we.
class UserCollection extends ResourceCollection {}
