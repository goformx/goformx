import { chromium, selectors, type FullConfig } from '@playwright/test';
import { TEST_USER } from './helpers/test-user';

async function globalSetup(config: FullConfig) {
    selectors.setTestIdAttribute('data-test');
    const { baseURL, ignoreHTTPSErrors } = config.projects[0].use;

    const browser = await chromium.launch();
    const context = await browser.newContext({ ignoreHTTPSErrors });
    const page = await context.newPage();

    await page.goto(`${baseURL}/login`);
    await page.getByLabel('Email address').fill(TEST_USER.email);
    await page.getByLabel('Password').fill(TEST_USER.password);
    await page.getByTestId('login-button').click();
    await page.waitForURL('**/dashboard');

    await context.storageState({ path: 'e2e/.auth/user.json' });
    await browser.close();
}

export default globalSetup;
