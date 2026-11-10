// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from '@testing-library/preact'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { setLang, t } from '../i18n'
import { draft, pending, sending } from '../lib/draft'
import { Composer } from './Composer'

const png = (n: string) => new File(['png'], n, { type: 'image/png' })

let n = 0
beforeEach(() => {
  setLang('en')
  URL.createObjectURL = vi.fn(() => `blob:${++n}`)
  URL.revokeObjectURL = vi.fn()
})
afterEach(() => {
  cleanup()
  pending.value = []
  draft.value = ''
  sending.value = false
})

function paste(el: Element, files: File[], text: string) {
  const ev = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'clipboardData', {
    value: { files, getData: (k: string) => (k === 'text/plain' ? text : '') },
  })
  act(() => { el.dispatchEvent(ev) })
  return ev
}

const textarea = () => screen.getByLabelText('Message') as HTMLTextAreaElement

test('pasting an image into the textarea adds it to the tray and inserts no text', () => {
  render(<Composer dragging={false} />)
  const ev = paste(textarea(), [png('image.png')], 'x')
  expect(ev.defaultPrevented).toBe(true)
  expect(pending.value.length).toBe(1)
  const thumbs = document.querySelectorAll('.tray img')
  expect(thumbs.length).toBe(1)
  expect(thumbs[0].getAttribute('src')).toMatch(/^blob:/)
  expect(textarea().value).toBe('')
  expect(draft.value).toBe('')
})

test('pasting plain text into the textarea is not intercepted', () => {
  render(<Composer dragging={false} />)
  const ev = paste(textarea(), [], 'hello')
  expect(ev.defaultPrevented).toBe(false)
  expect(pending.value.length).toBe(0)
})

test('✕ removes the attachment', () => {
  render(<Composer dragging={false} />)
  paste(textarea(), [new File(['abc'], 'notes.txt', { type: 'text/plain' })], '')
  expect(screen.getByText('notes.txt')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: t('composer.remove', { name: 'notes.txt' }) }))
  expect(pending.value.length).toBe(0)
  expect(screen.queryByText('notes.txt')).toBeNull()
})

test('send button enabled by attachment alone', () => {
  render(<Composer dragging={false} />)
  const send = screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement
  expect(send.disabled).toBe(true)
  paste(textarea(), [png('a.png')], '')
  expect((screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement).disabled).toBe(false)
})

test('tray hidden when empty', () => {
  render(<Composer dragging={false} />)
  expect(document.querySelector('.tray')).toBeNull()
  paste(textarea(), [png('a.png')], '')
  expect(document.querySelector('.tray')).not.toBeNull()
  fireEvent.click(screen.getByRole('button', { name: /^Remove / }))
  expect(document.querySelector('.tray')).toBeNull()
})

test('while sending: ✕ disabled, Send becomes Cancel, textarea read-only', () => {
  render(<Composer dragging={false} />)
  paste(textarea(), [png('a.png')], '')
  act(() => { sending.value = true })
  expect((screen.getByRole('button', { name: /^Remove / }) as HTMLButtonElement).disabled).toBe(true)
  expect(screen.queryByRole('button', { name: 'Send' })).toBeNull()
  expect(screen.getByRole('button', { name: t('composer.cancel') })).toBeTruthy()
  expect(textarea().readOnly).toBe(true)
})
