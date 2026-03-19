<?php

declare(strict_types=1);

namespace GoFormX\Service;

use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Inertia\InertiaResponse;

/**
 * Renders an InertiaResponse as a full HTML page with Vite dev/prod asset tags.
 *
 * Used for initial page loads. XHR Inertia requests (with X-Inertia header)
 * are handled by the ControllerDispatcher as JSON — this is only for the
 * first visit that needs a full HTML document.
 */
final class InertiaRenderer
{
    public function __construct(
        private readonly bool $isDev = true,
        private readonly string $viteDevUrl = 'http://localhost:5173',
    ) {
    }

    public function render(InertiaResponse $inertiaResponse, string $requestUri): Response
    {
        $pageObject = $inertiaResponse->toPageObject();
        $pageObject['url'] = $requestUri;

        $json = json_encode(
            $pageObject,
            JSON_THROW_ON_ERROR | JSON_HEX_TAG | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_HEX_AMP | JSON_UNESCAPED_UNICODE,
        );

        $pageScript = '<script type="application/json" data-page="true">' . $json . '</script>';

        if ($this->isDev) {
            $assetTags = <<<HTML
                <script type="module" src="{$this->viteDevUrl}/@vite/client"></script>
                <script type="module" src="{$this->viteDevUrl}/src/app.ts"></script>
            HTML;
        } else {
            $assetTags = $this->productionAssetTags();
        }

        $html = <<<HTML
        <!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="utf-8">
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <title>{$this->escapeHtml($pageObject['component'] ?? 'GoFormX')} - GoFormX</title>
            {$assetTags}
        </head>
        <body class="font-sans antialiased">
            <div id="app">
            </div>
            {$pageScript}
        </body>
        </html>
        HTML;

        return new Response($html, 200, ['Content-Type' => 'text/html; charset=UTF-8']);
    }

    private function productionAssetTags(): string
    {
        $manifestPath = dirname(__DIR__, 2) . '/public/build/.vite/manifest.json';
        if (!is_file($manifestPath)) {
            return '<!-- No Vite manifest found -->';
        }

        $manifest = json_decode(file_get_contents($manifestPath), true, 512, JSON_THROW_ON_ERROR);
        $entry = $manifest['src/app.ts'] ?? null;
        if ($entry === null) {
            return '<!-- app.ts not found in manifest -->';
        }

        $tags = '';
        if (isset($entry['css'])) {
            foreach ($entry['css'] as $css) {
                $tags .= '<link rel="stylesheet" href="/build/' . $css . '">' . "\n";
            }
        }
        $tags .= '<script type="module" src="/build/' . $entry['file'] . '"></script>';

        return $tags;
    }

    private function escapeHtml(string $value): string
    {
        return htmlspecialchars($value, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8');
    }
}
