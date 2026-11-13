<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Config\Repository;
use Illuminate\Foundation\Http\FormRequest;
use Illuminate\Validation\Rule;
use Illuminate\Validation\Rules\Password;

class RuleObjectsRequest extends FormRequest
{
    // Dependencies are injected into rules(), like Laravel does.
    public function rules(Repository $config): array
    {
        return [
            'role' => ['required', Rule::in(['admin', 'editor'])],
            'email' => ['required', Rule::unique('users')],
            'title' => Rule::string()->min(3),
            'password' => ['required', Password::min(8)],
            'code' => ['required', new Uppercase],
            'nickname' => ['nullable', fn ($attribute, $value, $fail) => null],
            'items.*.id' => 'required|integer',
            'empty_pipes' => 'required||string',
        ];
    }
}
