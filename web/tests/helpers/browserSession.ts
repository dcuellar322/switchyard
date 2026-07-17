import { execFileSync } from 'node:child_process'

export function browserBootstrapPath(pathname = '/'): string {
  const daemonAddress = process.env.SWITCHYARD_E2E_DAEMON_ADDRESS ?? '127.0.0.1:29616'
  const output = execFileSync(
    '../.cache/switchyard-e2e',
    ['--address', daemonAddress, '--data-dir', '../.switchyard-data/e2e', 'ui'],
    { encoding: 'utf8' },
  ).trim()
  const token = new URL(output).searchParams.get('bootstrap')
  if (!token) throw new Error('switchyard ui did not return a browser bootstrap token')
  return `${pathname}?bootstrap=${encodeURIComponent(token)}`
}
