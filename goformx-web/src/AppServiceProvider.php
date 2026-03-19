<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Controller\DashboardController;
use GoFormX\Controller\FormController;
use GoFormX\Middleware\SecurityHeadersMiddleware;
use GoFormX\Service\GoFormsClient;
use GoFormX\Service\GoFormsClientInterface;
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

        $this->singleton(GoFormsClientInterface::class, fn() => $this->resolve(GoFormsClient::class));
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

        // Form routes (authenticated, Inertia)
        $router->addRoute('forms.index', new Route('/forms', defaults: [
            '_controller' => fn(\Symfony\Component\HttpFoundation\Request $request) => (new FormController($this->resolve(GoFormsClientInterface::class)))->index('', 'free'),
        ]));
        $router->addRoute('forms.create', new Route('/forms', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('forms.edit', new Route('/forms/{id}/edit', defaults: ['_controller' => 'render.page']));
        $router->addRoute('forms.preview', new Route('/forms/{id}/preview', defaults: ['_controller' => 'render.page']));
        $router->addRoute('forms.submissions', new Route('/forms/{id}/submissions', defaults: ['_controller' => 'render.page']));
        $router->addRoute('forms.submission', new Route('/forms/{id}/submissions/{sid}', defaults: ['_controller' => 'render.page']));
        $router->addRoute('forms.embed', new Route('/forms/{id}/embed', defaults: ['_controller' => 'render.page']));
        $router->addRoute('forms.update', new Route('/forms/{id}', defaults: ['_controller' => 'render.page'], methods: ['PUT']));
        $router->addRoute('forms.destroy', new Route('/forms/{id}', defaults: ['_controller' => 'render.page'], methods: ['DELETE']));

        // Settings routes (authenticated, Inertia)
        $router->addRoute('settings.profile', new Route('/settings/profile', defaults: ['_controller' => 'render.page']));
        $router->addRoute('settings.profile.update', new Route('/settings/profile', defaults: ['_controller' => 'render.page'], methods: ['PATCH']));
        $router->addRoute('settings.profile.destroy', new Route('/settings/profile', defaults: ['_controller' => 'render.page'], methods: ['DELETE']));
        $router->addRoute('settings.password', new Route('/settings/password', defaults: ['_controller' => 'render.page']));
        $router->addRoute('settings.password.update', new Route('/settings/password', defaults: ['_controller' => 'render.page'], methods: ['PUT']));
        $router->addRoute('settings.appearance', new Route('/settings/appearance', defaults: ['_controller' => 'render.page']));
        $router->addRoute('settings.two-factor', new Route('/settings/two-factor', defaults: ['_controller' => 'render.page']));

        // Billing routes (authenticated, Inertia)
        $router->addRoute('billing.index', new Route('/billing', defaults: ['_controller' => 'render.page']));
        $router->addRoute('billing.checkout', new Route('/billing/checkout', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('billing.portal', new Route('/billing/portal', defaults: ['_controller' => 'render.page']));

        // Public form fill (SSR, no auth)
        $router->addRoute('forms.public', new Route('/forms/{id}', defaults: ['_controller' => 'render.page'], methods: ['GET']));

        // Stripe webhook
        $router->addRoute('stripe.webhook', new Route('/stripe/webhook', defaults: ['_controller' => 'render.page'], methods: ['POST']));
    }
}
