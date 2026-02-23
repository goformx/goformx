import { wayfinder } from '@laravel/vite-plugin-wayfinder';
import tailwindcss from '@tailwindcss/vite';
import vue from '@vitejs/plugin-vue';
import laravel from 'laravel-vite-plugin';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Plugin } from 'vite';
import { defineConfig } from 'vite';

/**
 * Resolve @goformx/formio to its package entry file so Vite never tries to read
 * the package directory as a file (EISDIR). Runs in resolveId so we only touch
 * the package when it is actually imported.
 */
function goformxFormioResolvePlugin(): Plugin {
    return {
        name: 'resolve-goformx-formio-entry',
        enforce: 'pre',
        resolveId(id) {
            if (id !== '@goformx/formio') return null;
            const root = process.cwd();
            const pkgDir = join(root, 'node_modules', '@goformx', 'formio');
            const pkgPath = join(pkgDir, 'package.json');
            if (!existsSync(pkgPath)) return null;
            const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8')) as {
                main?: string;
                exports?: string | Record<string, string | Record<string, string>>;
            };
            let entry: string | undefined;
            if (typeof pkg.exports === 'string') {
                entry = pkg.exports;
            } else if (pkg.exports?.['.']) {
                const dot = pkg.exports['.'];
                entry =
                    typeof dot === 'string'
                        ? dot
                        : (dot as Record<string, string>).import ??
                          (dot as Record<string, string>).require ??
                          (dot as Record<string, string>).default;
            }
            entry ??= pkg.main ?? 'index.js';
            return join(pkgDir, entry);
        },
    };
}

export default defineConfig({
    server:
        process.env.VITE_SERVER_URI
            ? {
                  origin: process.env.VITE_SERVER_URI,
                  cors: {
                      origin: [
                          process.env.VITE_SERVER_URI,
                          process.env.LARAVEL_APP_URL,
                      ].filter(Boolean),
                  },
              }
            : undefined,
    resolve: {
        dedupe: ['@formio/js', '@goformx/formio'],
    },
    optimizeDeps: {
        include: ['@formio/js', '@goformx/formio'],
        esbuildOptions: {
            define: {
                global: 'globalThis',
            },
        },
    },
    plugins: [
        goformxFormioResolvePlugin(),
        laravel({
            input: ['resources/js/app.ts'],
            ssr: 'resources/js/ssr.ts',
            refresh: true,
        }),
        tailwindcss(),
        wayfinder({
            formVariants: true,
        }),
        vue({
            template: {
                transformAssetUrls: {
                    base: null,
                    includeAbsolute: false,
                },
            },
        }),
    ],
});
