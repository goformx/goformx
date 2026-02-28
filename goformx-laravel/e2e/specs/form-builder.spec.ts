import { test, expect } from '../fixtures/auth';
import { deleteAllForms } from '../helpers/forms';

test.describe('Form builder', () => {
    let formEditUrl: string;

    test.beforeAll(async ({ browser }) => {
        const context = await browser.newContext({
            storageState: 'e2e/.auth/user.json',
            ignoreHTTPSErrors: true,
        });
        const page = await context.newPage();
        await page.goto('/forms');
        await page.getByRole('button', { name: /New form/i }).click();
        await page.waitForURL(/\/forms\/.*\/edit/);
        formEditUrl = new URL(page.url()).pathname;
        await context.close();
    });

    test.afterAll(async ({ browser }) => {
        const context = await browser.newContext({
            storageState: 'e2e/.auth/user.json',
            ignoreHTTPSErrors: true,
        });
        const page = await context.newPage();
        await deleteAllForms(page);
        await context.close();
    });

    test('builder page loads with toolbar', async ({ authenticatedPage: page }) => {
        await page.goto(formEditUrl);
        // Title is dynamic: the form title or "Edit Form"
        await expect(page).toHaveTitle(/Untitled Form|Edit Form/i);
        await expect(page.getByRole('button', { name: /Save/i })).toBeVisible();
        await expect(page.getByRole('button', { name: /Preview/i })).toBeVisible();
    });

    test('builder canvas renders', async ({ authenticatedPage: page }) => {
        await page.goto(formEditUrl);
        await expect(page.locator('#form-schema-builder')).toBeVisible({ timeout: 15000 });
    });

    test('preview toggle switches view', async ({ authenticatedPage: page }) => {
        await page.goto(formEditUrl);
        await page.locator('#form-schema-builder').waitFor({ timeout: 15000 });
        const previewButton = page.getByRole('button', { name: /Preview/i });
        await previewButton.click();
        await expect(page.getByRole('button', { name: /Builder/i })).toBeVisible();
    });

    test('title can be edited', async ({ authenticatedPage: page }) => {
        await page.goto(formEditUrl);
        const titleInput = page.locator('#title');
        await titleInput.fill('E2E Test Form');
        await expect(titleInput).toHaveValue('E2E Test Form');
    });

    test('drag Text Field from sidebar onto empty canvas', async ({ authenticatedPage: page }) => {
        await page.goto(formEditUrl);
        const builder = page.locator('#form-schema-builder');
        await builder.waitFor({ timeout: 15000 });

        // Verify initial state: empty canvas has only the Submit button component
        const componentsBefore = await builder.locator('.builder-component').count();
        const dropTarget = builder.locator('.drag-and-drop-alert');
        await expect(dropTarget).toBeVisible();

        // Sidebar drag source: the Text Field component button
        const dragSource = builder.locator('span.formcomponent.drag-copy[data-type="textfield"]');
        await expect(dragSource).toBeVisible({ timeout: 10000 });

        // Form.io uses Dragula which requires pointer events with buttons:1 held.
        // Playwright's high-level dragAndDrop handles this correctly.
        await dragSource.dragTo(builder.locator('.formio-builder-form'));

        // After drop: a new Text Field component should appear in the canvas
        // The empty placeholder should disappear and component count should increase
        await expect(async () => {
            const componentsAfter = await builder.locator('.builder-component').count();
            expect(componentsAfter).toBeGreaterThan(componentsBefore);
        }).toPass({ timeout: 5000 });
        // The drop zone placeholder should no longer be visible
        await expect(dropTarget).not.toBeVisible();
    });
});
