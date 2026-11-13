<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Http\Resources\Json\ResourceCollection;

class TeamCollection extends ResourceCollection
{
    public $collects = UserResource::class;
}
