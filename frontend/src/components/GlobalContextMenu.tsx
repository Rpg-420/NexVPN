import { useState, useEffect, useRef } from 'react'
import { Copy, Scissors, ClipboardPaste, CheckSquare, RefreshCw } from 'lucide-react'
import { t } from '../lib/i18n'

interface Position {
  x: number
  y: number
}

export default function GlobalContextMenu() {
  const [visible, setVisible] = useState(false)
  const [position, setPosition] = useState<Position>({ x: 0, y: 0 })
  const [targetElement, setTargetElement] = useState<HTMLElement | null>(null)
  const [isInput, setIsInput] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleContextMenu = (e: MouseEvent) => {
      e.preventDefault()
      const target = e.target as HTMLElement
      setTargetElement(target)

      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable

      setIsInput(isInputField)

      // Calculate position so menu stays on-screen
      const menuWidth = 180
      const menuHeight = 200
      let posX = e.clientX
      let posY = e.clientY

      if (posX + menuWidth > window.innerWidth) {
        posX = window.innerWidth - menuWidth - 10
      }
      if (posY + menuHeight > window.innerHeight) {
        posY = window.innerHeight - menuHeight - 10
      }

      setPosition({ x: Math.max(10, posX), y: Math.max(10, posY) })
      setVisible(true)
    }

    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setVisible(false)
      }
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setVisible(false)
      }
    }

    window.addEventListener('contextmenu', handleContextMenu)
    window.addEventListener('mousedown', handleClickOutside)
    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.removeEventListener('contextmenu', handleContextMenu)
      window.removeEventListener('mousedown', handleClickOutside)
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [])

  const handleCopy = async () => {
    setVisible(false)
    const selected = window.getSelection()?.toString() || ''
    if (selected) {
      await navigator.clipboard.writeText(selected)
    } else if (targetElement && (targetElement as HTMLInputElement).value) {
      await navigator.clipboard.writeText((targetElement as HTMLInputElement).value)
    }
  }

  const handleCut = async () => {
    setVisible(false)
    if (targetElement && isInput) {
      const input = targetElement as HTMLInputElement
      const start = input.selectionStart || 0
      const end = input.selectionEnd || 0
      const text = input.value.substring(start, end)
      if (text) {
        await navigator.clipboard.writeText(text)
        input.value = input.value.substring(0, start) + input.value.substring(end)
        input.dispatchEvent(new Event('input', { bubbles: true }))
      }
    }
  }

  const handlePaste = async () => {
    setVisible(false)
    try {
      const text = await navigator.clipboard.readText()
      if (targetElement && isInput) {
        const input = targetElement as HTMLInputElement
        const start = input.selectionStart || 0
        const end = input.selectionEnd || 0
        const currentVal = input.value || ''
        input.value = currentVal.substring(0, start) + text + currentVal.substring(end)
        input.setSelectionRange(start + text.length, start + text.length)
        input.dispatchEvent(new Event('input', { bubbles: true }))
        input.dispatchEvent(new Event('change', { bubbles: true }))
      }
    } catch (err) {
      console.error('Paste error:', err)
    }
  }

  const handleSelectAll = () => {
    setVisible(false)
    if (targetElement && isInput) {
      ;(targetElement as HTMLInputElement).select()
    } else {
      const range = document.createRange()
      range.selectNodeContents(document.body)
      const selection = window.getSelection()
      selection?.removeAllRanges()
      selection?.addRange(range)
    }
  }

  if (!visible) return null

  return (
    <div
      ref={menuRef}
      style={{ top: `${position.y}px`, left: `${position.x}px` }}
      className="fixed z-[9999] w-48 rounded-2xl bg-card/95 backdrop-blur-2xl border border-border shadow-2xl shadow-black/90 p-1.5 animate-in fade-in zoom-in-95 duration-100 select-none text-xs font-sans"
    >
      <div className="space-y-0.5">
        {/* Cut */}
        {isInput && (
          <button
            type="button"
            onClick={handleCut}
            className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-foreground hover:text-primary hover:bg-primary/15 transition-all text-start"
          >
            <div className="flex items-center gap-2.5">
              <Scissors size={14} className="text-primary" />
              <span>{t('context_cut')}</span>
            </div>
            <span className="text-[10px] text-muted-foreground font-mono">Ctrl+X</span>
          </button>
        )}

        {/* Copy */}
        <button
          type="button"
          onClick={handleCopy}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-foreground hover:text-primary hover:bg-primary/15 transition-all text-start"
        >
          <div className="flex items-center gap-2.5">
            <Copy size={14} className="text-success" />
            <span>{t('context_copy')}</span>
          </div>
          <span className="text-[10px] text-muted-foreground font-mono">Ctrl+C</span>
        </button>

        {/* Paste */}
        <button
          type="button"
          onClick={handlePaste}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-foreground hover:text-primary hover:bg-primary/15 transition-all text-start"
        >
          <div className="flex items-center gap-2.5">
            <ClipboardPaste size={14} className="text-warning" />
            <span>{t('context_paste')}</span>
          </div>
          <span className="text-[10px] text-muted-foreground font-mono">Ctrl+V</span>
        </button>

        {/* Select All */}
        <button
          type="button"
          onClick={handleSelectAll}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-foreground hover:text-primary hover:bg-primary/15 transition-all text-start"
        >
          <div className="flex items-center gap-2.5">
            <CheckSquare size={14} className="text-primary" />
            <span>{t('context_select_all')}</span>
          </div>
          <span className="text-[10px] text-muted-foreground font-mono">Ctrl+A</span>
        </button>

        <div className="border-t border-border my-1" />

        {/* Reload */}
        <button
          type="button"
          onClick={() => {
            setVisible(false)
            window.location.reload()
          }}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-muted-foreground hover:text-primary hover:bg-card transition-all text-start"
        >
          <div className="flex items-center gap-2.5">
            <RefreshCw size={13} className="text-muted-foreground" />
            <span>{t('context_reload')}</span>
          </div>
          <span className="text-[10px] text-muted-foreground font-mono">F5</span>
        </button>
      </div>
    </div>
  )
}
