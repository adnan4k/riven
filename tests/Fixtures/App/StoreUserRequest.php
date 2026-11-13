<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Foundation\Http\FormRequest;

class StoreUserRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'email' => 'required|email',
            'name' => ['required', 'string', 'max:255'],
        ];
    }
}
