import { test, expect } from '../fixtures/auth';

test.describe('Forms CRUD', () => {
    test('forms index loads', async ({ authenticatedPage: page }) => {
        await page.goto('/forms');
        await expect(page).toHaveTitle(/Forms/);
        await expect(page.getByRole('heading', { name: 'Forms' })).toBeVisible();
    });

    test('create form navigates to builder', async ({ authenticatedPage: page }) => {
        await page.goto('/forms');
        await page.getByRole('button', { name: /New form/i }).click();
        await page.waitForURL(/\/forms\/.*\/edit/, { timeout: 15000 });
        await expect(page).toHaveTitle(/Untitled Form/i);
    });

    test('delete form removes it from list', async ({ authenticatedPage: page }) => {
        // The previous test created a form — navigate to it to get its ID
        await page.goto('/forms');
        const formLink = page.locator('a[href*="/forms/"][href*="/edit"]').first();
        await expect(formLink).toBeVisible({ timeout: 10000 });
        const href = await formLink.getAttribute('href');
        const formId = href?.match(/\/forms\/([^/]+)\/edit/)?.[1];
        expect(formId).toBeTruthy();

        // Delete via API
        await page.evaluate(async (id) => {
            await fetch(`/forms/${id}`, {
                method: 'DELETE',
                headers: {
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-XSRF-TOKEN': decodeURIComponent(
                        document.cookie.match(/XSRF-TOKEN=([^;]+)/)?.[1] ?? '',
                    ),
                    Accept: 'text/html, application/xhtml+xml',
                },
            });
        }, formId);

        // Reload and verify form is gone
        await page.goto('/forms');
        const remainingLinks = await page.locator(`a[href*="/forms/${formId}/edit"]`).count();
        expect(remainingLinks).toBe(0);
    });
});
