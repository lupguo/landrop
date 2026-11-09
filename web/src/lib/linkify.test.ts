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
