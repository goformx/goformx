import { chromium, type FullConfig } from '@playwright/test';
import { TEST_USER } from './helpers/test-user';

async function globalSetup(config: FullConfig) {
    const project = config.projects[0];
    if (!project?.use?.baseURL) {
        throw new Error(
            'Global setup failed: baseURL is not configured in playwright.config.ts. ' +
            'Ensure projects[0].use.baseURL is set.',
        );
    }
    const { baseURL, ignoreHTTPSErrors } = project.use;

    const browser = await chromium.launch();
    try {
        const context = await browser.newContext({ ignoreHTTPSErrors });
        const page = await context.newPage();

        try {
            await page.goto(`${baseURL}/login`);
        } catch (err) {
            throw new Error(
                `Global setup failed: Could not reach ${baseURL}/login. ` +
                `Is DDEV running? (ddev start). Original error: ${err}`,
            );
        }

        await page.getByLabel('Email address').fill(TEST_USER.email);
        await page.getByLabel('Password').fill(TEST_USER.password);
        await page.locator('[data-test="login-button"]').click();

        try {
            await page.waitForURL('**/dashboard');
        } catch (err) {
            throw new Error(
                `Global setup failed: Login did not redirect to /dashboard. ` +
                `Has the E2E test user been seeded? (php artisan db:seed --class=E2eTestSeeder). ` +
                `Original error: ${err}`,
            );
        }

        await context.storageState({ path: 'e2e/.auth/user.json' });
    } finally {
        await browser.close();
    }
}

export default globalSetup;
