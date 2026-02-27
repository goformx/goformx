import { test, expect } from '../fixtures/auth';

test.describe('Settings - Password', () => {
    test('password page loads', async ({ authenticatedPage: page }) => {
        await page.goto('/settings/password');
        await expect(page).toHaveTitle(/Password settings/i);
        await expect(page.getByLabel('Current password')).toBeVisible();
        await expect(page.getByLabel('New password')).toBeVisible();
        await expect(page.getByLabel('Confirm password')).toBeVisible();
        await expect(page.getByTestId('update-password-button')).toBeVisible();
    });
});
