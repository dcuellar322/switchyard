import { DEFAULT_SITE_URL, PRODUCT_NAME } from '../src/config/site'

const origin = process.env.SMOKE_SITE_URL ?? DEFAULT_SITE_URL
const routes = ['/', '/download/', '/docs/', '/docs/start/', '/changelog/', '/community/', '/security/']
const problems: string[] = []
for (const route of routes) {
  const url = new URL(route, origin)
  const response = await fetch(url, { redirect: 'follow' })
  const body = await response.text()
  if (!response.ok) problems.push(`${url.href}: HTTP ${response.status}`)
  if (response.url !== url.href) problems.push(`${url.href}: resolved to ${response.url}`)
  if (!body.includes(PRODUCT_NAME)) problems.push(`${url.href}: differentiated product identity missing`)
  if (!body.includes(`<link rel="canonical" href="${url.href}"`)) problems.push(`${url.href}: canonical link missing or incorrect`)
}
if (problems.length > 0) throw new Error(`Production smoke failed:\n- ${problems.join('\n- ')}`)
console.log(`Production smoke passed for ${routes.length} canonical routes at ${origin}.`)
