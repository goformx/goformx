import { test, expect } from '../fixtures/auth';

test.describe('Form submission', () => {
    let formId: string;

    test.beforeAll(async ({ browser }) => {
        const context = await browser.newContext({
            storageState: 'e2e/.auth/user.json',
            ignoreHTTPSErrors: true,
        });
        const page = await context.newPage();
        await page.goto('/forms');
        await page.getByRole('button', { name: /New form/i }).click();
        await page.waitForURL(/\/forms\/.*\/edit/);
        const url = page.url();
        formId = url.match(/\/forms\/([^/]+)\/edit/)?.[1] ?? '';
        await context.close();
    });

    test('public form page loads', async ({ unauthenticatedPage: page }) => {
        await page.goto(`/forms/${formId}`);
        // The fill page should render (even if form has no fields yet)
        await expect(page.locator('body')).toBeVisible();
        // Should not redirect to login
        expect(page.url()).toContain(`/forms/${formId}`);
    });

    test('submissions page loads for authenticated user', async ({ authenticatedPage: page }) => {
        await page.goto(`/forms/${formId}/submissions`);
        await expect(page).toHaveTitle(/Submissions/i);
    });
});
