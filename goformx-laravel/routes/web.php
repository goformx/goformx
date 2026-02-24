<?php

use App\Http\Controllers\DemoController;
use App\Http\Controllers\FormController;
use App\Http\Controllers\PublicFormController;
use Illuminate\Support\Facades\Route;
use Inertia\Inertia;
use Laravel\Fortify\Features;
use Symfony\Component\HttpFoundation\Response;

Route::get('robots.txt', function (): Response {
    $appUrl = rtrim(config('app.url'), '/');

    return response("User-agent: *\nDisallow:\n\nSitemap: {$appUrl}/sitemap.xml\n", 200, [
        'Content-Type' => 'text/plain',
    ]);
})->name('robots');

Route::get('sitemap.xml', function (): Response {
    $appUrl = rtrim(config('app.url'), '/');
    $lastmod = now()->toAtomString();

    $urls = [
        ['loc' => $appUrl.'/', 'lastmod' => $lastmod],
        ['loc' => $appUrl.'/demo', 'lastmod' => $lastmod],
    ];

    $xml = '<?xml version="1.0" encoding="UTF-8"?>'."\n";
    $xml .= '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'."\n";
    foreach ($urls as $url) {
        $xml .= '  <url>'."\n";
        $xml .= '    <loc>'.e($url['loc']).'</loc>'."\n";
        $xml .= '    <lastmod>'.e($url['lastmod']).'</lastmod>'."\n";
        $xml .= '  </url>'."\n";
    }
    $xml .= '</urlset>';

    return response($xml, 200, [
        'Content-Type' => 'application/xml',
    ]);
})->name('sitemap');

Route::get('demo', DemoController::class)->name('demo');

Route::get('/', function () {
    return Inertia::render('Home', [
        'canRegister' => Features::enabled(Features::registration()),
    ]);
})->name('home');

Route::get('dashboard', function () {
    return Inertia::render('Dashboard');
})->middleware(['auth', 'verified'])->name('dashboard');

// Exact paths must be registered before parameterized forms/{id} so GET/POST /forms match
Route::get('forms', [FormController::class, 'index'])
    ->middleware(['auth', 'verified'])
    ->name('forms.index');
Route::post('forms', [FormController::class, 'store'])
    ->middleware(['auth', 'verified'])
    ->name('forms.store');

Route::middleware(['auth', 'verified'])->group(function () {
    Route::get('forms/{id}/edit', [FormController::class, 'edit'])->name('forms.edit');
    Route::get('forms/{id}/preview', [FormController::class, 'preview'])->name('forms.preview');
    Route::get('forms/{id}/submissions', [FormController::class, 'submissions'])->name('forms.submissions');
    Route::get('forms/{id}/submissions/{sid}', [FormController::class, 'submission'])->name('forms.submissions.show');
    Route::get('forms/{id}/embed', [FormController::class, 'embed'])->name('forms.embed');
    Route::put('forms/{id}', [FormController::class, 'update'])->name('forms.update');
    Route::delete('forms/{id}', [FormController::class, 'destroy'])->name('forms.destroy');
});

Route::get('forms/{id}', [PublicFormController::class, 'show'])->name('forms.fill');

require __DIR__.'/settings.php';
