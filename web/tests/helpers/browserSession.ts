import { execFileSync } from 'node:child_process'

export function browserBootstrapPath(): string {
  const output = execFileSync(
    '../.cache/switchyard-e2e',
    ['--data-dir', '../.switchyard-data/e2e', 'ui'],
    { encoding: 'utf8' },
  ).trim()
  const token = new URL(output).searchParams.get('bootstrap')
  if (!token) throw new Error('switchyard ui did not return a browser bootstrap token')
  return `/?bootstrap=${encodeURIComponent(token)}`
}
