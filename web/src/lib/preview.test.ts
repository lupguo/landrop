// @vitest-environment jsdom
import { afterEach, expect, test, vi } from 'vitest'
import { formatText, PDF_LIMIT, previewKind } from './preview'

afterEach(() => vi.restoreAllMocks())

function mediaSupports(types: string[]) {
  vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockImplementation((t: string) => (types.includes(t) ? 'maybe' : ''))
}
function pdfViewer(on: boolean) {
  Object.defineProperty(navigator, 'pdfViewerEnabled', { value: on, configurable: true })
}

test('images by mime or extension', () => {
  expect(previewKind('a.png', 'image/png', 1)).toBe('image')
  expect(previewKind('a.JPG', '', 1)).toBe('image')
  expect(previewKind('a.heic', 'image/heic', 1)).toBe(null)
})

test('text by mime or extension', () => {
  expect(previewKind('notes.txt', 'text/plain', 10)).toBe('text')
  expect(previewKind('a.json', 'application/json', 10)).toBe('text')
  expect(previewKind('data.csv', '', 10)).toBe('text')
  expect(previewKind('Makefile', '', 10)).toBe('text')
  expect(previewKind('a.bin', 'application/octet-stream', 10)).toBe(null)
})

test('media only when the browser can play it', () => {
  mediaSupports(['audio/mpeg', 'video/mp4'])
  expect(previewKind('song.mp3', '', 1)).toBe('audio')
  expect(previewKind('clip.mp4', 'video/mp4', 1)).toBe('video')
  expect(previewKind('clip.mov', 'video/quicktime', 1)).toBe(null)
  expect(previewKind('old.avi', 'video/x-msvideo', 1)).toBe(null)
})

test('pdf needs an inline viewer and a sane size', () => {
  pdfViewer(false)
  expect(previewKind('a.pdf', 'application/pdf', 1)).toBe(null)
  pdfViewer(true)
  expect(previewKind('a.pdf', 'application/pdf', 1)).toBe('pdf')
  expect(previewKind('a.pdf', '', PDF_LIMIT + 1)).toBe(null)
})

test('json is pretty-printed only when complete and valid', () => {
  expect(formatText('a.json', '', '{"a":1}', false)).toBe('{\n  "a": 1\n}')
  expect(formatText('a.json', '', '{"a":1}', true)).toBe('{"a":1}')
  expect(formatText('a.json', '', '{bad', false)).toBe('{bad')
  expect(formatText('a.txt', 'text/plain', '{"a":1}', false)).toBe('{"a":1}')
})
