<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Controller\DashboardController;
use GoFormX\Middleware\SecurityHeadersMiddleware;
use GoFormX\Service\GoFormsClient;
use Symfony\Component\Routing\Route;
use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
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

    /**
     * @return list<HttpMiddlewareInterface>
     */
    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        $isProduction = ($this->config['app_env'] ?? 'local') === 'production';

        return [
            new SecurityHeadersMiddleware($isProduction),
        ];
    }

    public function routes(WaaseyaaRouter $router, ?EntityTypeManager $entityTypeManager = null): void
    {
        // Public SSR pages
        $router->addRoute('home', new Route('/', defaults: ['_controller' => 'render.page']));
        $router->addRoute('pricing', new Route('/pricing', defaults: ['_controller' => 'render.page']));
        $router->addRoute('privacy', new Route('/privacy', defaults: ['_controller' => 'render.page']));
        $router->addRoute('terms', new Route('/terms', defaults: ['_controller' => 'render.page']));

        // Auth SSR pages (GET)
        $router->addRoute('login', new Route('/login', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('login.post', new Route('/login', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('register', new Route('/register', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('register.post', new Route('/register', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('forgot-password', new Route('/forgot-password', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('forgot-password.post', new Route('/forgot-password', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('reset-password', new Route('/reset-password/{token}', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('reset-password.post', new Route('/reset-password', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('verify-email', new Route('/verify-email', defaults: ['_controller' => 'render.page']));
        $router->addRoute('verify-email.verify', new Route('/verify-email/{id}/{hash}', defaults: ['_controller' => 'render.page']));
        $router->addRoute('two-factor-challenge', new Route('/two-factor-challenge', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('two-factor-challenge.post', new Route('/two-factor-challenge', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('logout', new Route('/logout', defaults: ['_controller' => 'render.page'], methods: ['POST']));

        // Authenticated Inertia routes
        $router->addRoute('dashboard', new Route('/dashboard', defaults: [
            '_controller' => fn(\Symfony\Component\HttpFoundation\Request $request) => (new DashboardController())->index($request),
        ]));

        // Stripe webhook
        $router->addRoute('stripe.webhook', new Route('/stripe/webhook', defaults: ['_controller' => 'render.page'], methods: ['POST']));
    }
}
