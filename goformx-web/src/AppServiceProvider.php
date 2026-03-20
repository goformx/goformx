<?php

declare(strict_types=1);

namespace GoFormX;

use Doctrine\DBAL\DriverManager;
use GoFormX\Controller\AuthController;
use GoFormX\Controller\BillingController;
use GoFormX\Controller\DashboardController;
use GoFormX\Controller\FormController;
use GoFormX\Controller\SettingsController;
use GoFormX\Entity\User;
use GoFormX\Mail\Transport\SmtpTransport;
use GoFormX\Middleware\SecurityHeadersMiddleware;
use GoFormX\Service\GoFormsClient;
use GoFormX\Service\GoFormsClientInterface;
use GoFormX\Service\InertiaRenderer;
use GoFormX\Service\StripeClient;
use GoFormX\Service\UserRepository;
use Symfony\Component\EventDispatcher\EventDispatcher;
use Symfony\Component\HttpFoundation\RedirectResponse;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\Routing\Route;
use Waaseyaa\Auth\AuthManager;
use Waaseyaa\Auth\EmailVerifier;
use Waaseyaa\Auth\Middleware\AuthenticateMiddleware;
use Waaseyaa\Auth\PasswordResetManager;
use Waaseyaa\Auth\TwoFactorManager;
use Waaseyaa\Billing\BillingManager;
use Waaseyaa\Billing\FakeStripeClient;
use Waaseyaa\Billing\SubscriptionData;
use Waaseyaa\Billing\WebhookHandler;
use Waaseyaa\Database\DBALDatabase;
use Waaseyaa\Entity\EntityType;
use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\EntityStorage\Connection\SingleConnectionResolver;
use Waaseyaa\EntityStorage\Driver\SqlStorageDriver;
use Waaseyaa\EntityStorage\EntityRepository;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;
use Waaseyaa\Mail\Envelope;
use Waaseyaa\Mail\Mailer;
use Waaseyaa\Mail\MailerInterface;
use Waaseyaa\Mail\Transport\TransportInterface;
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

        // MariaDB connection for user persistence (via Waaseyaa DatabaseInterface)
        $this->singleton(DBALDatabase::class . '.mariadb', function () {
            $connection = DriverManager::getConnection([
                'driver' => 'pdo_mysql',
                'host' => getenv('DB_HOST') ?: ($this->config['db_host'] ?? '127.0.0.1'),
                'dbname' => getenv('DB_DATABASE') ?: ($this->config['db_database'] ?? 'goformx'),
                'user' => getenv('DB_USERNAME') ?: ($this->config['db_username'] ?? 'goformx'),
                'password' => getenv('DB_PASSWORD') ?: ($this->config['db_password'] ?? 'goformx'),
                'charset' => 'utf8mb4',
            ]);

            return new DBALDatabase($connection);
        });

        // User entity repository (backed by MariaDB)
        $this->singleton(UserRepository::class, function () {
            $database = $this->resolve(DBALDatabase::class . '.mariadb');
            $resolver = new SingleConnectionResolver($database);
            $driver = new SqlStorageDriver($resolver);
            $entityType = new EntityType(
                id: 'users',
                label: 'User',
                class: User::class,
                keys: ['id' => 'id', 'label' => 'name'],
            );
            $eventDispatcher = new EventDispatcher();
            $entityRepository = new EntityRepository($entityType, $driver, $eventDispatcher);

            return new UserRepository($entityRepository, $database);
        });

        $this->singleton(\Waaseyaa\Billing\StripeClientInterface::class, fn() => new StripeClient(
            secretKey: $this->config['stripe_secret'] ?? '',
            webhookSecret: $this->config['stripe_webhook_secret'] ?? '',
        ));

        // Mail — override MailServiceProvider with SMTP transport for dev (Mailpit)
        $mailConfig = $this->config['mail'] ?? [];
        $this->singleton(TransportInterface::class, fn() => new SmtpTransport(
            host: $mailConfig['host'] ?? 'mailpit',
            port: (int) ($mailConfig['port'] ?? 1025),
        ));
        $this->singleton(MailerInterface::class, fn() => new Mailer(
            transport: $this->resolve(TransportInterface::class),
            defaultFrom: $mailConfig['from_address'] ?? 'noreply@goformx.com',
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

        // Form view: authenticated users go to edit, anonymous users see public fill
        $router->addRoute('forms.public', new Route('/forms/{id}', defaults: [
            '_controller' => function (Request $request, string $id): Response {
                if (!empty($_SESSION['waaseyaa_uid'])) {
                    return new RedirectResponse("/forms/{$id}/edit");
                }
                return $this->inertia($request, Inertia::render('Forms/Fill', ['formId' => $id]));
            },
        ], methods: ['GET']));
    }

    private function registerAuthRoutes(WaaseyaaRouter $router): void
    {
        // CSRF note: Inertia XHR requests send the custom X-Inertia header, which
        // cannot be set cross-origin without a CORS preflight. The CsrfMiddleware
        // exempts application/json content type, but the X-Inertia header provides
        // equivalent CSRF protection for Inertia PUT/PATCH/DELETE requests.
        // This matches Laravel's approach to Inertia CSRF handling.

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

                // Session-based rate limiting: 5 attempts per 60 seconds
                $rateLimitKey = 'login_attempts';
                $attempts = $_SESSION[$rateLimitKey] ?? 0;
                $lastAttempt = $_SESSION[$rateLimitKey . '_time'] ?? 0;

                // Reset counter after 60 seconds
                if (time() - $lastAttempt > 60) {
                    $attempts = 0;
                }

                if ($attempts >= 5) {
                    return ($this->twig('auth/login.html.twig', [
                        'error' => 'Too many login attempts. Please try again in a minute.',
                        'email' => $email,
                    ]))($request);
                }

                $authController = new AuthController();
                $errors = $authController->validateLogin($email, $password);
                if ($errors !== []) {
                    return ($this->twig('auth/login.html.twig', ['error' => $errors[0], 'email' => $email]))($request);
                }

                $users = $getUsers();
                $user = $users->findByEmail($email);
                if ($user === null || !password_verify($password, $user->password())) {
                    $_SESSION[$rateLimitKey] = $attempts + 1;
                    $_SESSION[$rateLimitKey . '_time'] = time();
                    return ($this->twig('auth/login.html.twig', ['error' => 'Invalid credentials.', 'email' => $email]))($request);
                }

                // Successful login — clear rate limit state
                unset($_SESSION[$rateLimitKey], $_SESSION[$rateLimitKey . '_time']);

                // Check if 2FA is enabled
                if ($user->hasTwoFactorEnabled()) {
                    $_SESSION['two_factor_uid'] = $user->id();
                    return new RedirectResponse('/two-factor-challenge');
                }

                $_SESSION['waaseyaa_uid'] = $user->id();
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

                // Send verification email
                $verifier = new EmailVerifier(
                    secret: $this->config['auth_secret'] ?? 'change-me',
                    urlLifetimeSeconds: $this->config['email_verification_lifetime'] ?? 3600,
                );
                // generateUrl puts all params as query string; reformat to match
                // route pattern /verify-email/{id}/{hash}?expires=...&signature=...
                $rawUrl = $verifier->generateUrl(
                    baseUrl: 'https://placeholder/verify-email',
                    userId: $uid,
                    email: $email,
                );
                $parsed = parse_url($rawUrl);
                parse_str($parsed['query'] ?? '', $params);
                $appUrl = $this->config['app_url'] ?? 'http://localhost:8080';
                $verificationUrl = $appUrl . '/verify-email/' . urlencode($params['id']) . '/' . urlencode($params['hash'])
                    . '?' . http_build_query(['expires' => $params['expires'], 'signature' => $params['signature']]);

                try {
                    $mailer = $this->resolve(MailerInterface::class);
                    $mailer->send(new Envelope(
                        to: [$email],
                        from: $this->config['mail']['from_address'] ?? 'noreply@goformx.com',
                        subject: 'Verify your email — GoFormX',
                        textBody: "Click to verify your email: {$verificationUrl}",
                        htmlBody: "<p>Click to verify your email:</p><p><a href=\"{$verificationUrl}\">Verify Email</a></p>",
                    ));
                } catch (\Throwable $e) {
                    error_log('[GoFormX] Failed to send verification email: ' . $e->getMessage());
                }

                $_SESSION['waaseyaa_uid'] = $uid;
                return new RedirectResponse('/verify-email');
            },
        ], methods: ['POST']));

        $router->addRoute('forgot-password.post', new Route('/forgot-password', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $email = trim((string) $request->request->get('email', ''));

                // Look up user and send reset email if they exist
                $users = $getUsers();
                $user = $users->findByEmail($email);
                if ($user !== null) {
                    $resetManager = new PasswordResetManager(
                        secret: $this->config['auth_secret'] ?? 'change-me',
                        tokenLifetimeSeconds: $this->config['password_reset_lifetime'] ?? 3600,
                    );
                    $token = $resetManager->createToken($user->id(), $email);
                    $resetUrl = ($this->config['app_url'] ?? 'http://localhost:8080') . '/reset-password/' . $token;

                    try {
                        $mailer = $this->resolve(MailerInterface::class);
                        $mailer->send(new Envelope(
                            to: [$email],
                            from: $this->config['mail']['from_address'] ?? 'noreply@goformx.com',
                            subject: 'Reset your password — GoFormX',
                            textBody: "Reset your password: {$resetUrl}",
                            htmlBody: "<p>Click to reset your password:</p><p><a href=\"{$resetUrl}\">Reset Password</a></p>",
                        ));
                    } catch (\Throwable $e) {
                        error_log('[GoFormX] Failed to send password reset email: ' . $e->getMessage());
                    }
                }

                // Always show success to prevent email enumeration
                return ($this->twig('auth/forgot-password.html.twig', ['status' => 'If an account exists, a reset link has been sent.']))($request);
            },
        ], methods: ['POST']));

        $router->addRoute('reset-password.post', new Route('/reset-password', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $token = (string) $request->request->get('token', '');
                $email = trim((string) $request->request->get('email', ''));
                $password = (string) $request->request->get('password', '');
                $confirmation = (string) $request->request->get('password_confirmation', '');

                $authController = new AuthController();
                $errors = $authController->validatePasswordReset($email, $password, $confirmation);
                if ($errors !== []) {
                    return ($this->twig('auth/reset-password.html.twig', ['error' => $errors[0], 'token' => $token, 'email' => $email]))($request);
                }

                $resetManager = new PasswordResetManager(
                    secret: $this->config['auth_secret'] ?? 'change-me',
                );
                $users = $getUsers();
                $user = $users->findByEmail($email);

                if ($user === null || !$resetManager->validateToken($token, $user->id(), $email)) {
                    return ($this->twig('auth/reset-password.html.twig', ['error' => 'Invalid or expired reset link.', 'token' => $token, 'email' => $email]))($request);
                }

                $users->updatePassword((string) $user->id(), $password);
                return new RedirectResponse('/login');
            },
        ], methods: ['POST']));

        $router->addRoute('verify-email.verify', new Route('/verify-email/{id}/{hash}', defaults: [
            '_controller' => function (Request $request, string $id, string $hash) use ($getUsers) {
                $expires = (int) $request->query->get('expires', '0');
                $signature = (string) $request->query->get('signature', '');

                $verifier = new EmailVerifier(
                    secret: $this->config['auth_secret'] ?? 'change-me',
                );

                $users = $getUsers();
                $user = $users->findById($id);
                if ($user === null) {
                    return new RedirectResponse('/login');
                }

                $isValid = $verifier->verify(
                    userId: $id,
                    email: $user->email(),
                    expires: $expires,
                    hash: $hash,
                    signature: $signature,
                );

                if ($isValid) {
                    $users->verifyEmail($id);
                }

                $_SESSION['waaseyaa_uid'] = $id;
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
                $secret = $user->twoFactorSecret() ?? '';

                if ($code !== '' && $twoFactor->verifyCode($secret, $code)) {
                    unset($_SESSION['two_factor_uid']);
                    $_SESSION['waaseyaa_uid'] = $uid;
                    return new RedirectResponse('/dashboard');
                }

                if ($recoveryCode !== '') {
                    $codes = $user->twoFactorRecoveryCodes();
                    if ($twoFactor->verifyRecoveryCode($recoveryCode, $codes)) {
                        $remaining = array_values(array_filter($codes, fn($c) => $c !== $recoveryCode));
                        $getUsers()->updateRecoveryCodes($uid, $remaining);
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
                unset($_SESSION['waaseyaa_uid'], $_SESSION['two_factor_uid']);
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
                'name' => $ctx['user']?->name() ?? '',
                'email' => $ctx['user']?->email() ?? '',
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
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

        $formShowHandler = function (Request $request, string $id) use ($getClient, $getUserContext) {
            $ctx = $getUserContext();
            if ($ctx['userId'] === '') {
                return new RedirectResponse('/login');
            }
            Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
            $controller = new FormController($getClient());
            try {
                $response = $controller->edit($id, $ctx['userId'], $ctx['planTier']);
                return $this->inertia($request, $response);
            } catch (\RuntimeException) {
                return new RedirectResponse('/forms');
            }
        };

        $router->addRoute('forms.edit', new Route('/forms/{id}/edit', defaults: [
            '_controller' => $formShowHandler,
        ]));

        $router->addRoute('forms.preview', new Route('/forms/{id}/preview', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
                $controller = new FormController($getClient());
                try {
                    $response = $controller->show($id, $ctx['userId'], $ctx['planTier']);
                    return $this->inertia($request, $response);
                } catch (\RuntimeException) {
                    return new RedirectResponse('/forms');
                }
            },
        ]));

        $router->addRoute('forms.submissions', new Route('/forms/{id}/submissions', defaults: [
            '_controller' => function (Request $request, string $id) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
                $controller = new FormController($getClient());
                try {
                    $response = $controller->submissions($id, $ctx['userId'], $ctx['planTier']);
                    return $this->inertia($request, $response);
                } catch (\RuntimeException) {
                    return new RedirectResponse('/forms');
                }
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
                        'form' => $form['data']['form'] ?? [],
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
                $controller = new SettingsController();
                $response = $controller->profile(['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']);
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('settings.profile.update', new Route('/settings/profile', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $uid = $_SESSION['waaseyaa_uid'] ?? '';
                if ($uid === '') {
                    return new RedirectResponse('/login');
                }

                $name = trim((string) $request->request->get('name', ''));
                $email = trim((string) $request->request->get('email', ''));

                $controller = new SettingsController();
                $errors = $controller->validateProfileUpdate($name, $email);
                if ($errors !== []) {
                    return new RedirectResponse('/settings/profile');
                }

                $getUsers()->updateProfile($uid, $name, $email);
                return new RedirectResponse('/settings/profile');
            },
        ], methods: ['PATCH']));

        $router->addRoute('settings.profile.destroy', new Route('/settings/profile', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $uid = $_SESSION['waaseyaa_uid'] ?? '';
                if ($uid === '') {
                    return new RedirectResponse('/login');
                }

                $getUsers()->delete($uid);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
                $controller = new SettingsController();
                $response = $controller->password();
                return $this->inertia($request, $response);
            },
        ]));

        $router->addRoute('settings.password.update', new Route('/settings/password', defaults: [
            '_controller' => function (Request $request) use ($getUsers) {
                $uid = $_SESSION['waaseyaa_uid'] ?? '';
                if ($uid === '') {
                    return new RedirectResponse('/login');
                }

                $currentPassword = (string) $request->request->get('current_password', '');
                $newPassword = (string) $request->request->get('password', '');
                $confirmation = (string) $request->request->get('password_confirmation', '');

                $controller = new SettingsController();
                $errors = $controller->validatePasswordChange($currentPassword, $newPassword, $confirmation);
                if ($errors !== []) {
                    return new RedirectResponse('/settings/password');
                }

                $users = $getUsers();
                $user = $users->findById($uid);
                if ($user === null || !password_verify($currentPassword, $user->password())) {
                    return new RedirectResponse('/settings/password');
                }

                $users->updatePassword($uid, $newPassword);
                return new RedirectResponse('/settings/password');
            },
        ], methods: ['PUT']));

        $router->addRoute('settings.appearance', new Route('/settings/appearance', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
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
                Inertia::share('auth', ['user' => ['id' => $ctx['userId'], 'name' => $ctx['user']?->name() ?? '', 'email' => $ctx['user']?->email() ?? '']]);
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
                $usage = ['forms' => 0, 'submissions' => 0];
                try {
                    $formsCount = $getClient()->get('/api/forms/usage/forms-count', $ctx['userId'], $ctx['planTier']);
                    $usage['forms'] = $formsCount['data']['count'] ?? 0;
                } catch (\RuntimeException) {
                }

                return $renderInertia($request, 'Billing/Index', [
                    'currentTier' => $ctx['planTier'],
                    'subscription' => null,
                    'usage' => $usage,
                    'prices' => [
                        'pro_monthly' => $this->config['billing_price_tier_map'] ? array_search('pro', $this->config['billing_price_tier_map']) ?: null : null,
                        'business_monthly' => $this->config['billing_price_tier_map'] ? array_search('business', $this->config['billing_price_tier_map']) ?: null : null,
                        'growth_monthly' => $this->config['billing_price_tier_map'] ? array_search('growth', $this->config['billing_price_tier_map']) ?: null : null,
                    ],
                ], $ctx);
            },
        ]));

        $router->addRoute('billing.checkout', new Route('/billing/checkout', defaults: [
            '_controller' => function (Request $request) use ($getClient, $getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }

                $priceId = (string) $request->request->get('price_id', '');
                if ($priceId === '') {
                    return new RedirectResponse('/billing');
                }

                $stripeCustomerId = $ctx['user']?->stripeId();
                if ($stripeCustomerId === null || $stripeCustomerId === '') {
                    // No Stripe customer yet — redirect to billing with error
                    return new RedirectResponse('/billing');
                }

                $stripeClient = $this->resolve(\Waaseyaa\Billing\StripeClientInterface::class);
                $billing = new BillingManager(
                    stripe: $stripeClient,
                    priceTierMap: $this->config['billing_price_tier_map'] ?? [],
                    successUrl: $this->config['billing_success_url'] ?? '/',
                    cancelUrl: $this->config['billing_cancel_url'] ?? '/',
                    portalReturnUrl: $this->config['billing_portal_return_url'] ?? '/',
                );

                try {
                    $session = $billing->createCheckoutSession($stripeCustomerId, $priceId);
                    return new RedirectResponse($session->url);
                } catch (\Exception $e) {
                    return new RedirectResponse('/billing');
                }
            },
        ], methods: ['POST']));

        $router->addRoute('billing.portal', new Route('/billing/portal', defaults: [
            '_controller' => function (Request $request) use ($getUserContext) {
                $ctx = $getUserContext();
                if ($ctx['userId'] === '') {
                    return new RedirectResponse('/login');
                }

                $stripeCustomerId = $ctx['user']?->stripeId();
                if ($stripeCustomerId === null || $stripeCustomerId === '') {
                    return new RedirectResponse('/billing');
                }

                $stripeClient = $this->resolve(\Waaseyaa\Billing\StripeClientInterface::class);
                $billing = new BillingManager(
                    stripe: $stripeClient,
                    priceTierMap: $this->config['billing_price_tier_map'] ?? [],
                    successUrl: $this->config['billing_success_url'] ?? '/',
                    cancelUrl: $this->config['billing_cancel_url'] ?? '/',
                    portalReturnUrl: $this->config['billing_portal_return_url'] ?? '/',
                );

                try {
                    $url = $billing->getPortalUrl($stripeCustomerId);
                    return new RedirectResponse($url);
                } catch (\Exception $e) {
                    return new RedirectResponse('/billing');
                }
            },
        ]));
    }

    private function registerWebhookRoutes(WaaseyaaRouter $router): void
    {
        $router->addRoute('stripe.webhook', new Route('/stripe/webhook', defaults: [
            '_controller' => function (Request $request) {
                $payload = $request->getContent();
                $signature = $request->headers->get('Stripe-Signature', '');

                $stripeClient = $this->resolve(\Waaseyaa\Billing\StripeClientInterface::class);
                $handler = new WebhookHandler($stripeClient);

                try {
                    $result = $handler->handle($payload, $signature);
                    // TODO: Update subscription data in MariaDB based on $result
                    return new Response('', 200);
                } catch (\Exception $e) {
                    return new Response('Webhook error', 400);
                }
            },
        ], methods: ['POST']));
    }
}
