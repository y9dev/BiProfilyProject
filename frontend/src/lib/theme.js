// Light/dark theme: `data-theme` on <html> (set before first paint by index.html), stored in bip:v1:theme.
import { useCallback, useState } from 'react'
import { getItem, keys, setItem } from './storage'

function current() {
  const attr = document.documentElement.getAttribute('data-theme')
  if (attr === 'dark' || attr === 'light') return attr
  const stored = getItem(keys.theme())
  if (stored === 'dark' || stored === 'light') return stored
  return 'light' // default; the OS preference is deliberately ignored
}

export function useTheme() {
  const [theme, setThemeState] = useState(current)
  const setTheme = useCallback((next) => {
    document.documentElement.setAttribute('data-theme', next)
    setItem(keys.theme(), next)
    setThemeState(next)
  }, [])
  const toggle = useCallback(() => setTheme(theme === 'dark' ? 'light' : 'dark'), [theme, setTheme])
  return { theme, setTheme, toggle }
}
