import { defineConfig } from '@playwright/test';

export default defineConfig({
    testDir: './e2e/specs',
    globalSetup: './e2e/global-setup.ts',
    fullyParallel: false,
    workers: 1,
    retries: process.env.CI ? 1 : 0,
    reporter: 'html',
    use: {
        baseURL: 'https://goformx-laravel.ddev.site',
        ignoreHTTPSErrors: true,
        testIdAttribute: 'data-test',
        screenshot: 'only-on-failure',
        trace: 'retain-on-failure',
    },
    projects: [
        {
            name: 'chromium',
            use: { browserName: 'chromium' },
        },
    ],
});
