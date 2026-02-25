<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class PricingController extends Controller
{
    public function __invoke(Request $request): Response
    {
        return Inertia::render('Pricing', [
            'currentTier' => $request->user()?->planTier() ?? 'free',
            'prices' => config('services.stripe.prices'),
        ]);
    }
}
