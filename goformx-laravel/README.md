# GoFormX Laravel

Web UI and identity layer for GoFormX. Laravel handles auth (Fortify), dashboard, and the form builder; form data and public embeds are served by the [Go forms API](https://github.com/goformx/goforms) (goforms).

## Architecture

- **This app (Laravel):** Auth, sessions, Inertia + Vue pages (dashboard, form builder). Database: users, sessions only.
- **Go service (goforms):** Form CRUD, schema, submissions, event bus. Laravel calls it with signed headers (`X-User-Id`, `X-Timestamp`, `X-Signature`). Public embed/submit go directly to Go.

See [docs/plans/2026-02-18-goformx-laravel-go-split-design.md](docs/plans/2026-02-18-goformx-laravel-go-split-design.md) for the full design.

## Tech Stack

- PHP 8.4, Laravel 12
- Inertia.js v2 + Vue 3
- Laravel Fortify (auth), Wayfinder (routes)
- Form.io form builder (Vue)
- Tailwind CSS, shadcn-vue

## Requirements

- PHP 8.2+
- Composer, Node.js, npm
- Go forms API running (e.g. goforms at `http://localhost:8090`)

## Quick Start

1. **Clone and install**

   ```bash
   git clone <repo-url> goformx-laravel
   cd goformx-laravel
   composer install
   cp .env.example .env
   php artisan key:generate
   npm install
   ```

2. **Configure Go API**

   In `.env` set (use the same secret in the Go app):

   ```bash
   GOFORMS_API_URL=http://localhost:8090
   GOFORMS_SHARED_SECRET=your-shared-secret
   ```

3. **Database and assets**

   ```bash
   php artisan migrate
   npm run build
   ```

4. **Run**

   ```bash
   php artisan serve
   ```

   With Go running on 8090, open the app (e.g. `http://localhost:8000`), register/login, then use **Forms** to create and edit forms.

## Development

- `composer run dev` — PHP server, queue, Pail, Vite (single command)
- `php artisan test` — run tests
- `vendor/bin/pint` — code style

## License

MIT.
