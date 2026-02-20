<?php

namespace App\Http\Controllers;

use Inertia\Inertia;
use Inertia\Response;

class DemoController extends Controller
{
    public function __invoke(): Response
    {
        $demoFormId = config('services.goforms.demo_form_id');

        if (empty($demoFormId)) {
            return Inertia::render('DemoUnconfigured');
        }

        return Inertia::render('Demo', ['formId' => $demoFormId]);
    }
}
