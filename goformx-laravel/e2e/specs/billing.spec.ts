import { test, expect } from '../fixtures/auth';

test.describe('Billing', () => {
    test('billing page loads', async ({ authenticatedPage: page }) => {
        await page.goto('/billing');
        await expect(page).toHaveTitle(/Billing/i);
    });
});
