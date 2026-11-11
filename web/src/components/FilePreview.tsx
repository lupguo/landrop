import { useEffect, useState } from 'preact/hooks'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { formatText, TEXT_LIMIT, type PreviewKind } from '../lib/preview'

/**
 * One item in the viewer. `file` is a local (not yet sent) file; otherwise `src`
 * is the server URL. `download` is omitted for local files.
 */
export type ViewerItem = {
  name: string
  kind?: PreviewKind
  src?: string
  file?: File
  download?: string
  mime?: string
  size?: number
}

/** Object URL for a local file, revoked on unmount or when the file changes. */
function useObjectUrl(file?: File): string | undefined {
  const [url, setUrl] = useState<string>()
  useEffect(() => {
    if (!file) return
    const u = URL.createObjectURL(file)
    setUrl(u)
    return () => { URL.revokeObjectURL(u); setUrl(undefined) }
  }, [file])
  return url
}

function Failed(props: { name: string }) {
  return (
    <div class="pv-card pv-msg" role="alert">
      <div class="pv-name">{props.name}</div>
      <p>{t('viewer.failed')}</p>
    </div>
  )
}

function Loading() {
  return <div class="pv-card pv-msg">{t('viewer.loading')}</div>
}

function TextPreview(props: { item: ViewerItem }) {
  const { item } = props
  const [state, setState] = useState<{ text: string; truncated: boolean } | 'loading' | 'error'>('loading')
  useEffect(() => {
    const ctl = new AbortController()
    async function load(): Promise<string> {
      if (item.file) return item.file.slice(0, TEXT_LIMIT).text()
      const res = await fetch(item.src!, {
        credentials: 'same-origin',
        headers: { Range: `bytes=0-${TEXT_LIMIT - 1}` },
        signal: ctl.signal,
      })
      if (!res.ok) throw new Error(`http_${res.status}`)
      return res.text()
    }
    load().then(
      (raw) => {
        if (ctl.signal.aborted) return
        const size = item.file?.size ?? item.size ?? 0
        const truncated = size > TEXT_LIMIT || raw.length > TEXT_LIMIT
        const text = raw.length > TEXT_LIMIT ? raw.slice(0, TEXT_LIMIT) : raw
        setState({ text: formatText(item.name, item.mime ?? item.file?.type ?? '', text, truncated), truncated })
      },
      () => { if (!ctl.signal.aborted) setState('error') },
    )
    return () => ctl.abort()
  }, [item.file, item.src])
  if (state === 'loading') return <Loading />
  if (state === 'error') return <Failed name={item.name} />
  return (
    <div class="pv-card pv-text">
      <div class="pv-head">
        <span class="pv-name" title={item.name}>{item.name}</span>
        {state.truncated && <span class="pv-note">{t('viewer.truncated', { size: formatSize(TEXT_LIMIT) })}</span>}
      </div>
      <pre class="pv-pre" tabIndex={0}>{state.text}</pre>
    </div>
  )
}

function PdfPreview(props: { item: ViewerItem }) {
  const { item } = props
  const local = useObjectUrl(item.file)
  const [remote, setRemote] = useState<string | 'error'>()
  useEffect(() => {
    if (item.file || !item.src) return
    // Server only serves images inline (pdf comes back as an attachment), so
    // load it into memory and show it from a blob URL instead.
    const ctl = new AbortController()
    let url: string | undefined
    fetch(item.src, { credentials: 'same-origin', signal: ctl.signal })
      .then((r) => { if (!r.ok) throw new Error(`http_${r.status}`); return r.blob() })
      .then((b) => {
        if (ctl.signal.aborted) return
        url = URL.createObjectURL(new Blob([b], { type: 'application/pdf' }))
        setRemote(url)
      })
      .catch(() => { if (!ctl.signal.aborted) setRemote('error') })
    return () => { ctl.abort(); if (url) URL.revokeObjectURL(url) }
  }, [item.file, item.src])
  const url = local ?? remote
  if (url === 'error') return <Failed name={item.name} />
  if (!url) return <Loading />
  return <iframe class="pv-card pv-pdf" src={url} title={item.name} />
}

function MediaPreview(props: { item: ViewerItem; kind: 'audio' | 'video' }) {
  const { item, kind } = props
  const local = useObjectUrl(item.file)
  const [failed, setFailed] = useState(false)
  const src = item.file ? local : item.src
  if (failed) return <Failed name={item.name} />
  if (!src) return <Loading />
  if (kind === 'video') {
    return <video class="pv-video" src={src} controls autoPlay playsInline aria-label={item.name} onError={() => setFailed(true)} />
  }
  return (
    <div class="pv-card pv-audio">
      <div class="pv-name" title={item.name}>{item.name}</div>
      <audio src={src} controls autoPlay aria-label={item.name} onError={() => setFailed(true)} />
    </div>
  )
}

/** Non-image preview body for the viewer. */
export function FilePreview(props: { item: ViewerItem }) {
  const { item } = props
  switch (item.kind) {
    case 'text': return <TextPreview item={item} />
    case 'pdf': return <PdfPreview item={item} />
    case 'audio': return <MediaPreview item={item} kind="audio" />
    case 'video': return <MediaPreview item={item} kind="video" />
    default: return <Failed name={item.name} />
  }
}
