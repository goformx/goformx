import type { Page } from '@playwright/test';

/**
 * Delete all forms for the current authenticated user.
 * Navigates to /forms, extracts form IDs from edit links, and deletes each via API.
 * Pass baseURL when calling from global setup (where pages lack a configured baseURL).
 */
export async function deleteAllForms(page: Page, baseURL?: string): Promise<void> {
    const formsUrl = baseURL ? `${baseURL}/forms` : '/forms';
    await page.goto(formsUrl);
    await page.waitForLoadState('networkidle');

    const formIds = await page.locator('a[href*="/forms/"][href*="/edit"]').evaluateAll((links) =>
        links
            .map((a) => a.getAttribute('href')?.match(/\/forms\/([^/]+)\/edit/)?.[1])
            .filter((id): id is string => !!id),
    );

    for (const id of formIds) {
        await page.evaluate(async (formId) => {
            await fetch(`/forms/${formId}`, {
                method: 'DELETE',
                headers: {
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-XSRF-TOKEN': decodeURIComponent(
                        document.cookie.match(/XSRF-TOKEN=([^;]+)/)?.[1] ?? '',
                    ),
                    Accept: 'text/html, application/xhtml+xml',
                },
            });
        }, id);
    }
}
