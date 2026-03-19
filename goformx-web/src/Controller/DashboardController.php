<?php

declare(strict_types=1);

namespace GoFormX\Controller;

use Symfony\Component\HttpFoundation\Request;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

final class DashboardController
{
    public function index(Request $request): InertiaResponse
    {
        return Inertia::render('Dashboard', []);
    }
}
