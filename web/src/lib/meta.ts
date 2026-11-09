/** Eight fixed avatar hues; the peer id picks one deterministically. */
export const AVATAR_HUES = ['#f59e0b', '#0ea5e9', '#10b981', '#8b5cf6', '#ef4444', '#ec4899', '#14b8a6', '#6366f1'] as const

export function avatarColor(peerId: string): string {
  let h = 2166136261 // FNV-1a
  for (let i = 0; i < peerId.length; i++) {
    h ^= peerId.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return AVATAR_HUES[(h >>> 0) % AVATAR_HUES.length]
}

function firstGrapheme(s: string): string {
  const Seg = (Intl as unknown as { Segmenter?: new (l?: string, o?: { granularity: string }) => { segment(s: string): Iterable<{ segment: string }> } }).Segmenter
  if (Seg) {
    for (const g of new Seg(undefined, { granularity: 'grapheme' }).segment(s)) return g.segment
    return ''
  }
  return Array.from(s)[0] ?? ''
}

export function avatarLetter(name: string): string {
  const g = firstGrapheme((name || '').trim())
  return g ? g.toUpperCase() : '?'
}

/** Local time as HH:mm; '' for an unparsable timestamp. */
export function hhmm(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function lineCount(text: string): number {
  return text.split('\n').length
}

const enc = new TextEncoder()
export function utf8Len(text: string): number {
  return enc.encode(text).length
}

/** Uppercased file extension, at most 4 chars; '' when there is none. */
export function extLabel(name: string): string {
  const i = name.lastIndexOf('.')
  if (i <= 0 || i === name.length - 1) return ''
  return name.slice(i + 1).slice(0, 4).toUpperCase()
}

const IMAGE = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp'])
export function isImageMime(mime: string): boolean {
  return IMAGE.has(mime)
}
