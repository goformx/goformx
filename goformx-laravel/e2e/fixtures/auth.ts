import { test as base, type Page } from '@playwright/test';

type AuthFixtures = {
    authenticatedPage: Page;
    unauthenticatedPage: Page;
};

export const test = base.extend<AuthFixtures>({
    authenticatedPage: async ({ browser }, use) => {
        const context = await browser.newContext({
            storageState: 'e2e/.auth/user.json',
            ignoreHTTPSErrors: true,
        });
        const page = await context.newPage();
        await use(page);
        await context.close();
    },

    unauthenticatedPage: async ({ browser }, use) => {
        const context = await browser.newContext({
            ignoreHTTPSErrors: true,
        });
        const page = await context.newPage();
        await use(page);
        await context.close();
    },
});

export { expect } from '@playwright/test';
