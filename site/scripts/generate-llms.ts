import { readdir, readFile, writeFile } from 'node:fs/promises'
import { relative, resolve, sep } from 'node:path'
import { canonicalUrl, PRODUCT_NAME, REPOSITORY_URL } from '../src/config/site'
import { docsRoot, publicRoot } from './paths'

interface Document { path: string; title: string; description: string; body: string }

async function markdownFiles(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true })
  const nested = await Promise.all(entries.map((entry) => {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) return markdownFiles(path)
    return /\.mdx?$/.test(entry.name) ? [path] : []
  }))
  return nested.flat()
}

function parseDocument(path: string, source: string): Document {
  const match = /^---\n([\s\S]*?)\n---\n([\s\S]*)$/.exec(source)
  if (!match?.[1]) throw new Error(`${path} is missing frontmatter`)
  const title = /^title:\s*["']?(.+?)["']?$/m.exec(match[1])?.[1]
  const description = /^description:\s*(.+)$/m.exec(match[1])?.[1]
  if (!title || !description) throw new Error(`${path} requires title and description`)
  const sourcePath = relative(docsRoot, path).split(sep).join('/')
  const routePath = sourcePath.replace(/\.mdx?$/, '').replace(/(^|\/)(README|index)$/, '$1').replace(/\/$/, '')
  return { path: routePath, title, description, body: match[2] ?? '' }
}

export async function generateLLMS(): Promise<void> {
  const docs = await Promise.all((await markdownFiles(docsRoot)).sort().map(async (path) => parseDocument(path, await readFile(path, 'utf8'))))
  const index = [
    `# ${PRODUCT_NAME}`,
    '',
    '> Open-source, local-first command center for projects, runtimes, logs, ports, Git, terminals, and coding agents.',
    '',
    `Canonical site: ${canonicalUrl('/').href}`,
    `Source: ${REPOSITORY_URL}`,
    '',
    '## Documentation',
    '',
    ...docs.filter((doc) => !doc.path.startsWith('progress/')).map((doc) => `- [${doc.title}](${canonicalUrl(doc.path ? `/docs/${doc.path}/` : '/docs/').href}): ${doc.description}`),
    '',
    '## Agent integration',
    '',
    `- [Codex](${canonicalUrl('/integrations/codex/').href})`,
    `- [Claude Code](${canonicalUrl('/integrations/claude-code/').href})`,
    `- [MCP reference](${canonicalUrl('/docs/mcp/').href})`,
    '',
  ].join('\n')
  const full = docs.map((doc) => `# ${doc.title}\n\nSource: ${canonicalUrl(doc.path ? `/docs/${doc.path}/` : '/docs/').href}\n\n${doc.description}\n\n${doc.body.trim()}`).join('\n\n---\n\n')
  await writeFile(resolve(publicRoot, 'llms.txt'), index, 'utf8')
  await writeFile(resolve(publicRoot, 'llms-full.txt'), `# ${PRODUCT_NAME}: complete documentation\n\n${full}\n`, 'utf8')
}

if (import.meta.url === `file://${process.argv[1]}`) await generateLLMS()
