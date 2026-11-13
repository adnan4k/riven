<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Http\Resources\Json\AnonymousResourceCollection;

/**
 * One method per return style ResourceDetector must handle. Never executed, only parsed.
 */
class ResourceController
{
    public function typed(): UserResource
    {
        return $this->build();
    }

    public function newResource()
    {
        return new UserResource([]);
    }

    public function make()
    {
        return UserResource::make([]);
    }

    public function collection()
    {
        return UserResource::collection([]);
    }

    public function anonymousCollectionType(): AnonymousResourceCollection
    {
        return UserResource::collection([]);
    }

    public function namedCollection(): UserCollection
    {
        return new UserCollection([]);
    }

    public function explicitCollects()
    {
        return new TeamCollection([]);
    }

    public function chained()
    {
        return (new UserResource([]))->additional(['meta' => 1])->response()->setStatusCode(201);
    }

    public function sameEverywhere(bool $flag)
    {
        if ($flag) {
            return new UserResource([]);
        }

        return UserResource::make([]);
    }

    public function conflicting(bool $flag)
    {
        if ($flag) {
            return new UserResource([]);
        }

        return UserResource::collection([]);
    }

    public function partlyUnknown(bool $flag)
    {
        if ($flag) {
            return new UserResource([]);
        }

        return response()->noContent();
    }

    public function nestedClosureOnly()
    {
        $build = fn () => new UserResource([]);

        return $build();
    }

    public function notAResource()
    {
        return new \ArrayObject([]);
    }

    public function plainArray()
    {
        return ['ok' => true];
    }

    private function build(): UserResource
    {
        return new UserResource([]);
    }
}
