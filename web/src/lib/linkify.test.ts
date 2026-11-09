import { expect, test } from 'vitest'
import { linkify } from './linkify'

test('linkify', () => {
  expect(linkify('open https://a.b/c?x=1). ok')).toEqual([{ text: 'open ' }, { text: 'https://a.b/c?x=1', href: 'https://a.b/c?x=1' }, { text: '). ok' }])
  expect(linkify('javascript:alert(1)')).toEqual([{ text: 'javascript:alert(1)' }])
})

test('linkify extra cases', () => {
  expect(linkify('')).toEqual([])
  expect(linkify('http://x.y')).toEqual([{ text: 'http://x.y', href: 'http://x.y' }])
  expect(linkify('a http://x.y! b https://z.w/q;')).toEqual([
    { text: 'a ' }, { text: 'http://x.y', href: 'http://x.y' }, { text: '! b ' },
    { text: 'https://z.w/q', href: 'https://z.w/q' }, { text: ';' },
  ])
  expect(linkify('ftp://x.y data:text/html,hi')).toEqual([{ text: 'ftp://x.y data:text/html,hi' }])
  expect(linkify('see (https://a.b/c_(d))')).toEqual([{ text: 'see (' }, { text: 'https://a.b/c_(d', href: 'https://a.b/c_(d' }, { text: '))' }])
})

test('prose with a URL is linkified', () => {
  expect(linkify('Got this from Bob\nhttps://x.y')).toEqual([{ text: 'Got this from Bob\n' }, { text: 'https://x.y', href: 'https://x.y' }])
})

test('linkify is linear on pathological input', () => {
  const ws = ('x' + ' '.repeat(100_000) + '\n' + '\t'.repeat(100_000) + 'y').repeat(5)
  const parens = 'https://a.b/' + ')'.repeat(1_000_000) + 'x'
  for (const s of [ws, parens]) {
    const t0 = performance.now()
    linkify(s)
    expect(performance.now() - t0).toBeLessThan(200)
  }
  expect(linkify('see https://a.b/c)).,')).toEqual([{ text: 'see ' }, { text: 'https://a.b/c', href: 'https://a.b/c' }, { text: ')).,' }])
})
