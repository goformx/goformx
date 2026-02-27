import { test, expect } from '../fixtures/auth';
import { TEST_USER } from '../helpers/test-user';

test.describe('Settings - Profile', () => {
    test('profile page loads with user info', async ({ authenticatedPage: page }) => {
        await page.goto('/settings/profile');
        await expect(page).toHaveTitle(/Profile settings/i);
        await expect(page.getByLabel('Name')).toHaveValue(TEST_USER.name);
        await expect(page.getByLabel('Email address')).toHaveValue(TEST_USER.email);
        await expect(page.getByTestId('update-profile-button')).toBeVisible();
    });

    test('profile name can be updated', async ({ authenticatedPage: page }) => {
        await page.goto('/settings/profile');
        const nameInput = page.getByLabel('Name');
        await nameInput.fill('E2E Updated Name');
        await page.getByTestId('update-profile-button').click();
        // Wait for the "Saved." confirmation text
        await expect(page.getByText('Saved.')).toBeVisible({ timeout: 10000 });
        // Reload and verify persisted
        await page.reload();
        await expect(page.getByLabel('Name')).toHaveValue('E2E Updated Name');
        // Restore original name
        await page.getByLabel('Name').fill(TEST_USER.name);
        await page.getByTestId('update-profile-button').click();
        await expect(page.getByText('Saved.')).toBeVisible({ timeout: 10000 });
    });
});
