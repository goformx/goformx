import { test, expect } from '../fixtures/auth';
import { TEST_USER } from '../helpers/test-user';

test.describe('Password reset', () => {
    test('forgot password page loads', async ({ unauthenticatedPage: page }) => {
        await page.goto('/forgot-password');
        await expect(page).toHaveTitle(/Forgot password/i);
        await expect(page.getByLabel('Email address')).toBeVisible();
        await expect(page.getByTestId('email-password-reset-link-button')).toBeVisible();
    });

    test('request reset link shows status message', async ({ unauthenticatedPage: page }) => {
        await page.goto('/forgot-password');
        await page.getByLabel('Email address').fill(TEST_USER.email);
        await page.getByTestId('email-password-reset-link-button').click();
        await expect(page.getByText(/we have emailed your password reset link|if an account exists/i)).toBeVisible({ timeout: 10000 });
    });
});
