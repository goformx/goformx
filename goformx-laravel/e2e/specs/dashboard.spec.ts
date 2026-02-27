import { test, expect } from '../fixtures/auth';

test.describe('Dashboard', () => {
    test('loads for authenticated user', async ({ authenticatedPage: page }) => {
        await page.goto('/dashboard');
        await expect(page).toHaveTitle(/Dashboard/);
        await expect(page.getByRole('link', { name: 'Forms', exact: true })).toBeVisible();
    });

    test('redirects unauthenticated user to login', async ({ unauthenticatedPage: page }) => {
        await page.goto('/dashboard');
        await page.waitForURL('**/login');
        await expect(page).toHaveTitle(/Log in/);
    });
});
