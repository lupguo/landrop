export interface Segment { text: string; href?: string }

const URL_RE = /https?:\/\/[^\s<>"'`]+/g
const TRAIL = ').,;!'

// Strips trailing punctuation by hand: /[).,;!]+$/ is quadratic on long runs.
function stripTrail(url: string): string {
  let end = url.length
  while (end > 0 && TRAIL.includes(url[end - 1])) end--
  return url.slice(0, end)
}

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
    const url = stripTrail(m[0])
    const start = m.index ?? 0
    if (!/^https?:\/\/[^/]/.test(url)) continue
    push(text.slice(last, start))
    out.push({ text: url, href: url })
    last = start + url.length
  }
  push(text.slice(last))
  return out
}
