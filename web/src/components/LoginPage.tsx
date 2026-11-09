import { useSignal } from '@preact/signals'
import { useEffect, useRef } from 'preact/hooks'
import { api, ApiError } from '../api'
import { t } from '../i18n'
import { LangSwitch } from './LangSwitch'

export function LoginPage(props: { onSuccess: () => void }) {
  const code = useSignal('')
  const err = useSignal<'' | 'login.bad_code' | 'login.locked' | 'login.error'>('')
  const busy = useSignal(false)
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => { input.current?.focus() }, [])

  async function submit(c: string) {
    busy.value = true
    err.value = ''
    try {
      await api.auth(c)
      props.onSuccess()
    } catch (e) {
      const code_ = e instanceof ApiError ? e.code : ''
      err.value = code_ === 'bad_code' ? 'login.bad_code' : code_ === 'locked' ? 'login.locked' : 'login.error'
      code.value = ''
      input.current?.focus()
    } finally {
      busy.value = false
    }
  }

  function onInput(e: Event) {
    const v = (e.target as HTMLInputElement).value.replace(/\D/g, '').slice(0, 4)
    code.value = v
    if (v.length === 4 && !busy.value) void submit(v)
  }

  return (
    <div class="login">
      <div class="login-top"><LangSwitch /></div>
      <form class="card login-card" onSubmit={(e) => { e.preventDefault(); if (code.value.length === 4 && !busy.value) void submit(code.value) }}>
        <div class="logo">Lan<span>Drop</span></div>
        <h1>{t('login.title')}</h1>
        <p class="mut">{t('login.hint')}</p>
        <input
          ref={input}
          class="code"
          type="text"
          inputMode="numeric"
          pattern="[0-9]*"
          maxLength={4}
          autoComplete="one-time-code"
          autoFocus
          aria-label={t('login.label')}
          value={code.value}
          disabled={busy.value}
          onInput={onInput}
        />
        <p class="error" role="alert">{err.value ? t(err.value) : ''}</p>
      </form>
    </div>
  )
}
