<?php

declare(strict_types=1);

namespace GoFormX\Controller;

final class PublicController
{
    /** @var array<string, string> */
    private const array PAGE_TEMPLATES = [
        'home' => 'home.html.twig',
        'pricing' => 'pricing.html.twig',
        'privacy' => 'privacy.html.twig',
        'terms' => 'terms.html.twig',
    ];

    public function templateFor(string $page): ?string
    {
        return self::PAGE_TEMPLATES[$page] ?? null;
    }
}
