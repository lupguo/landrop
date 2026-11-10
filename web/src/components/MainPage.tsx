import { useSignal } from '@preact/signals'
import { useEffect, useRef } from 'preact/hooks'
import type { Item } from '../api'
import { t } from '../i18n'
import { openOverlay } from '../lib/overlay'
import { me } from '../live'
import { addFiles, draft, pending, sending } from '../lib/draft'
import { utf8Len } from '../lib/meta'
import { Composer, sendText, TEXT_LIMIT } from './Composer'
import { ConnBanner } from './ConnBanner'
import { DirBrowser } from './DirBrowser'
import { Feed } from './Feed'
import { PeersPanel } from './PeersPanel'
import { Settings } from './Settings'
import { ShareDialog } from './ShareDialog'
import { toast } from './Toast'
import { TopBar } from './TopBar'

function isEditable(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable
}

function hasFiles(e: DragEvent): boolean {
  return !!e.dataTransfer && Array.from(e.dataTransfer.types).includes('Files')
}

export function MainPage() {
  const dragging = useSignal(false)
  const ta = useRef<HTMLTextAreaElement>(null)
  const panel = useSignal<'' | 'peers' | 'settings'>('')
  const sharing = useSignal<Item | null>(null)
  const tab = useSignal<'messages' | 'folder'>('messages')
  const hasDir = !!me.value?.has_dir

  useEffect(() => {
    let depth = 0

    function onPaste(e: ClipboardEvent) {
      if (openOverlay.value > 0) return // a dialog/panel is open
      if (isEditable(e.target) || isEditable(document.activeElement)) return
      const cd = e.clipboardData
      if (!cd) return
      const files = Array.from(cd.files)
      if (files.length) {
        e.preventDefault()
        addFiles(files, { pasted: true })
        ta.current?.focus()
        return
      }
      const text = cd.getData('text/plain')
      if (!text.trim()) return
      e.preventDefault()
      if (sending.value) {
        // The draft is being sent; changing it now would make the next send repeat it.
        toast(t('composer.busy'), 'err')
        return
      }
      if (draft.value.trim() || pending.value.length) {
        // A draft is in progress: the text joins it instead of being sent on its own.
        draft.value += text
        ta.current?.focus()
        return
      }
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
      const files = hasFiles(e)
      // Text/link drags into an editable field keep the browser default (drop inserts text).
      if (!files && isEditable(e.target)) return
      // Everything else: prevent default so a stray drop never navigates away from the app.
      e.preventDefault()
      if (e.dataTransfer) e.dataTransfer.dropEffect = files ? 'copy' : 'none'
    }
    function onDrop(e: DragEvent) {
      depth = 0
      dragging.value = false
      if (!hasFiles(e) && isEditable(e.target)) return
      e.preventDefault()
      const files = e.dataTransfer ? Array.from(e.dataTransfer.files) : []
      if (files.length) addFiles(files)
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

  const showFolder = hasDir && tab.value === 'folder'
  return (
    <div class="main">
      <TopBar
        onPeersClick={() => { panel.value = panel.value === 'peers' ? '' : 'peers' }}
        onSettingsClick={() => { panel.value = 'settings' }}
      />
      <ConnBanner />
      <div class="main-col">
        {hasDir && (
          <div class="tabs" role="tablist">
            <button role="tab" class={`tab${!showFolder ? ' on' : ''}`} aria-selected={!showFolder} onClick={() => { tab.value = 'messages' }}>{t('dir.tab_messages')}</button>
            <button role="tab" class={`tab${showFolder ? ' on' : ''}`} aria-selected={showFolder} onClick={() => { tab.value = 'folder' }}>{t('dir.tab_folder')}</button>
          </div>
        )}
        {showFolder ? <DirBrowser /> : (
          <>
            <Composer dragging={dragging.value} textareaRef={ta} />
            <Feed onShare={(it) => { sharing.value = it }} />
          </>
        )}
      </div>
      {panel.value === 'peers' && <PeersPanel onClose={() => { panel.value = '' }} />}
      {panel.value === 'settings' && <Settings onClose={() => { panel.value = '' }} />}
      {sharing.value && <ShareDialog item={sharing.value} onClose={() => { sharing.value = null }} />}
    </div>
  )
}
