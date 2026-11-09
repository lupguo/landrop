import { expect, test } from 'vitest'
import { HIGHLIGHT_LIMIT, highlight } from './highlight'

test('highlight escapes html and marks keywords', () => {
  const out = highlight('func a() {\n  return "<b>"\n}')
  expect(out).not.toContain('<b>')
  expect(out).toContain('&lt;b&gt;')
  expect(out).toContain('hljs-')
})

test('highlight caps work, tail is escaped plain text', () => {
  const line = '2026-11-09 10:21:00 ERROR [svc] pool exhausted: timeout after 30s (conn=42) {a:1};\n'
  const big = line.repeat(Math.ceil(HIGHLIGHT_LIMIT / line.length) + 2) + '<script>alert(1)</script>'
  const t0 = performance.now()
  const out = highlight(big)
  expect(performance.now() - t0).toBeLessThan(2000)
  expect(out.endsWith('&lt;script&gt;alert(1)&lt;/script&gt;')).toBe(true)
  expect(out).not.toContain('<script>')
})

test('pathological long word is rendered plain and fast', () => {
  const big = 'x'.repeat(100_000) + '<i>'
  const t0 = performance.now()
  const out = highlight(big)
  expect(performance.now() - t0).toBeLessThan(200)
  expect(out).toBe('x'.repeat(100_000) + '&lt;i&gt;')
})
