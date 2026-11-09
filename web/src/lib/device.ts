import { signal, type Signal } from '@preact/signals'

export function guessDeviceName(ua: string): string {
  let os = ''
  if (/iPhone/.test(ua)) os = 'iPhone'
  else if (/iPad/.test(ua)) os = 'iPad'
  else if (/Android/.test(ua)) os = 'Android'
  else if (/Windows/.test(ua)) os = 'Windows'
  else if (/Macintosh|Mac OS X/.test(ua)) os = 'Mac'
  else if (/CrOS/.test(ua)) os = 'ChromeOS'
  else if (/Linux|X11/.test(ua)) os = 'Linux'
  let br = ''
  if (/Edg(e|A|iOS)?\//.test(ua)) br = 'Edge'
  else if (/OPR\/|Opera/.test(ua)) br = 'Opera'
  else if (/Firefox\/|FxiOS\//.test(ua)) br = 'Firefox'
  else if (/Chrome\/|CriOS\//.test(ua)) br = 'Chrome'
  else if (/Safari\//.test(ua)) br = 'Safari'
  if (os && br) return `${os} · ${br}`
  return os || br || 'Browser'
}

const KEY = 'landrop.name'

function initial(): string {
  try {
    const v = localStorage.getItem(KEY)
    if (v) return v
  } catch { /* ignore */ }
  return guessDeviceName(typeof navigator !== 'undefined' ? navigator.userAgent : '')
}

export const deviceName: Signal<string> = signal(initial())

export function setDeviceName(n: string): void {
  deviceName.value = n
  try { localStorage.setItem(KEY, n) } catch { /* ignore */ }
}

export const NAME_MAX = 40

/** Trim and cap a user-entered device name; empty falls back to the guessed name. */
export function normalizeDeviceName(raw: string, guessed: string): string {
  const n = Array.from(raw.trim()).slice(0, NAME_MAX).join('')
  return n || guessed
}

/** Persist a user-entered name (empty resets to the guessed name and clears storage). */
export function saveDeviceName(raw: string): string {
  const guessed = guessDeviceName(typeof navigator !== 'undefined' ? navigator.userAgent : '')
  const n = normalizeDeviceName(raw, guessed)
  deviceName.value = n
  try {
    if (n === guessed) localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, n)
  } catch { /* ignore */ }
  return n
}
