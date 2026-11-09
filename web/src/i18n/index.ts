import { signal, type Signal } from '@preact/signals'
import { en } from './en'
import { zh } from './zh'
import { ja } from './ja'
import { ko } from './ko'

export type Lang = 'en' | 'zh' | 'ja' | 'ko'
export const LANGS: Lang[] = ['en', 'zh', 'ja', 'ko']
export const dicts: Record<Lang, Record<string, string>> = { en, zh, ja, ko }

const KEY = 'landrop.lang'

function initial(): Lang {
  try {
    const v = localStorage.getItem(KEY)
    if (v && (LANGS as string[]).includes(v)) return v as Lang
  } catch { /* storage unavailable */ }
  return 'en'
}

export const lang: Signal<Lang> = signal(initial())

export function setLang(l: Lang): void {
  lang.value = l
  try { localStorage.setItem(KEY, l) } catch { /* ignore */ }
  try { document.documentElement.lang = l } catch { /* no DOM */ }
}

export function t(key: keyof typeof en, params?: Record<string, string | number>): string {
  let s = dicts[lang.value][key] || en[key] || key
  if (params) s = s.replace(/\{(\w+)\}/g, (m, p) => (p in params ? String(params[p]) : m))
  return s
}

try { document.documentElement.lang = lang.value } catch { /* no DOM */ }
