import { getCollection, type CollectionEntry } from 'astro:content'
import type { APIRoute, GetStaticPaths } from 'astro'

interface Props {
  entry: CollectionEntry<'docs'>
}

export const getStaticPaths: GetStaticPaths = async () => {
  const docs = await getCollection('docs')
  return docs.map((entry) => ({
    params: { slug: entry.id.replace(/^docs\/?/, '') || 'index' },
    props: { entry },
  }))
}

export const GET: APIRoute<Props> = ({ props }) => {
  const { entry } = props
  const header = `# ${entry.data.title}\n\n> ${entry.data.description}\n\n`
  return new Response(`${header}${entry.body ?? ''}`, {
    headers: {
      'Content-Type': 'text/markdown; charset=utf-8',
      'Content-Disposition': `inline; filename="${entry.id.split('/').at(-1) || 'index'}.md"`,
    },
  })
}
