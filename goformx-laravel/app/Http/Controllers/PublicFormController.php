<?php

namespace App\Http\Controllers;

use Inertia\Inertia;
use Inertia\Response;

class PublicFormController extends Controller
{
    /**
     * Show the public form-fill page. Schema is loaded from Go by the client.
     */
    public function show(string $id): Response
    {
        return Inertia::render('Forms/Fill', [
            'formId' => $id,
        ]);
    }
}
