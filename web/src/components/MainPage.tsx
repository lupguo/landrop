import { useSignal } from '@preact/signals'
import { useEffect, useRef } from 'preact/hooks'
import { utf8Len } from '../lib/meta'
import { enqueue } from '../lib/uploads'
import { Composer, draft, sendText, TEXT_LIMIT } from './Composer'
import { ConnBanner } from './ConnBanner'
import { Feed } from './Feed'
import { TopBar } from './TopBar'

function isEditable(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable
}

function hasFiles(e: DragEvent): boolean {
  return !!e.dataTransfer && Array.from(e.dataTransfer.types).includes('Files')
}

const noop = () => {}

export function MainPage() {
  const dragging = useSignal(false)
  const ta = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    let depth = 0

    function onPaste(e: ClipboardEvent) {
      if (isEditable(e.target) || isEditable(document.activeElement)) return
      const cd = e.clipboardData
      if (!cd) return
      const files = Array.from(cd.files)
      if (files.length) {
        e.preventDefault()
        enqueue(files)
        return
      }
      const text = cd.getData('text/plain')
      if (!text.trim()) return
      e.preventDefault()
      if (utf8Len(text) > TEXT_LIMIT) {
        // Too large for a message: show it in the composer, which offers "send as file".
        draft.value = text
        ta.current?.focus()
        return
      }
      void sendText(text)
    }
    function onDragEnter(e: DragEvent) {
      if (!hasFiles(e)) return
      depth++
      dragging.value = true
    }
    function onDragLeave(e: DragEvent) {
      if (!hasFiles(e)) return
      depth = Math.max(0, depth - 1)
      if (depth === 0) dragging.value = false
    }
    function onDragOver(e: DragEvent) {
      // Always prevent default so a stray drop never navigates away from the app.
      e.preventDefault()
      if (e.dataTransfer) e.dataTransfer.dropEffect = hasFiles(e) ? 'copy' : 'none'
    }
    function onDrop(e: DragEvent) {
      e.preventDefault()
      depth = 0
      dragging.value = false
      const files = e.dataTransfer ? Array.from(e.dataTransfer.files) : []
      if (files.length) enqueue(files)
    }

    document.addEventListener('paste', onPaste)
    document.addEventListener('dragenter', onDragEnter)
    document.addEventListener('dragleave', onDragLeave)
    document.addEventListener('dragover', onDragOver)
    document.addEventListener('drop', onDrop)
    return () => {
      document.removeEventListener('paste', onPaste)
      document.removeEventListener('dragenter', onDragEnter)
      document.removeEventListener('dragleave', onDragLeave)
      document.removeEventListener('dragover', onDragOver)
      document.removeEventListener('drop', onDrop)
    }
  }, [])

  return (
    <div class="main">
      <TopBar onPeersClick={noop} onSettingsClick={noop} />
      <ConnBanner />
      <div class="main-col">
        <Composer dragging={dragging.value} textareaRef={ta} />
        <Feed onShare={noop} />
      </div>
    </div>
  )
}
