<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Foundation\Http\FormRequest;
use Illuminate\Validation\Rule;

/**
 * Needs the bound route parameter, which only exists during a real request.
 */
class UpdateUserRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'email' => ['required', Rule::unique('users')->ignore($this->route('user')->id)],
        ];
    }
}
