import { readdir, readFile, stat } from 'node:fs/promises'
import { dirname, relative, resolve } from 'node:path'
import { docsRoot, repositoryRoot, siteRoot } from './paths'

async function filesUnder(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true })
  return (await Promise.all(entries.map((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? filesUnder(path) : [path]
  }))).flat()
}

async function exists(path: string): Promise<boolean> {
  try { await stat(path); return true } catch { return false }
}

const problems: string[] = []
const markdown = (await filesUnder(docsRoot)).filter((path) => /\.mdx?$/.test(path))
for (const path of markdown) {
  const source = await readFile(path, 'utf8')
  const frontmatter = /^---\n([\s\S]*?)\n---\n/.exec(source)?.[1]
  const label = relative(repositoryRoot, path)
  if (!frontmatter) {
    problems.push(`${label}: missing frontmatter`)
    continue
  }
  for (const field of ['title', 'description', 'category', 'audience', 'lastVerified']) {
    if (!new RegExp(`^${field}:`, 'm').test(frontmatter)) problems.push(`${label}: missing ${field}`)
  }
  for (const match of source.matchAll(/\[[^\]]+\]\(([^)]+\.md(?:#[^)]+)?)\)/g)) {
    const target = match[1]?.split('#')[0]
    if (!target || target.startsWith('http')) continue
    if (!(await exists(resolve(dirname(path), target)))) problems.push(`${label}: missing link target ${target}`)
  }
}

const siteSources = (await filesUnder(resolve(siteRoot, 'src'))).filter((path) => /\.(astro|ts|tsx)$/.test(path))
for (const path of siteSources) {
  const source = await readFile(path, 'utf8')
  const label = relative(repositoryRoot, path)
  if (/from\s+['"][^'"]*web\/src\//.test(source) || source.includes('@switchyard/web')) {
    problems.push(`${label}: public site must not import product application source`)
  }
  if (/href=["']https:\/\/github\.com\/dcuellar322\/switchyard\/releases\/download\//.test(source)) {
    problems.push(`${label}: release asset URLs must come from the normalized manifest`)
  }
}

const requiredScreenshots = [
  'web/tests/visual/dashboard.spec.ts-snapshots/dashboard-alpha.png',
  'web/tests/visual/project.spec.ts-snapshots/project-alpha.png',
  'web/tests/visual/ports.spec.ts-snapshots/port-registry.png',
  'web/tests/visual/workspaces.spec.ts-snapshots/workspace-progress.png',
  'web/tests/visual/project.spec.ts-snapshots/project-terminal.png',
  'web/tests/visual/diagnostics.spec.ts-snapshots/diagnostic-automation-review.png',
]
for (const screenshot of requiredScreenshots) {
  if (!(await exists(resolve(repositoryRoot, screenshot)))) problems.push(`missing sanitized screenshot fixture ${screenshot}`)
}

if (problems.length > 0) throw new Error(`Content validation failed:\n- ${problems.join('\n- ')}`)
console.log(`Validated ${markdown.length} documentation sources and ${siteSources.length} site source files.`)
