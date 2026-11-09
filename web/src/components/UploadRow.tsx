import { t } from '../i18n'
import { errText } from '../lib/errors'
import { formatSize, formatSpeed } from '../lib/format'
import { dismiss, retry, type UploadTask } from '../lib/uploads'

export function UploadRow(props: { task: UploadTask }) {
  const u = props.task
  const pct = u.size > 0 ? Math.min(100, (u.loaded / u.size) * 100) : u.state === 'done' ? 100 : 0
  return (
    <div class={`item upload ${u.state}`} data-testid="upload-row">
      <div class="up-head">
        <span class="up-name" title={u.name}>{u.name}</span>
        <span class="fsub">
          {formatSize(u.loaded)} / {formatSize(u.size)}
          {u.state === 'uploading' && u.speed > 0 && <> · {formatSpeed(u.speed)}</>}
          {u.state === 'queued' && <> · {t('upload.queued')}</>}
          {u.state === 'done' && <> · {t('upload.done')}</>}
        </span>
      </div>
      <div class="bar-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(pct)} aria-label={u.name}>
        <div class="bar-fill" style={{ width: `${pct}%` }} />
      </div>
      {u.state === 'error' && (
        <div class="up-err">
          <span class="error" role="alert">{errText(u.error)}</span>
          <span class="sp" />
          <button class="btn" onClick={() => retry(u.id)}>{t('upload.retry')}</button>
          <button class="btn" onClick={() => dismiss(u.id)}>{t('upload.remove')}</button>
        </div>
      )}
    </div>
  )
}
