import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import laravel from 'laravel-vite-plugin';

export default defineConfig({
  plugins: [
    laravel({
      input: ['resources/js/app.jsx', 'resources/js/app.css'],
      ssr: 'resources/js/ssr.jsx',
      publicDirectory: 'public',
      buildDirectory: 'bootstrap',
      refresh: true,
    }),
    react(),
  ],
  build: {
    ssr: true,
    outDir: 'bootstrap',
    rollupOptions: {
      input: 'resources/js/ssr.jsx',
      output: {
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]',
      },
    },
    chunkSizeWarningLimit: 500,
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,
        drop_debugger: true,
      },
    },
  },
});
