<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Illuminate\Support\Facades\File;
use Inertia\Inertia;
use Inertia\Response;
use League\CommonMark\Environment\Environment;
use League\CommonMark\Extension\CommonMark\CommonMarkCoreExtension;
use League\CommonMark\Extension\FrontMatter\FrontMatterExtension;
use League\CommonMark\Extension\FrontMatter\Output\RenderedContentWithFrontMatter;
use League\CommonMark\MarkdownConverter;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

class DocsController extends Controller
{
    private const DEFAULT_SLUG = 'getting-started';

    public function __invoke(Request $request, ?string $slug = null): Response
    {
        $slug = $slug ?? self::DEFAULT_SLUG;
        $path = resource_path("docs/{$slug}.md");

        if (! File::exists($path)) {
            throw new NotFoundHttpException;
        }

        $converter = $this->makeConverter();

        $result = $converter->convert(File::get($path));
        $frontMatter = $result instanceof RenderedContentWithFrontMatter
            ? $result->getFrontMatter()
            : [];

        return Inertia::render('Docs/Show', [
            'title' => $frontMatter['title'] ?? $slug,
            'content' => $result->getContent(),
            'slug' => $slug,
            'navigation' => $this->buildNavigation($slug),
        ]);
    }

    /** @return array<int, array{title: string, slug: string, active: bool}> */
    private function buildNavigation(string $activeSlug): array
    {
        $docsPath = resource_path('docs');
        $files = File::glob("{$docsPath}/*.md");
        $nav = [];

        $converter = $this->makeConverter();

        foreach ($files as $file) {
            $fileSlug = pathinfo($file, PATHINFO_FILENAME);
            $result = $converter->convert(File::get($file));
            $frontMatter = $result instanceof RenderedContentWithFrontMatter
                ? $result->getFrontMatter()
                : [];

            $nav[] = [
                'title' => $frontMatter['title'] ?? $fileSlug,
                'slug' => $fileSlug,
                'order' => $frontMatter['order'] ?? 99,
                'active' => $fileSlug === $activeSlug,
            ];
        }

        usort($nav, fn ($a, $b) => $a['order'] <=> $b['order']);

        return $nav;
    }

    private function makeConverter(): MarkdownConverter
    {
        $environment = new Environment([]);
        $environment->addExtension(new CommonMarkCoreExtension);
        $environment->addExtension(new FrontMatterExtension);

        return new MarkdownConverter($environment);
    }
}
