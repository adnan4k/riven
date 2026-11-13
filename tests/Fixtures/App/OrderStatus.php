<?php

namespace Riven\Tests\Fixtures\App;

enum OrderStatus: string
{
    case Pending = 'pending';
    case Paid = 'paid';
}
