import { signal } from '@preact/signals'
import { t } from '../i18n'

interface ToastMsg { id: number; text: string; kind: 'ok' | 'err' }
const list = signal<ToastMsg[]>([])
let seq = 0

export function toast(text: string, kind: 'ok' | 'err' = 'ok', ms = 2500): void {
  const id = ++seq
  list.value = [...list.value, { id, text, kind }]
  setTimeout(() => { list.value = list.value.filter((x) => x.id !== id) }, ms)
}

export function Toasts() {
  return (
    <div class="toasts" role="status" aria-live="polite">
      {list.value.map((m) => (
        <div key={m.id} class={`toast ${m.kind}`}>
          {m.text}
          <button class="toast-x" aria-label={t('toast.close')} onClick={() => { list.value = list.value.filter((x) => x.id !== m.id) }}>×</button>
        </div>
      ))}
    </div>
  )
}
