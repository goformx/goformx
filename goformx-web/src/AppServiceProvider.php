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
use GoFormX\Service\InertiaRenderer;
use GoFormX\Service\UserRepository;
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

        $this->singleton(InertiaRenderer::class, fn() => new InertiaRenderer(
            isDev: ($this->config['app_env'] ?? 'local') !== 'production',
            viteDevUrl: 'http://localhost:5173',
        ));

        $this->singleton(UserRepository::class, fn() => new UserRepository(
            host: $_ENV['DB_HOST'] ?? $this->config['db_host'] ?? '127.0.0.1',
            database: $_ENV['DB_DATABASE'] ?? $this->config['db_database'] ?? 'goformx',
            username: $_ENV['DB_USERNAME'] ?? $this->config['db_username'] ?? 'goformx',
            password: $_ENV['DB_PASSWORD'] ?? $this->config['db_password'] ?? 'goformx',
        ));
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

    /**
     * Render an Inertia response — returns full HTML for initial loads,
     * or InertiaResponse for XHR (handled by ControllerDispatcher as JSON).
     */
    private function inertia(Request $request, InertiaResponse $response): Response|InertiaResponse
    {
        if ($request->headers->get('X-Inertia') === 'true') {
            return $response;
        }

        $renderer = $this->resolve(InertiaRenderer::class);
        return $renderer->render($response, $request->getRequestUri());
    }

    private function twig(string $template, array $vars = []): \Closure
    {
        return function (Request $request) use ($template, $vars): Response {
            $loader = new \Twig\Loader\FilesystemLoader($this->projectRoot . '/templates');
            $twig = new \Twig\Environment($loader);
            $vars['csrf_token'] = $_SESSION['_csrf_token'] ?? bin2hex(random_bytes(16));
            $_SESSION['_csrf_token'] ??= $vars['csrf_token'];
            $html = $twig->render($template, $vars);
            return new Response($html, 200, ['Content-Type' => 'text/html; charset=UTF-8']);
        };
    }

    private function registerPublicRoutes(WaaseyaaRouter $router): void
    {
        $router->addRoute('home', new Route('/', defaults: ['_controller' => $this->twig('home.html.twig')]));
        $router->addRoute('pricing', new Route('/pricing', defaults: ['_controller' => $this->twig('pricing.html.twig')]));
        $router->addRoute('privacy', new Route('/privacy', defaults: ['_controller' => $this->twig('privacy.html.twig')]));
        $router->addRoute('terms', new Route('/terms', defaults: ['_controller' => $this->twig('terms.html.twig')]));

        // Public form fill (Inertia)
        $router->addRoute('forms.public', new Route('/forms/{id}', defaults: [
            '_controller' => fn(Request $request, string $id) => $this->inertia($request, Inertia::render('Forms/Fill', ['formId' => $id])),
        ], methods: ['GET']));
    }

    private function registerAuthRoutes(WaaseyaaRouter $router): void
    {
        // SSR auth GET pages
        $router->addRoute('login', new Route('/login', defaults: ['_controller' => $this->twig('auth/login.html.twig')], methods: ['GET']));
        $router->addRoute('register', new Route('/register', defaults: ['_controller' => $this->twig('auth/register.html.twig')], methods: ['GET']));
        $router->addRoute('forgot-password', new Route('/forgot-password', defaults: ['_controller' => $this->twig('auth/forgot-password.html.twig')], methods: ['GET']));
        $router->addRoute('reset-password', new Route('/reset-password/{token}', defaults: [
            '_controller' => fn(Request $request, string $token) => ($this->twig('auth/reset-password.html.twig', ['token' => $token]))($request),
        ], methods: ['GET']));
        $router->addRoute('verify-email', new Route('/verify-email', defaults: ['_controller' => $this->twig('auth/verify-email.html.twig')]));
        $router->addRoute('two-factor-challenge', new Route('/two-factor-challenge', defaults: ['_controller' => $this->twig('auth/two-factor-challenge.html.twig')], methods: ['GET']));

        // Auth POST handlers
        $getUsers = fn(): UserRepository => $this->resolve(UserRepository::class);

        $router->addRoute('login.post', new Route('/login', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $email = trim((string) $request->request->get('email', ''));
                $password = (string) $request->request->get('password', '');

                $authController = new AuthController();
                $errors = $authController->validateLogin($email, $password);
                if ($errors !== []) {
                    return ($this->twig('auth/login.html.twig', ['error' => $errors[0], 'email' => $email]))($request);
                }

                $users = $getUsers();
                $user = $users->findByEmail($email);
                if ($user === null || !password_verify($password, $user['pass'] ?? '')) {
                    return ($this->twig('auth/login.html.twig', ['error' => 'Invalid credentials.', 'email' => $email]))($request);
                }

                if (($user['status'] ?? 0) != 1) {
                    return ($this->twig('auth/login.html.twig', ['error' => 'Account is disabled.', 'email' => $email]))($request);
                }

                // Check if 2FA is enabled
                if (!empty($user['two_factor_secret']) && !empty($user['two_factor_confirmed_at'])) {
                    $_SESSION['two_factor_uid'] = $user['uid'];
                    return new RedirectResponse('/two-factor-challenge');
                }

                $_SESSION['waaseyaa_uid'] = $user['uid'];
                return new RedirectResponse('/dashboard');
            },
        ], methods: ['POST']));

        $router->addRoute('register.post', new Route('/register', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $name = trim((string) $request->request->get('name', ''));
                $email = trim((string) $request->request->get('email', ''));
                $password = (string) $request->request->get('password', '');
                $confirmation = (string) $request->request->get('password_confirmation', '');

                $authController = new AuthController();
                $errors = $authController->validateRegistration($name, $email, $password, $confirmation);

                $users = $getUsers();
                if ($errors === [] && $users->findByEmail($email) !== null) {
                    $errors[] = 'An account with this email already exists.';
                }

                if ($errors !== []) {
                    return ($this->twig('auth/register.html.twig', ['errors' => $errors, 'name' => $name, 'email' => $email]))($request);
                }

                $uid = $users->create(['name' => $name, 'email' => $email, 'password' => $password]);
                $_SESSION['waaseyaa_uid'] = $uid;
                return new RedirectResponse('/verify-email');
            },
        ], methods: ['POST']));

        $router->addRoute('forgot-password.post', new Route('/forgot-password', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $email = trim((string) $request->request->get('email', ''));
                // Always show success to prevent email enumeration
                return ($this->twig('auth/forgot-password.html.twig', ['status' => 'If an account exists, a reset link has been sent.']))($request);
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
            '_controller' => function (Request $request) use ($getUsers) {
                $code = trim((string) $request->request->get('code', ''));
                $recoveryCode = trim((string) $request->request->get('recovery_code', ''));
                $uid = $_SESSION['two_factor_uid'] ?? '';

                if ($uid === '') {
                    return new RedirectResponse('/login');
                }

                $user = $getUsers()->findById($uid);
                if ($user === null) {
                    return new RedirectResponse('/login');
                }

                $twoFactor = new TwoFactorManager();
                $secret = $user['two_factor_secret'] ?? '';

                if ($code !== '' && $twoFactor->verifyCode($secret, $code)) {
                    unset($_SESSION['two_factor_uid']);
                    $_SESSION['waaseyaa_uid'] = $uid;
                    return new RedirectResponse('/dashboard');
                }

                if ($recoveryCode !== '') {
                    $codes = json_decode($user['two_factor_recovery_codes'] ?? '[]', true) ?: [];
                    if ($twoFactor->verifyRecoveryCode($recoveryCode, $codes)) {
                        $remaining = array_values(array_filter($codes, fn($c) => $c !== $recoveryCode));
                        $stmt = $getUsers()->findById($uid); // just verify; update codes would need a method
                        unset($_SESSION['two_factor_uid']);
                        $_SESSION['waaseyaa_uid'] = $uid;
                        return new RedirectResponse('/dashboard');
                    }
                }

                return ($this->twig('auth/two-factor-challenge.html.twig', ['error' => 'Invalid code.']))($request);
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
        $getUsers = fn(): UserRepository => $this->resolve(UserRepository::class);
        $getUserContext = function () use ($getUsers): array {
            $uid = $_SESSION['waaseyaa_uid'] ?? '';
            if ($uid === '') {
                return ['userId' => '', 'planTier' => 'free', 'user' => null];
            }
            $user = $getUsers()->findById($uid);
            $planTier = $user !== null ? $getUsers()->getPlanTier($uid) : 'free';
            return ['userId' => $uid, 'planTier' => $planTier, 'user' => $user];
        };

        // Helper to share auth and render Inertia
        $renderInertia = function (Request $request, string $component, array $props, array $ctx): Response|InertiaResponse {
            Inertia::share('auth', ['user' => [
                'id' => $ctx['userId'],
                'name' => $ctx['user']['name'] ?? '',
                'email' => $ctx['user']['mail'] ?? '',
            ]]);
            return $this->inertia($request, Inertia::render($component, $props));
        };

        // Dashboard
        $router->addRoute('dashboard', new Route('/dashboard', defaults: [
            '_controller' => function (Request $request) use ($getUserContext, $renderInertia) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                return $renderInertia($request, 'Dashboard', [], $ctx);
            },
        ]));

        // Forms
        $router->addRoute('forms.index', new Route('/forms', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new FormController($getClient());
                $response = $controller->index($ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new FormController($getClient());
                $response = $controller->edit($id, $ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('forms.preview', new Route('/forms/{id}/preview', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new FormController($getClient());
                $response = $controller->show($id, $ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('forms.submissions', new Route('/forms/{id}/submissions', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new FormController($getClient());
                $response = $controller->submissions($id, $ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('forms.submission', new Route('/forms/{id}/submissions/{sid}', defaults: [
            '_controller' => function (Request $request, string $id, string $sid) use ($getClient, $getUserContext, $renderInertia) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                // Show single submission
                try {
                    $form = $getClient()->get("/api/forms/{$id}", $ctx['userId'], $ctx['planTier']);
                    $submission = $getClient()->get("/api/forms/{$id}/submissions/{$sid}", $ctx['userId'], $ctx['planTier']);
                    return $renderInertia($request, 'Forms/SubmissionShow', [
                        'form' => $form['data'] ?? [],
                        'submission' => $submission['data'] ?? [],
                    ], $ctx);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                Inertia::share('goFormsPublicUrl', $this->config['goforms_public_url'] ?? '');
                $controller = new FormController($getClient());
                $response = $controller->embed($id, $ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new SettingsController();
                $response = $controller->profile(['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']);
                return $this->inertia($request, $response);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new SettingsController();
                $response = $controller->password();
                return $this->inertia($request, $response);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new SettingsController();
                $response = $controller->appearance();
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('settings.two-factor', new Route('/settings/two-factor', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']['name'] ?? '', 'email' => $ctx['user']['mail'] ?? '']]);
                $controller = new SettingsController();
                $response = $controller->twoFactor(['enabled' => false]);
                return $this->inertia($request, $response);
            },
        ]));

        // Billing
        $router->addRoute('billing.index', new Route('/billing', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext, $renderInertia) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                // For now render with basic tier info
                return $renderInertia($request, 'Billing/Index', [
                    'tier' => $ctx['planTier'],
                    'is_paid' => false,
                    'stripe_id' => null,
                    'usage' => ['forms_count' => 0],
                ], $ctx);
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
