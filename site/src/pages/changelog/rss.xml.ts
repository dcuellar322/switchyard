import rss from '@astrojs/rss'
import releaseManifest from '@/data/release-manifest.json'
import { PRODUCT_NAME, SITE_DESCRIPTION } from '@/config/site'
import type { ReleaseManifest } from '@/lib/releases/types'

export async function GET(context: { site?: URL }) {
  const manifest = releaseManifest as ReleaseManifest
  return rss({
    title: `${PRODUCT_NAME} releases`,
    description: SITE_DESCRIPTION,
    site: context.site ?? new URL('https://switchyard.davidcuellar.tech'),
    items: manifest.releases.map((release) => ({
      title: release.title,
      description: release.summary ?? `Release notes for Switchyard ${release.tag}.`,
      pubDate: new Date(release.publishedAt),
      link: `/changelog/${release.tag}/`,
      categories: [release.channel],
    })),
    customData: '<language>en-us</language>',
  })
}
