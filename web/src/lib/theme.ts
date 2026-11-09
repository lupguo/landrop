import { signal, type Signal } from '@preact/signals'

export type Theme = 'system' | 'light' | 'dark'
const KEY = 'landrop.theme'

function initial(): Theme {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark') return v
  } catch { /* storage unavailable */ }
  return 'system'
}

export const theme: Signal<Theme> = signal(initial())

/** Reflect the current theme on <html>: "system" removes the attribute. */
export function applyTheme(): void {
  try {
    const el = document.documentElement
    if (theme.value === 'system') el.removeAttribute('data-theme')
    else el.setAttribute('data-theme', theme.value)
  } catch { /* no DOM */ }
}

export function setTheme(th: Theme): void {
  theme.value = th
  try {
    if (th === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, th)
  } catch { /* ignore */ }
  applyTheme()
}
