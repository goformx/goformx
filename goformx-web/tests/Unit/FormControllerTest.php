<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\FormController;
use GoFormX\Service\GoFormsClientInterface;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

#[CoversClass(FormController::class)]
final class FormControllerTest extends TestCase
{
    protected function setUp(): void
    {
        Inertia::reset();
        Inertia::setVersion('v1');
    }

    public function testIndexReturnsInertiaResponseWithForms(): void
    {
        $client = $this->createMock(GoFormsClientInterface::class);
        $client->method('get')->willReturn(['data' => ['forms' => [['id' => '1', 'title' => 'Contact']]]]);

        $controller = new FormController($client);
        $response = $controller->index('user-1', 'free');

        $this->assertInstanceOf(InertiaResponse::class, $response);
        $page = $response->toPageObject();
        $this->assertSame('Forms/Index', $page['component']);
        $this->assertCount(1, $page['props']['forms']);
    }

    public function testIndexReturnsEmptyOnApiError(): void
    {
        $client = $this->createMock(GoFormsClientInterface::class);
        $client->method('get')->willThrowException(new \RuntimeException('API error'));

        $controller = new FormController($client);
        $response = $controller->index('user-1', 'free');

        $page = $response->toPageObject();
        $this->assertSame([], $page['props']['forms']);
    }

    public function testEditReturnsFormData(): void
    {
        $client = $this->createMock(GoFormsClientInterface::class);
        $client->method('get')->willReturn(['data' => ['form' => ['id' => 'form-1', 'title' => 'Survey']]]);

        $controller = new FormController($client);
        $response = $controller->edit('form-1', 'user-1', 'pro');

        $page = $response->toPageObject();
        $this->assertSame('Forms/Edit', $page['component']);
        $this->assertSame('form-1', $page['props']['form']['id']);
    }

    public function testSubmissionsReturnsFormAndSubmissions(): void
    {
        $client = $this->createMock(GoFormsClientInterface::class);
        $client->method('get')->willReturnOnConsecutiveCalls(
            ['data' => ['form' => ['id' => 'form-1']]],
            ['data' => ['submissions' => [['id' => 'sub-1'], ['id' => 'sub-2']]]],
        );

        $controller = new FormController($client);
        $response = $controller->submissions('form-1', 'user-1', 'pro');

        $page = $response->toPageObject();
        $this->assertSame('Forms/Submissions', $page['component']);
        $this->assertSame('form-1', $page['props']['form']['id']);
        $this->assertCount(2, $page['props']['submissions']);
    }

    public function testEmbedReturnsFormData(): void
    {
        $client = $this->createMock(GoFormsClientInterface::class);
        $client->method('get')->willReturn(['data' => ['form' => ['id' => 'form-1']]]);

        $controller = new FormController($client);
        $response = $controller->embed('form-1', 'user-1', 'free');

        $page = $response->toPageObject();
        $this->assertSame('Forms/Embed', $page['component']);
    }
}
