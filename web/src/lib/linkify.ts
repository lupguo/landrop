export interface Segment { text: string; href?: string }

const URL_RE = /https?:\/\/[^\s<>"'`]+/g
const TRAIL = /[).,;!]+$/

/** Split text into plain and link segments. Only http(s) URLs become links. */
export function linkify(text: string): Segment[] {
  const out: Segment[] = []
  let last = 0
  const push = (s: string) => {
    if (!s) return
    const prev = out[out.length - 1]
    if (prev && !prev.href) prev.text += s
    else out.push({ text: s })
  }
  for (const m of text.matchAll(URL_RE)) {
    const url = m[0].replace(TRAIL, '')
    const start = m.index ?? 0
    if (!/^https?:\/\/[^/]/.test(url)) continue
    push(text.slice(last, start))
    out.push({ text: url, href: url })
    last = start + url.length
  }
  push(text.slice(last))
  return out
}
