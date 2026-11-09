import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import go from 'highlight.js/lib/languages/go'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import python from 'highlight.js/lib/languages/python'
import sql from 'highlight.js/lib/languages/sql'

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('go', go)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('python', python)
hljs.registerLanguage('sql', sql)

/** Only this many characters are highlighted; the rest is escaped plain text. */
export const HIGHLIGHT_LIMIT = 200 * 1024

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!)
}

const SAMPLE = 8 * 1024
// hljs's js/ts/bash grammars are quadratic on very long identifier runs
// (e.g. pasted base64): 20K chars ~ 1s. Such text renders plain instead.
const LONG_WORD = /[\w$]{256,}/

/**
 * Returns HTML. The language is auto-detected on a small sample, then only
 * the first HIGHLIGHT_LIMIT chars are highlighted with that one grammar;
 * the rest is escaped plain text. hljs escapes its own input.
 */
export function highlight(code: string): string {
  const head = code.slice(0, HIGHLIGHT_LIMIT)
  const tail = code.slice(HIGHLIGHT_LIMIT)
  if (LONG_WORD.test(head)) return escapeHtml(code)
  let html: string
  try {
    const lang = hljs.highlightAuto(head.slice(0, SAMPLE)).language
    html = lang ? hljs.highlight(head, { language: lang, ignoreIllegals: true }).value : escapeHtml(head)
  } catch {
    html = escapeHtml(head)
  }
  return html + escapeHtml(tail)
}
