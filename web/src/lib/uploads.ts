import { signal, type Signal } from '@preact/signals'
import { ApiError, upload } from '../api'

export type UploadTask = {
  id: string
  name: string
  size: number
  loaded: number
  speed: number
  state: 'queued' | 'uploading' | 'done' | 'error'
  error?: string
}

export const CONCURRENCY = 3
const SPEED_WINDOW_MS = 1000
const REMOVE_DONE_MS = 1200

export const uploads: Signal<UploadTask[]> = signal([])

type Sample = [number, number] // [time ms, loaded bytes]
const files = new Map<string, File>()
const samples = new Map<string, Sample[]>()
let seq = 0

/** Average bytes/s across the samples inside the last window (falls back to the previous sample). */
export function speedOver(s: Sample[], now: number, windowMs = SPEED_WINDOW_MS): number {
  if (s.length < 2) return 0
  let i = s.length - 1
  while (i > 0 && s[i - 1][0] >= now - windowMs) i--
  if (i === s.length - 1) i--
  const [t0, b0] = s[i]
  const [t1, b1] = s[s.length - 1]
  return t1 > t0 ? Math.round(((b1 - b0) * 1000) / (t1 - t0)) : 0
}

function patch(id: string, p: Partial<UploadTask>): void {
  uploads.value = uploads.value.map((x) => (x.id === id ? { ...x, ...p } : x))
}

function start(task: UploadTask): void {
  const file = files.get(task.id)
  if (!file) return
  samples.set(task.id, [[Date.now(), 0]])
  patch(task.id, { state: 'uploading', loaded: 0, speed: 0, error: undefined })
  const { promise } = upload(file, (loaded) => {
    const now = Date.now()
    const s = samples.get(task.id) ?? []
    s.push([now, loaded])
    while (s.length > 2 && s[1][0] < now - SPEED_WINDOW_MS) s.shift()
    samples.set(task.id, s)
    patch(task.id, { loaded, speed: speedOver(s, now) })
  })
  promise.then(
    () => {
      files.delete(task.id)
      samples.delete(task.id)
      patch(task.id, { state: 'done', loaded: task.size, speed: 0 })
      setTimeout(() => { uploads.value = uploads.value.filter((x) => x.id !== task.id) }, REMOVE_DONE_MS)
    },
    (e: unknown) => {
      samples.delete(task.id)
      patch(task.id, { state: 'error', speed: 0, error: e instanceof ApiError ? e.code || `http_${e.status}` : 'network' })
    },
  ).finally(pump)
}

function pump(): void {
  let active = uploads.value.filter((x) => x.state === 'uploading').length
  for (const task of uploads.value) {
    if (active >= CONCURRENCY) break
    if (task.state !== 'queued') continue
    start(task)
    active++
  }
}

/** Queue files for upload; at most CONCURRENCY run at once. */
export function enqueue(list: File[]): void {
  if (!list.length) return
  const added: UploadTask[] = list.map((f) => {
    const id = `u${++seq}`
    files.set(id, f)
    return { id, name: f.name, size: f.size, loaded: 0, speed: 0, state: 'queued' }
  })
  uploads.value = [...uploads.value, ...added]
  pump()
}

export function retry(id: string): void {
  const task = uploads.value.find((x) => x.id === id)
  if (!task || task.state !== 'error') return
  patch(id, { state: 'queued', loaded: 0, speed: 0, error: undefined })
  pump()
}

export function dismiss(id: string): void {
  files.delete(id)
  samples.delete(id)
  uploads.value = uploads.value.filter((x) => x.id !== id)
}
