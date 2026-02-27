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
        // Create a form first so this test is self-contained
        await page.goto('/forms');
        await page.getByRole('button', { name: /New form/i }).click();
        await page.waitForURL(/\/forms\/.*\/edit/, { timeout: 15000 });

        const url = page.url();
        const match = url.match(/\/forms\/([^/]+)\/edit/);
        if (!match?.[1]) {
            throw new Error(
                `Failed to extract form ID from URL: ${url}. ` +
                `Expected URL to match /forms/{id}/edit pattern.`,
            );
        }
        const formId = match[1];

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
