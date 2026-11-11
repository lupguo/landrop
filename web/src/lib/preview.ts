import { isImageMime } from './meta'

/** How a file can be previewed in the viewer; null means download only. */
export type PreviewKind = 'image' | 'text' | 'audio' | 'video' | 'pdf'

/** Text previews read at most this many bytes. */
export const TEXT_LIMIT = 1 << 20
/** PDFs are loaded whole into memory; larger ones are download only. */
export const PDF_LIMIT = 100 << 20

const TEXT_EXT = new Set([
  'txt', 'text', 'log', 'md', 'markdown', 'json', 'jsonl', 'ndjson', 'csv', 'tsv', 'xml', 'yaml', 'yml',
  'toml', 'ini', 'conf', 'cfg', 'env', 'properties', 'sql', 'diff', 'patch', 'srt', 'vtt',
  'go', 'mod', 'sum', 'js', 'mjs', 'cjs', 'ts', 'tsx', 'jsx', 'py', 'rb', 'rs', 'java', 'kt', 'swift',
  'c', 'h', 'cc', 'cpp', 'hpp', 'cs', 'php', 'lua', 'sh', 'bash', 'zsh', 'ps1', 'bat',
  'css', 'scss', 'less', 'html', 'htm', 'vue', 'svelte', 'proto', 'gradle', 'makefile', 'dockerfile',
])
const TEXT_MIME = new Set([
  'application/json', 'application/x-ndjson', 'application/xml', 'application/yaml', 'application/x-yaml',
  'application/toml', 'application/javascript', 'application/x-sh', 'application/sql',
])
const AUDIO_EXT: Record<string, string> = {
  mp3: 'audio/mpeg', m4a: 'audio/mp4', aac: 'audio/aac', wav: 'audio/wav', ogg: 'audio/ogg', oga: 'audio/ogg',
  opus: 'audio/ogg; codecs=opus', flac: 'audio/flac', weba: 'audio/webm',
}
const VIDEO_EXT: Record<string, string> = {
  mp4: 'video/mp4', m4v: 'video/mp4', webm: 'video/webm', mov: 'video/quicktime', ogv: 'video/ogg',
}
const IMAGE_EXT: Record<string, string> = {
  png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', webp: 'image/webp',
}

function ext(name: string): string {
  const i = name.lastIndexOf('.')
  return i < 0 ? name.toLowerCase() : name.slice(i + 1).toLowerCase()
}

function baseMime(mime: string): string {
  return (mime || '').split(';')[0].trim().toLowerCase()
}

/** Whether this browser says it can play the media type ('' means no). */
export function canPlay(kind: 'audio' | 'video', type: string): boolean {
  try {
    return document.createElement(kind).canPlayType(type) !== ''
  } catch {
    return false
  }
}

/** Whether this browser renders PDFs inline (false on Android Chrome). */
export function pdfInline(): boolean {
  return (navigator as Navigator & { pdfViewerEnabled?: boolean }).pdfViewerEnabled === true
}

/**
 * Pick the preview for a file from its MIME type, falling back to the extension
 * (CLI uploads may carry no type). Media and PDF depend on what this browser supports.
 */
export function previewKind(name: string, mime: string, size: number): PreviewKind | null {
  const m = baseMime(mime)
  const e = ext(name)
  if (isImageMime(m) || (!m.startsWith('image/') && IMAGE_EXT[e])) return 'image'
  if (m === 'application/pdf' || e === 'pdf') return size <= PDF_LIMIT && pdfInline() ? 'pdf' : null
  if (m.startsWith('audio/') || AUDIO_EXT[e]) return canPlay('audio', m.startsWith('audio/') ? m : AUDIO_EXT[e]) ? 'audio' : null
  if (m.startsWith('video/') || VIDEO_EXT[e]) return canPlay('video', m.startsWith('video/') ? m : VIDEO_EXT[e]) ? 'video' : null
  if (m.startsWith('text/') || TEXT_MIME.has(m) || m.endsWith('+json') || m.endsWith('+xml') || TEXT_EXT.has(e)) return 'text'
  return null
}

/** Pretty-print complete JSON documents; anything else (or a truncated read) is returned as is. */
export function formatText(name: string, mime: string, text: string, truncated: boolean): string {
  const m = baseMime(mime)
  if (truncated || !(ext(name) === 'json' || m === 'application/json' || m.endsWith('+json'))) return text
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}
