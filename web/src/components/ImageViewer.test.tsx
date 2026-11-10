// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/preact'
import { afterEach, expect, test } from 'vitest'
import { setLang } from '../i18n'
import { openOverlay } from '../lib/overlay'
import { ImageViewer } from './ImageViewer'

afterEach(cleanup)

const imgs = [1, 2, 3].map((i) => ({ src: `/i/${i}`, download: `/d/${i}`, name: `n${i}.png` }))

test('opens at start, arrows and keys navigate, wraps none', () => {
  setLang('en')
  const { getByText, getByLabelText } = render(<ImageViewer images={imgs} start={1} onClose={() => {}} />)
  getByText('2 / 3')
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  getByText('3 / 3')
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  getByText('3 / 3')
  fireEvent.keyDown(document, { key: 'ArrowLeft' })
  getByText('2 / 3')
  fireEvent.click(getByLabelText('Next'))
  getByText('3 / 3')
})

test('single image hides prev/next and count', () => {
  setLang('en')
  const { queryByLabelText, queryByText } = render(<ImageViewer images={[imgs[0]]} start={0} onClose={() => {}} />)
  expect(queryByLabelText('Previous')).toBeNull()
  expect(queryByLabelText('Next')).toBeNull()
  expect(queryByText('1 / 1')).toBeNull()
})

test('Escape and backdrop click close', () => {
  setLang('en')
  let n = 0
  const { container } = render(<ImageViewer images={imgs} start={0} onClose={() => { n++ }} />)
  fireEvent.keyDown(document, { key: 'Escape' })
  expect(n).toBe(1)
  fireEvent.mouseDown(container.querySelector('.overlay')!)
  expect(n).toBe(2)
})

test('registers as overlay', () => {
  const before = openOverlay.value
  const { unmount } = render(<ImageViewer images={imgs} start={0} onClose={() => {}} />)
  expect(openOverlay.value).toBe(before + 1)
  unmount()
  expect(openOverlay.value).toBe(before)
})

test('download link points at the current image', () => {
  setLang('en')
  const { getByLabelText } = render(<ImageViewer images={imgs} start={0} onClose={() => {}} />)
  const a = getByLabelText('Download') as HTMLAnchorElement
  expect(a.getAttribute('href')).toBe('/d/1')
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  expect(a.getAttribute('href')).toBe('/d/2')
})

test('Esc and arrows do not leak to other document listeners (stacked overlays)', () => {
  const seen: string[] = []
  const h = (e: KeyboardEvent) => { seen.push(e.key) }
  document.addEventListener('keydown', h)
  let closed = 0
  render(<ImageViewer images={imgs} start={0} onClose={() => { closed++ }} />)
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  fireEvent.keyDown(document, { key: 'Escape' })
  document.removeEventListener('keydown', h)
  expect(seen).toEqual([])
  expect(closed).toBe(1)
})

test('modifier+arrow is ignored', () => {
  const { getByText } = render(<ImageViewer images={imgs} start={0} onClose={() => {}} />)
  fireEvent.keyDown(document, { key: 'ArrowRight', altKey: true })
  getByText('1 / 3')
})

test('Tab cycles within the dialog', () => {
  const { getByLabelText } = render(<ImageViewer images={imgs} start={1} onClose={() => {}} />)
  const dl = getByLabelText('Download')
  const next = getByLabelText('Next')
  next.focus()
  fireEvent.keyDown(document, { key: 'Tab' })
  expect(document.activeElement).toBe(dl)
  fireEvent.keyDown(document, { key: 'Tab', shiftKey: true })
  expect(document.activeElement).toBe(next)
})

test('focus stays inside when Next disables at the end', () => {
  const { getByLabelText, container } = render(<ImageViewer images={imgs} start={1} onClose={() => {}} />)
  const next = getByLabelText('Next') as HTMLButtonElement
  next.focus()
  fireEvent.click(next)
  expect(next.disabled).toBe(true)
  expect(container.querySelector('.viewer-box')!.contains(document.activeElement)).toBe(true)
})

test('keyboard clamps against current image count', () => {
  const { getByText, rerender } = render(<ImageViewer images={imgs} start={0} onClose={() => {}} />)
  rerender(<ImageViewer images={imgs.slice(0, 2)} start={0} onClose={() => {}} />)
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  fireEvent.keyDown(document, { key: 'ArrowRight' })
  getByText('2 / 2')
})
