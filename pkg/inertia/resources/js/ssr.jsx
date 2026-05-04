import { renderToString } from 'react-dom/server';
import { createInertiaApp } from '@inertiajs/react';
import createServer from '@inertiajs/react/server';
import { resolvePageComponent } from 'laravel-vite-plugin/inertia-helpers';
import './app.css';

createServer((page) =>
  createInertiaApp({
    resolve: (name) => resolvePageComponent(`./Pages/${name}.tsx`, import.meta.glob('./Pages/**/*.tsx')),
    page,
    render: renderToString,
    setup({ App, props }) {
      return <App {...props} />;
    },
  })
);
