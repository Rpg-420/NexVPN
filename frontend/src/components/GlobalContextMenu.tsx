import React, { useState, useEffect, useRef } from 'react'
import { Copy, Scissors, ClipboardPaste, CheckSquare, RefreshCw } from 'lucide-react'
import { getLanguage } from '../lib/i18n'

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
  const isFa = getLanguage() === 'fa'

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
      className="fixed z-[9999] w-48 rounded-2xl bg-slate-900/95 backdrop-blur-2xl border border-cyan-500/30 shadow-2xl shadow-black/90 p-1.5 animate-in fade-in zoom-in-95 duration-100 select-none text-xs font-sans"
    >
      <div className="space-y-0.5">
        {/* Cut */}
        {isInput && (
          <button
            type="button"
            onClick={handleCut}
            className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-slate-200 hover:text-white hover:bg-cyan-500/15 transition-all text-left rtl:text-right"
          >
            <div className="flex items-center gap-2.5">
              <Scissors size={14} className="text-cyan-400" />
              <span>{isFa ? 'برش' : 'Cut'}</span>
            </div>
            <span className="text-[10px] text-slate-500 font-mono">Ctrl+X</span>
          </button>
        )}

        {/* Copy */}
        <button
          type="button"
          onClick={handleCopy}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-slate-200 hover:text-white hover:bg-cyan-500/15 transition-all text-left rtl:text-right"
        >
          <div className="flex items-center gap-2.5">
            <Copy size={14} className="text-emerald-400" />
            <span>{isFa ? 'کپی' : 'Copy'}</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono">Ctrl+C</span>
        </button>

        {/* Paste */}
        <button
          type="button"
          onClick={handlePaste}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-slate-200 hover:text-white hover:bg-cyan-500/15 transition-all text-left rtl:text-right"
        >
          <div className="flex items-center gap-2.5">
            <ClipboardPaste size={14} className="text-amber-400" />
            <span>{isFa ? 'چسباندن' : 'Paste'}</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono">Ctrl+V</span>
        </button>

        {/* Select All */}
        <button
          type="button"
          onClick={handleSelectAll}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-slate-200 hover:text-white hover:bg-cyan-500/15 transition-all text-left rtl:text-right"
        >
          <div className="flex items-center gap-2.5">
            <CheckSquare size={14} className="text-purple-400" />
            <span>{isFa ? 'انتخاب همه' : 'Select All'}</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono">Ctrl+A</span>
        </button>

        <div className="border-t border-white/5 my-1" />

        {/* Reload */}
        <button
          type="button"
          onClick={() => {
            setVisible(false)
            window.location.reload()
          }}
          className="w-full flex items-center justify-between px-3 py-2 rounded-xl text-slate-400 hover:text-cyan-300 hover:bg-slate-800 transition-all text-left rtl:text-right"
        >
          <div className="flex items-center gap-2.5">
            <RefreshCw size={13} className="text-slate-400" />
            <span>{isFa ? 'بارگذاری مجدد' : 'Reload'}</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono">F5</span>
        </button>
      </div>
    </div>
  )
}
