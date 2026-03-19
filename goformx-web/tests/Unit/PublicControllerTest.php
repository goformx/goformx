<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\PublicController;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(PublicController::class)]
final class PublicControllerTest extends TestCase
{
    public function testAvailablePages(): void
    {
        $controller = new PublicController();

        $this->assertSame('home.html.twig', $controller->templateFor('home'));
        $this->assertSame('pricing.html.twig', $controller->templateFor('pricing'));
        $this->assertSame('privacy.html.twig', $controller->templateFor('privacy'));
        $this->assertSame('terms.html.twig', $controller->templateFor('terms'));
    }

    public function testUnknownPageReturnsNull(): void
    {
        $controller = new PublicController();

        $this->assertNull($controller->templateFor('nonexistent'));
    }
}
