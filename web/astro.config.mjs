// @ts-check
import { defineConfig } from 'astro/config';

import vue from '@astrojs/vue';
import tailwindcss from '@tailwindcss/vite';

import node from '@astrojs/node';

// https://astro.build/config
export default defineConfig({
  integrations: [vue()],

  vite: {
    plugins: [tailwindcss()],
    optimizeDeps: {
      include: ['monaco-editor'],
    },
  },

  server: {
    port: 3000,
  },

  adapter: node({
    mode: 'standalone',
  }),
});