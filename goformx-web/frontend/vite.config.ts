import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';
import { resolve } from 'path';

export default defineConfig({
    plugins: [
        tailwindcss(),
        vue(),
    ],
    resolve: {
        alias: {
            '@': resolve(__dirname, 'src'),
        },
        dedupe: ['@formio/js', '@goformx/formio'],
    },
    base: '/build/',
    build: {
        outDir: '../public/build',
        manifest: true,
        rollupOptions: {
            input: resolve(__dirname, 'src/app.ts'),
        },
    },
    server: {
        port: 5173,
        strictPort: true,
    },
});
