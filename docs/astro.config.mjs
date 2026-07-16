import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://djosh34.github.io',
  base: '/openapi-validate',
  integrations: [starlight({ title: 'openapi-validate' })],
});
