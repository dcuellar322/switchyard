import type { APIRoute } from 'astro'
import { canonicalUrl, isPreview } from '@/config/site'

export const GET: APIRoute = () => {
  const body = isPreview
    ? 'User-agent: *\nDisallow: /\n'
    : `User-agent: *\nAllow: /\nSitemap: ${canonicalUrl('/sitemap-index.xml').href}\n`
  return new Response(body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } })
}
