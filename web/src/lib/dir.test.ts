import { expect, test } from 'vitest'
import { breadcrumbs, sortEntries, joinPath } from './dir'

test('breadcrumbs', () => {
  expect(breadcrumbs('')).toEqual([])
  expect(breadcrumbs('a/b c/d')).toEqual([
    { name: 'a', path: 'a' }, { name: 'b c', path: 'a/b c' }, { name: 'd', path: 'a/b c/d' },
  ])
  expect(breadcrumbs('/a//b/')).toEqual([{ name: 'a', path: 'a' }, { name: 'b', path: 'a/b' }])
})
test('joinPath', () => {
  expect(joinPath('', 'x')).toBe('x')
  expect(joinPath('a/b', 'x')).toBe('a/b/x')
})
test('sortEntries dirs first then name', () => {
  const e = [
    { name: 'b.txt', is_dir: false, size: 1, mod_time: '' },
    { name: 'z', is_dir: true, size: 0, mod_time: '' },
    { name: 'A.txt', is_dir: false, size: 1, mod_time: '' },
    { name: 'm', is_dir: true, size: 0, mod_time: '' },
  ]
  expect(sortEntries(e).map((x) => x.name)).toEqual(['m', 'z', 'A.txt', 'b.txt'])
})
