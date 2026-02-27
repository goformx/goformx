import { test, expect } from '../fixtures/auth';

test.describe('Navigation', () => {
    test('sidebar Forms link navigates to forms index', async ({ authenticatedPage: page }) => {
        await page.goto('/dashboard');
        await page.getByRole('link', { name: 'Forms' }).first().click();
        await page.waitForURL('**/forms');
        await expect(page).toHaveTitle(/Forms/);
    });

    test('sidebar Dashboard link navigates to dashboard', async ({ authenticatedPage: page }) => {
        await page.goto('/forms');
        await page.getByRole('link', { name: 'Dashboard' }).first().click();
        await page.waitForURL('**/dashboard');
        await expect(page).toHaveTitle(/Dashboard/);
    });

    test('breadcrumbs render on forms page', async ({ authenticatedPage: page }) => {
        await page.goto('/forms');
        await expect(page.locator('nav[aria-label="breadcrumb"]')).toBeVisible();
    });
});
