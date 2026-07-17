import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'
import { isPreview, PRODUCT_NAME, REPOSITORY_URL, SITE_DESCRIPTION, siteUrl } from './src/config/site.ts'

export default defineConfig({
  site: siteUrl.href,
  output: 'static',
  integrations: [
    starlight({
      title: PRODUCT_NAME,
      description: SITE_DESCRIPTION,
      favicon: '/favicon.svg',
      customCss: ['./src/styles/starlight.css'],
      disable404Route: true,
      editLink: {
        baseUrl: `${REPOSITORY_URL}/edit/master/site/`,
      },
      social: [
        { icon: 'github', label: 'GitHub', href: REPOSITORY_URL },
      ],
      components: {
        Footer: './src/components/docs/DocsFooter.astro',
      },
      head: [
        { tag: 'meta', attrs: { name: 'theme-color', content: '#0a0d12' } },
        { tag: 'meta', attrs: { property: 'og:site_name', content: PRODUCT_NAME } },
        ...(isPreview
          ? [{ tag: 'meta', attrs: { name: 'robots', content: 'noindex, nofollow' } }]
          : []),
      ],
      sidebar: [
        { label: 'Docs home', link: '/docs/' },
        {
          label: 'Start here',
          items: [
            { label: 'Install Switchyard', link: '/docs/start/install/' },
            { slug: 'docs/getting-started', label: 'Add your first project' },
            { label: 'Docker Compose tutorial', link: '/docs/start/compose/' },
            { label: 'Native process tutorial', link: '/docs/start/native-process/' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { slug: 'docs/tutorials/index', label: 'Tutorial index' },
            { slug: 'docs/how-to/index', label: 'How-to index' },
            { slug: 'docs/desktop-installation' },
            { slug: 'docs/troubleshooting' },
            { slug: 'docs/support-bundles' },
            { slug: 'docs/migration-v1' },
            { slug: 'docs/team-configuration' },
          ],
        },
        {
          label: 'Concepts',
          items: [
            { slug: 'docs/concepts/index', label: 'Concept index' },
            { autogenerate: { directory: 'docs/architecture' } },
          ],
        },
        {
          label: 'Reference',
          items: [
            { slug: 'docs/reference/index', label: 'Reference index' },
            { slug: 'docs/cli' },
            { slug: 'docs/manifest-reference' },
            { slug: 'docs/settings' },
            { slug: 'docs/mcp' },
            { slug: 'docs/plugin-sdk' },
            { slug: 'docs/platform-support' },
            { slug: 'docs/compatibility' },
            { slug: 'docs/privacy' },
          ],
        },
        {
          label: 'Contributing',
          items: [
            { slug: 'docs/contributing/index', label: 'Contributor index' },
            { slug: 'docs/adapter-development' },
            { slug: 'docs/release' },
            { autogenerate: { directory: 'docs/adr' } },
          ],
        },
      ],
    }),
  ],
})
