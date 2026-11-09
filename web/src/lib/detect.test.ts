import { expect, test } from 'vitest'
import { looksLikeCode, looksLikeLog } from './detect'

const LOG14 = Array.from({ length: 14 }, (_, i) => `2026-11-09 10:2${i % 10}:0${i % 6} ERROR [order-svc] pool exhausted: timeout after 30s (conn=${40 + i})`).join('\n')
const GO = 'func retry(ctx context.Context, fn func() error) error {\n    for i := 0; i < 3; i++ {\n        if err := fn(); err == nil {\n            return nil\n        }\n        time.Sleep(time.Second << i)\n    }\n    return errors.New("retry exhausted")\n}'

test('looksLikeCode', () => {
  expect(looksLikeCode('kubectl get pods -n prod | grep Error')).toBe(true)
  expect(looksLikeCode('func a() {\n  return 1\n}')).toBe(true)
  expect(looksLikeCode('see you at 5pm')).toBe(false)
  expect(looksLikeCode('Line one of a note.\nLine two of a note.')).toBe(false)
})

test('looksLikeCode extra cases', () => {
  expect(looksLikeCode('$ ls -la')).toBe(true)
  expect(looksLikeCode('git status')).toBe(true)
  expect(looksLikeCode('docker ps')).toBe(true)
  expect(looksLikeCode(GO)).toBe(true)
  expect(looksLikeCode('const a = 1;\nconst b = 2;')).toBe(true) // keyword + semicolons
  expect(looksLikeCode('PS C:\\> dir\nPS C:\\> cd src;')).toBe(true) // prompt + semicolon
  expect(looksLikeCode('def main():\n    a = 1\n    return a')).toBe(true) // keyword + indent
  expect(looksLikeCode('https://grafana.internal/d/x')).toBe(false)
  expect(looksLikeCode('')).toBe(false)
  expect(looksLikeCode('Shopping list\nmilk and eggs')).toBe(false)
})

test('a single signal is not enough', () => {
  expect(looksLikeCode('Got this from Bob\nhttps://x.y')).toBe(false) // keyword only
  expect(looksLikeCode("let me know\nwhen you return")).toBe(false) // keywords only
  expect(looksLikeCode('a\n\tindented\n    also')).toBe(false) // indentation only
  expect(looksLikeCode('x = 1;\ny = 2')).toBe(false) // semicolon only
  expect(looksLikeCode('# Notes\nbuy milk')).toBe(false) // prompt-ish only
  expect(looksLikeCode(LOG14)).toBe(false)
})

test('looksLikeLog', () => {
  expect(looksLikeLog(LOG14)).toBe(true)
  expect(looksLikeLog('2026/11/09 10:00:01 started\n2026/11/09 10:00:02 ready')).toBe(true)
  expect(looksLikeLog('10:20:00.123 GET /a\n10:20:00.456 GET /b')).toBe(true)
  expect(looksLikeLog('[2026-11-09T10:20:00Z] boot\n[2026-11-09T10:20:01Z] up')).toBe(true)
  expect(looksLikeLog('Nov 09 10:20:01 host sshd[1]: a\nNov 09 10:20:02 host sshd[1]: b')).toBe(true)
  expect(looksLikeLog('I1109 10:20:00.1 1 main.go:1] a\nE1109 10:20:00.2 1 main.go:2] b')).toBe(true)
  expect(looksLikeLog('ERROR boom\nWARN careful\nINFO ok')).toBe(true)
  expect(looksLikeLog('[error] a\n[warn] b')).toBe(true)
  expect(looksLikeLog('E/ActivityManager: crash\nW/System: hmm')).toBe(true)
  expect(looksLikeLog('ERROR boom\n  at a.b(c.java:1)\n  at d.e(f.java:2)\nINFO done')).toBe(true) // 2 of 4 = 50%
  expect(looksLikeLog('ERROR only one line')).toBe(false)
  expect(looksLikeLog('Got this from Bob\nhttps://x.y')).toBe(false)
  expect(looksLikeLog(GO)).toBe(false)
  expect(looksLikeLog('ERROR boom\na\nb\nc')).toBe(false) // 25%
})

// ~1 MB of long whitespace runs with a few non-space chars: the old regexes
// (/^\s*…/m and /\s+$/) were quadratic here and froze every client.
const WS_BOMB = ('x' + ' '.repeat(100_000) + '\n' + '\t'.repeat(100_000) + 'y').repeat(5) + ' '.repeat(10_000) + 'z'

test('detection is linear on long whitespace runs', () => {
  for (const fn of [looksLikeCode, looksLikeLog]) {
    const t0 = performance.now()
    fn(WS_BOMB)
    expect(performance.now() - t0).toBeLessThan(200)
  }
})

test('prompt detection still works per line', () => {
  expect(looksLikeCode('notes\n  $ ls -la\n  $ cd src')).toBe(true) // prompt + indent
  expect(looksLikeCode('trailing ws  \n  \n')).toBe(false)
})
