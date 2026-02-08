import { defineConfig } from 'astro/config';
import vue from '@astrojs/vue';

export default defineConfig({
  integrations: [vue()],
  server: {
    port: 4321,
  },
  vite: {
    server: {
      proxy: {
        '/v1': {
          target: 'http://localhost:7533',
          changeOrigin: true,
        },
      },
    },
  },
});
