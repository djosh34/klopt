import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://djosh34.github.io',
  base: '/klopt',
  integrations: [
    starlight({
      title: 'klopt',
      sidebar: [
        { label: 'Getting Started', link: '/' },
        { label: 'Philosophy', link: '/philosophy/' },
        { label: 'Architecture', link: '/architecture/' },
        { label: 'Query Decoding', link: '/query-decoding/' },
        { label: 'Patterns', link: '/patterns/' },
        { label: 'OpenAPI Compatibility', link: '/openapi-compatibility/' },
        { label: 'Roadmap', link: '/roadmap/' },
      ],
    }),
  ],
});
