import { signal } from '@preact/signals'
import { useLayoutEffect, useRef } from 'preact/hooks'
import { api } from '../api'
import { t } from '../i18n'
import { errText } from '../lib/errors'
import { draft } from '../lib/draft'
import { utf8Len } from '../lib/meta'
import { enqueue } from '../lib/uploads'
import { toast } from './Toast'

export const TEXT_LIMIT = 1 << 20
const MAX_H = 180

const busy = signal(false)

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)

export async function sendText(text: string): Promise<boolean> {
  if (!text.trim()) return false
  try {
    await api.sendText(text)
    return true
  } catch (e) {
    toast(errText(e), 'err', 4000)
    return false
  }
}

export function Composer(props: { dragging: boolean; textareaRef?: { current: HTMLTextAreaElement | null } }) {
  const local = useRef<HTMLTextAreaElement>(null)
  const ta = props.textareaRef ?? local
  const fileInput = useRef<HTMLInputElement>(null)
  const text = draft.value
  const tooLarge = text.length > TEXT_LIMIT / 4 && utf8Len(text) > TEXT_LIMIT

  useLayoutEffect(() => {
    const el = ta.current
    if (!el) return
    el.style.height = 'auto'
    const h = el.scrollHeight
    el.style.height = `${Math.min(h, MAX_H)}px`
    el.style.overflowY = h > MAX_H ? 'auto' : 'hidden'
  }, [text])

  async function send() {
    if (busy.value || tooLarge || !draft.value.trim()) return
    busy.value = true
    const ok = await sendText(draft.value)
    busy.value = false
    if (ok) draft.value = ''
  }

  function sendAsFile() {
    enqueue([new File([draft.value], 'text.txt', { type: 'text/plain' })])
    draft.value = ''
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !e.isComposing) {
      e.preventDefault()
      void send()
    }
  }

  function onPick(e: Event) {
    const input = e.target as HTMLInputElement
    if (input.files?.length) enqueue(Array.from(input.files))
    input.value = ''
  }

  return (
    <div class={`composer${props.dragging ? ' drag' : ''}`}>
      <textarea
        ref={ta}
        rows={2}
        value={text}
        aria-label={t('composer.label')}
        placeholder={props.dragging ? t('composer.drop') : t('composer.placeholder')}
        onInput={(e) => { draft.value = (e.target as HTMLTextAreaElement).value }}
        onKeyDown={onKeyDown}
      />
      {tooLarge && (
        <div class="too-large" role="alert">
          <span>{t('composer.too_large')}</span>
          <button class="btn" onClick={sendAsFile}>{t('composer.send_as_file')}</button>
        </div>
      )}
      <div class="row">
        <button class="btn" onClick={() => fileInput.current?.click()}>
          <span class="ico" aria-hidden="true">📎</span>{t('composer.pick')}
        </button>
        <input ref={fileInput} type="file" multiple hidden onChange={onPick} />
        <span class="row-end">
          {text && <span class="hint">{t('composer.chars', { n: text.length })}</span>}
          <button class="btn pri" disabled={busy.value || tooLarge || !text.trim()} onClick={() => void send()}>
            {t('composer.send')} <span class="kbd" aria-hidden="true">{isMac ? '⌘↵' : 'Ctrl+↵'}</span>
          </button>
        </span>
      </div>
    </div>
  )
}
