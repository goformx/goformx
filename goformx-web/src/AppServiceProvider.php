<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Service\GoFormsClient;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;
use Waaseyaa\Routing\WaaseyaaRouter;

final class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(GoFormsClient::class, fn() => new GoFormsClient(
            baseUrl: $this->config['goforms_api_url'] ?? 'http://localhost:8090',
            sharedSecret: $this->config['goforms_shared_secret'] ?? '',
        ));
    }

    public function routes(WaaseyaaRouter $router, ?\Waaseyaa\Entity\EntityTypeManager $entityTypeManager = null): void
    {
        $router->addRoute('home', new \Symfony\Component\Routing\Route(
            path: '/',
            defaults: ['_controller' => 'render.page'],
        ));
    }
}
