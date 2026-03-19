import { createApp, h, type DefineComponent } from 'vue';
import { createInertiaApp, resolvePageComponent } from '@inertiajs/vue3';
import { Toaster } from 'vue-sonner';
import '../css/app.css';

createInertiaApp({
    title: (title) => `${title} - GoFormX`,
    resolve: (name) =>
        resolvePageComponent(
            `./pages/${name}.vue`,
            import.meta.glob<DefineComponent>('./pages/**/*.vue'),
        ),
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
