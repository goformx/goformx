<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Controller\AuthController;
use GoFormX\Controller\BillingController;
use GoFormX\Controller\DashboardController;
use GoFormX\Controller\FormController;
use GoFormX\Controller\SettingsController;
use GoFormX\Middleware\SecurityHeadersMiddleware;
use GoFormX\Service\GoFormsClient;
use GoFormX\Service\GoFormsClientInterface;
use Symfony\Component\HttpFoundation\RedirectResponse;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\Routing\Route;
use Waaseyaa\Auth\AuthManager;
use Waaseyaa\Auth\EmailVerifier;
use Waaseyaa\Auth\Middleware\AuthenticateMiddleware;
use Waaseyaa\Auth\PasswordResetManager;
use Waaseyaa\Auth\RateLimiter;
use Waaseyaa\Auth\TwoFactorManager;
use Waaseyaa\Billing\BillingManager;
use Waaseyaa\Billing\FakeStripeClient;
use Waaseyaa\Billing\SubscriptionData;
use Waaseyaa\Billing\WebhookHandler;
use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;
use Waaseyaa\Routing\WaaseyaaRouter;

final class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(GoFormsClientInterface::class, fn() => new GoFormsClient(
            baseUrl: $this->config['goforms_api_url'] ?? 'http://localhost:8090',
            sharedSecret: $this->config['goforms_shared_secret'] ?? '',
        ));

        $this->singleton(GoFormsClient::class, fn() => $this->resolve(GoFormsClientInterface::class));
    }

    /** @return list<HttpMiddlewareInterface> */
    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        $isProduction = ($this->config['app_env'] ?? 'local') === 'production';

        return [
            new SecurityHeadersMiddleware($isProduction),
        ];
    }

    public function routes(WaaseyaaRouter $router, ?EntityTypeManager $entityTypeManager = null): void
    {
        // Set Inertia version from config
        Inertia::setVersion($this->config['inertia_version'] ?? '');

        // Share common props for all Inertia responses
        Inertia::share('goFormsPublicUrl', $this->config['goforms_public_url'] ?? '');

        $this->registerPublicRoutes($router);
        $this->registerAuthRoutes($router);
        $this->registerAppRoutes($router);
        $this->registerWebhookRoutes($router);
    }

    private function registerPublicRoutes(WaaseyaaRouter $router): void
    {
        // SSR public pages
        $router->addRoute('home', new Route('/', defaults: ['_controller' => 'render.page']));
        $router->addRoute('pricing', new Route('/pricing', defaults: ['_controller' => 'render.page']));
        $router->addRoute('privacy', new Route('/privacy', defaults: ['_controller' => 'render.page']));
        $router->addRoute('terms', new Route('/terms', defaults: ['_controller' => 'render.page']));

        // Public form fill
        $router->addRoute('forms.public', new Route('/forms/{id}', defaults: [
            '_controller' => fn(Request $request, string $id) => Inertia::render('Forms/Fill', ['formId' => $id]),
        ], methods: ['GET']));
    }

    private function registerAuthRoutes(WaaseyaaRouter $router): void
    {
        // SSR auth GET pages (render templates)
        $router->addRoute('login', new Route('/login', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('register', new Route('/register', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('forgot-password', new Route('/forgot-password', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('reset-password', new Route('/reset-password/{token}', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('verify-email', new Route('/verify-email', defaults: ['_controller' => 'render.page']));
        $router->addRoute('two-factor-challenge', new Route('/two-factor-challenge', defaults: ['_controller' => 'render.page'], methods: ['GET']));

        // Auth POST handlers
        $router->addRoute('login.post', new Route('/login', defaults: [
            '_controller' => function (Request $request) {
                $email = trim((string) $request->request->get('email', ''));
                $password = (string) $request->request->get('password', '');

                $authController = new AuthController();
                $errors = $authController->validateLogin($email, $password);
                if ($errors !== []) {
                    return new Response('', 302, ['Location' => '/login']);
                }

                // Look up user by email and authenticate
                $auth = new AuthManager();
                // For now, return redirect — full implementation needs entity storage lookup
                return new RedirectResponse('/dashboard');
            },
        ], methods: ['POST']));

        $router->addRoute('register.post', new Route('/register', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/verify-email');
            },
        ], methods: ['POST']));

        $router->addRoute('forgot-password.post', new Route('/forgot-password', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/forgot-password');
            },
        ], methods: ['POST']));

        $router->addRoute('reset-password.post', new Route('/reset-password', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/login');
            },
        ], methods: ['POST']));

        $router->addRoute('verify-email.verify', new Route('/verify-email/{id}/{hash}', defaults: [
            '_controller' => function (Request $request, string $id, string $hash) {
                return new RedirectResponse('/dashboard');
            },
        ]));

        $router->addRoute('two-factor-challenge.post', new Route('/two-factor-challenge', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/dashboard');
            },
        ], methods: ['POST']));

        $router->addRoute('logout', new Route('/logout', defaults: [
            '_controller' => function (Request $request) {
                $auth = new AuthManager();
                $auth->logout();
                return new RedirectResponse('/');
            },
        ], methods: ['POST']));
    }

    private function registerAppRoutes(WaaseyaaRouter $router): void
    {
        $getClient = fn() => $this->resolve(GoFormsClientInterface::class);
        $getUserContext = function (): array {
            $uid = $_SESSION['waaseyaa_uid'] ?? '';
            // TODO: Look up user entity for planTier. Default to 'free' for now.
            return ['userId' => $uid, 'planTier' => 'free'];
        };

        // Dashboard
        $router->addRoute('dashboard', new Route('/dashboard', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                return Inertia::render('Dashboard', []);
            },
        ]));

        // Forms
        $router->addRoute('forms.index', new Route('/forms', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new FormController($getClient());
                return $controller->index($ctx['userId'], $ctx['planTier']);
            },
        ]));

        $router->addRoute('forms.create', new Route('/forms', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                $controller = new FormController($getClient());
                $data = json_decode($request->getContent(), true) ?? [];
                $controller->store($ctx['userId'], $ctx['planTier'], $data);
                return new RedirectResponse('/forms');
            },
        ], methods: ['POST']));

        $router->addRoute('forms.edit', new Route('/forms/{id}/edit', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new FormController($getClient());
                return $controller->edit($id, $ctx['userId'], $ctx['planTier']);
            },
        ]));

        $router->addRoute('forms.preview', new Route('/forms/{id}/preview', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new FormController($getClient());
                return $controller->show($id, $ctx['userId'], $ctx['planTier']);
            },
        ]));

        $router->addRoute('forms.submissions', new Route('/forms/{id}/submissions', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new FormController($getClient());
                return $controller->submissions($id, $ctx['userId'], $ctx['planTier']);
            },
        ]));

        $router->addRoute('forms.submission', new Route('/forms/{id}/submissions/{sid}', defaults: [
            '_controller' => function (Request $request, string $id, string $sid) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new FormController($getClient());
                // Show single submission
                try {
                    $form = $getClient()->get("/api/forms/{$id}", $ctx['userId'], $ctx['planTier']);
                    $submission = $getClient()->get("/api/forms/{$id}/submissions/{$sid}", $ctx['userId'], $ctx['planTier']);
                    return Inertia::render('Forms/SubmissionShow', [
                        'form' => $form['data'] ?? [],
                        'submission' => $submission['data'] ?? [],
                    ]);
                } catch (\RuntimeException) {
                    return new RedirectResponse('/forms');
                }
            },
        ]));

        $router->addRoute('forms.embed', new Route('/forms/{id}/embed', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                Inertia::share('goFormsPublicUrl', $this->config['goforms_public_url'] ?? '');
                $controller = new FormController($getClient());
                return $controller->embed($id, $ctx['userId'], $ctx['planTier']);
            },
        ]));

        $router->addRoute('forms.update', new Route('/forms/{id}', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                $controller = new FormController($getClient());
                $data = json_decode($request->getContent(), true) ?? [];
                $controller->update($id, $ctx['userId'], $ctx['planTier'], $data);
                return new RedirectResponse("/forms/{$id}/edit");
            },
        ], methods: ['PUT']));

        $router->addRoute('forms.destroy', new Route('/forms/{id}', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                $controller = new FormController($getClient());
                $controller->destroy($id, $ctx['userId'], $ctx['planTier']);
                return new RedirectResponse('/forms');
            },
        ], methods: ['DELETE']));

        // Settings
        $router->addRoute('settings.profile', new Route('/settings/profile', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new SettingsController();
                return $controller->profile(['id' => $ctx['userId'], 'name' => '', 'email' => '']);
            },
        ]));

        $router->addRoute('settings.profile.update', new Route('/settings/profile', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/settings/profile');
            },
        ], methods: ['PATCH']));

        $router->addRoute('settings.profile.destroy', new Route('/settings/profile', defaults: [
            '_controller' => function (Request $request) {
                $auth = new AuthManager();
                $auth->logout();
                return new RedirectResponse('/');
            },
        ], methods: ['DELETE']));

        $router->addRoute('settings.password', new Route('/settings/password', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new SettingsController();
                return $controller->password();
            },
        ]));

        $router->addRoute('settings.password.update', new Route('/settings/password', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/settings/password');
            },
        ], methods: ['PUT']));

        $router->addRoute('settings.appearance', new Route('/settings/appearance', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new SettingsController();
                return $controller->appearance();
            },
        ]));

        $router->addRoute('settings.two-factor', new Route('/settings/two-factor', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                $controller = new SettingsController();
                return $controller->twoFactor(['enabled' => false]);
            },
        ]));

        // Billing
        $router->addRoute('billing.index', new Route('/billing', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId']]]);
                // For now render with basic tier info
                return Inertia::render('Billing/Index', [
                    'tier' => $ctx['planTier'],
                    'is_paid' => false,
                    'stripe_id' => null,
                    'usage' => ['forms_count' => 0],
                ]);
            },
        ]));

        $router->addRoute('billing.checkout', new Route('/billing/checkout', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/billing');
            },
        ], methods: ['POST']));

        $router->addRoute('billing.portal', new Route('/billing/portal', defaults: [
            '_controller' => function (Request $request) {
                return new RedirectResponse('/billing');
            },
        ]));
    }

    private function registerWebhookRoutes(WaaseyaaRouter $router): void
    {
        $router->addRoute('stripe.webhook', new Route('/stripe/webhook', defaults: [
            '_controller' => function (Request $request) {
                $payload = $request->getContent();
                $signature = $request->headers->get('Stripe-Signature', '');

                // TODO: Wire to WebhookHandler with real StripeClient
                return new Response('', 200);
            },
        ], methods: ['POST']));
    }
}
