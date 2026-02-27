import { test, expect } from '@playwright/test';

test.describe('Public pages', () => {
    test('homepage loads with hero content', async ({ page }) => {
        await page.goto('/');
        await expect(page).toHaveTitle(/GoFormX/);
        await expect(page.getByText('Your Forms,')).toBeVisible();
        await expect(page.getByText('Our Backend')).toBeVisible();
    });

    test('pricing page loads', async ({ page }) => {
        await page.goto('/pricing');
        await expect(page).toHaveTitle(/Pricing/);
    });

    test('privacy page loads', async ({ page }) => {
        await page.goto('/privacy');
        await expect(page).toHaveTitle(/Privacy/);
    });

    test('terms page loads', async ({ page }) => {
        await page.goto('/terms');
        await expect(page).toHaveTitle(/Terms/);
    });

    test('demo page loads', async ({ page }) => {
        await page.goto('/demo');
        await expect(page).toHaveTitle(/Demo/);
    });

    test('sitemap.xml returns XML', async ({ page }) => {
        const response = await page.goto('/sitemap.xml');
        expect(response?.status()).toBe(200);
        const body = await page.content();
        expect(body).toContain('urlset');
    });

    test('robots.txt returns content', async ({ page }) => {
        await page.goto('/robots.txt');
        const body = await page.content();
        expect(body).toContain('User-agent');
        expect(body).toContain('Sitemap');
    });
});
