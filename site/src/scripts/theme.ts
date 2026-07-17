const storageKey = 'switchyard-site-theme'
const root = document.documentElement
const stored = window.localStorage.getItem(storageKey)

if (stored === 'dark' || stored === 'light') {
  root.dataset.theme = stored
}

document.querySelectorAll<HTMLButtonElement>('[data-theme-toggle]').forEach((button) => {
  button.addEventListener('click', () => {
    const current = root.dataset.theme
    const systemLight = window.matchMedia('(prefers-color-scheme: light)').matches
    const next = current === 'dark' || (!current && !systemLight) ? 'light' : 'dark'
    root.dataset.theme = next
    window.localStorage.setItem(storageKey, next)
    button.setAttribute('aria-label', `Use ${next === 'dark' ? 'light' : 'dark'} theme`)
  })
})
