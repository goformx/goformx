import { wayfinder } from '@laravel/vite-plugin-wayfinder';
import tailwindcss from '@tailwindcss/vite';
import vue from '@vitejs/plugin-vue';
import laravel from 'laravel-vite-plugin';
import { resolve } from 'path';
import { defineConfig } from 'vite';

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
        alias: {
            '@goformx/formio': resolve(__dirname, 'node_modules/@goformx/formio'),
        },
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
