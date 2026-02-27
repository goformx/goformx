import { test, expect } from '../fixtures/auth';

test.describe('Settings - Appearance', () => {
    test('appearance page loads', async ({ authenticatedPage: page }) => {
        await page.goto('/settings/appearance');
        await expect(page).toHaveTitle(/Appearance settings/i);
    });

    test('theme toggle is present', async ({ authenticatedPage: page }) => {
        await page.goto('/settings/appearance');
        await expect(page.getByText('Light')).toBeVisible();
        await expect(page.getByText('Dark')).toBeVisible();
        await expect(page.getByText('System')).toBeVisible();
    });
});
