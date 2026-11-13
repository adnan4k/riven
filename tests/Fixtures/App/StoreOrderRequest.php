<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Foundation\Http\FormRequest;
use Illuminate\Validation\Rule;
use Illuminate\Validation\Rules\Password;

class StoreOrderRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'customer_id' => 'required|integer|exists:customers,id',
            'email' => ['required', 'email', 'unique:customers,email'],
            'status' => ['required', Rule::enum(OrderStatus::class)],
            'items' => 'required|array|min:2',
            'items.*.product_id' => 'required|integer|exists:products,id',
            'items.*.qty' => 'required|integer|between:1,5',
            'shipping.city' => 'required|string|max:40',
            'coupon' => 'nullable|string|max:8',
            'priority' => 'required|in:low,normal,high',
            'reference' => ['required', 'regex:/^ORD-[0-9]{4}$/'],
            'pin' => 'required|digits:4',
            'deliver_on' => 'required|date|after:today',
            'password' => ['required', 'confirmed', Password::defaults()],
            'terms' => 'accepted',
            'avatar' => 'nullable|image',
        ];
    }
}
