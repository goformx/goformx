import { test, expect } from '../fixtures/auth';

test.describe('Registration', () => {
    test('register page loads', async ({ unauthenticatedPage: page }) => {
        await page.goto('/register');
        await expect(page).toHaveTitle(/Register/);
        await expect(page.getByLabel('Name')).toBeVisible();
        await expect(page.getByLabel('Email address')).toBeVisible();
        await expect(page.getByTestId('register-user-button')).toBeVisible();
    });

    test('register with unique email creates account', async ({ unauthenticatedPage: page }) => {
        const uniqueEmail = `e2e-reg-${Date.now()}@goformx.test`;

        await page.goto('/register');
        await page.getByLabel('Name').fill('E2E Register Test');
        await page.getByLabel('Email address').fill(uniqueEmail);
        await page.getByLabel('Password', { exact: true }).fill('SecurePass!2026');
        await page.getByLabel('Confirm password').fill('SecurePass!2026');
        await page.getByTestId('register-user-button').click();
        // Fortify redirects to email verification when email_verification feature is enabled
        await page.waitForURL('**/email/verify');
        await expect(page).toHaveTitle(/Email verification/i);
    });
});
