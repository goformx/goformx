import { test, expect } from '../fixtures/auth';
import { TEST_USER } from '../helpers/test-user';

test.describe('Login', () => {
    test('login page loads', async ({ unauthenticatedPage: page }) => {
        await page.goto('/login');
        await expect(page).toHaveTitle(/Log in/);
        await expect(page.getByLabel('Email address')).toBeVisible();
        await expect(page.getByLabel('Password')).toBeVisible();
        await expect(page.getByTestId('login-button')).toBeVisible();
    });

    test('login with valid credentials redirects to dashboard', async ({ unauthenticatedPage: page }) => {
        await page.goto('/login');
        await page.getByLabel('Email address').fill(TEST_USER.email);
        await page.getByLabel('Password').fill(TEST_USER.password);
        await page.getByTestId('login-button').click();
        await page.waitForURL('**/dashboard');
        await expect(page).toHaveTitle(/Dashboard/);
    });

    test('login with bad credentials shows error', async ({ unauthenticatedPage: page }) => {
        await page.goto('/login');
        await page.getByLabel('Email address').fill('wrong@example.com');
        await page.getByLabel('Password').fill('wrongpassword');
        await page.getByTestId('login-button').click();
        await expect(page.getByText(/These credentials do not match/)).toBeVisible();
    });

    test('logout returns to home page', async ({ unauthenticatedPage: page }) => {
        // Log in fresh so we don't invalidate the global storageState session
        await page.goto('/login');
        await page.getByLabel('Email address').fill(TEST_USER.email);
        await page.getByLabel('Password').fill(TEST_USER.password);
        await page.getByTestId('login-button').click();
        await page.waitForURL('**/dashboard');

        await page.getByTestId('sidebar-menu-button').click();
        await page.getByTestId('logout-button').click();
        await page.waitForURL('/');
        await expect(page).toHaveTitle(/GoFormX/);
    });
});
