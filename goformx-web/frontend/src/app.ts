import { createApp, h, type DefineComponent } from 'vue';
import { createInertiaApp } from '@inertiajs/vue3';
import { Toaster } from 'vue-sonner';
import '../css/app.css';

const pages = import.meta.glob<DefineComponent>('./pages/**/*.vue');

createInertiaApp({
    title: (title) => `${title} - GoFormX`,
    resolve: (name) => {
        const page = pages[`./pages/${name}.vue`];
        if (!page) {
            throw new Error(`Page not found: ${name}`);
        }
        return page();
    },
    setup({ el, App, props, plugin }) {
        createApp({
            render: () =>
                h('div', [
                    h(App, props),
                    h(Toaster, { position: 'top-right', richColors: true }),
                ]),
        })
            .use(plugin)
            .mount(el);
    },
    progress: { color: '#4B5563' },
});
