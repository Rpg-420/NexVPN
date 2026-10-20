import { useState } from 'react'
import {
  FileCode2,
  Copy,
  Check,
  Download,
  ArrowRightLeft,
  X,
  Sparkles,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'

interface Props {
  isOpen: boolean
  onClose: () => void
}

type FormatOption = 'clash' | 'singbox' | 'base64' | 'links'

export default function ConfigConverterModal({ isOpen, onClose }: Props) {
  const [inputContent, setInputContent] = useState('')
  const [targetFormat, setTargetFormat] = useState<FormatOption>('clash')
  const [outputContent, setOutputContent] = useState('')
  const [isConverting, setIsConverting] = useState(false)
  const [copied, setCopied] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  if (!isOpen) return null

  const handleConvert = async () => {
    if (!inputContent.trim()) return
    setIsConverting(true)
    setErrorMsg(null)
    setOutputContent('')

    try {
      const res = await api.convertConfig(inputContent.trim(), targetFormat)
      if (res && typeof res === 'string') {
        setOutputContent(res)
        sound.playSuccess()
      } else {
        setErrorMsg('Conversion failed. Please verify input format.')
      }
    } catch (err: any) {
      setErrorMsg(err?.message || 'Conversion error')
    } finally {
      setIsConverting(false)
    }
  }

  const handleCopy = () => {
    if (!outputContent) return
    navigator.clipboard.writeText(outputContent)
    setCopied(true)
    sound.playSuccess()
    setTimeout(() => setCopied(false), 2000)
  }

  const handleDownload = () => {
    if (!outputContent) return
    let filename = 'nexvpn_converted'
    let mimeType = 'text/plain'

    if (targetFormat === 'clash') {
      filename += '.yaml'
      mimeType = 'text/yaml'
    } else if (targetFormat === 'singbox') {
      filename += '.json'
      mimeType = 'application/json'
    } else {
      filename += '.txt'
    }

    const blob = new Blob([outputContent], { type: mimeType })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-warning/20 border border-warning/30 flex items-center justify-center text-warning">
              <ArrowRightLeft className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">
                <span>{t('config_converter_title') || 'Universal Config Converter'}</span>
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-warning/20 text-warning border border-warning/30">
                  Universal Hub
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">
                {t('config_converter_subtitle') || 'Convert configs between Clash Meta YAML, Sing-Box JSON, and Base64'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 overflow-y-auto flex-1">
          {/* Format Selector Pills */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-muted-foreground">
              {t('target_format_label') || 'Target Output Format:'}
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {[
                { id: 'clash', label: 'Clash Meta (YAML)', badge: 'Mihomo / Clash' },
                { id: 'singbox', label: 'Sing-Box (JSON)', badge: 'Sing-Box Core' },
                { id: 'base64', label: 'Base64 Sub', badge: 'Standard Sub' },
                { id: 'links', label: 'Clean Links', badge: 'VLESS/VMess/Trojan' },
              ].map(f => (
                <button
                  key={f.id}
                  type="button"
                  onClick={() => setTargetFormat(f.id as FormatOption)}
                  className={`p-2.5 rounded-2xl border text-start rtl:text-end flex flex-col justify-between transition-all ${
                    targetFormat === f.id
                      ? 'bg-warning/20 border-warning/50 text-warning-foreground shadow-sm'
                      : 'bg-background border-border text-muted-foreground hover:text-foreground hover:bg-muted'
                  }`}
                >
                  <span className="text-xs font-bold">{f.label}</span>
                  <span className="text-[10px] text-muted-foreground mt-0.5">{f.badge}</span>
                </button>
              ))}
            </div>
          </div>

          {/* Input Text Area */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs font-bold text-muted-foreground">
                {t('paste_raw_configs_label') || 'Input Configs (Raw links or subscription content):'}
              </label>
              {inputContent && (
                <button
                  onClick={() => setInputContent('')}
                  className="text-[11px] text-muted-foreground hover:text-destructive transition"
                >
                  {t('clear_all')}
                </button>
              )}
            </div>
            <textarea
              rows={5}
              value={inputContent}
              onChange={e => setInputContent(e.target.value)}
              placeholder="vless://uuid@server:443?security=reality&sni=example.com#MyServer&#10;vmess://...&#10;trojan://password@server:443#TrojanServer"
              className="w-full p-3 bg-background border border-border rounded-2xl font-mono text-xs text-foreground placeholder-muted-foreground focus:outline-none focus:border-warning/50 resize-y"
            />
          </div>

          {/* Convert Action Button */}
          <button
            type="button"
            onClick={handleConvert}
            disabled={isConverting || !inputContent.trim()}
            className="bg-primary w-full py-2.5 px-4 disabled:opacity-50 text-primary-foreground rounded-xl text-xs font-bold shadow-sm flex items-center justify-center gap-2 active:scale-95 transition-all"
          >
            {isConverting ? (
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Sparkles className="w-3.5 h-3.5" />
            )}
            <span>{isConverting ? t('converting') || 'Converting...' : t('convert_now_btn') || 'Convert Configs'}</span>
          </button>

          {/* Error Banner */}
          {errorMsg && (
            <div className="p-3 bg-destructive/40 border border-destructive/40 rounded-xl text-xs text-destructive">
              {errorMsg}
            </div>
          )}

          {/* Output Area */}
          {outputContent && (
            <div className="space-y-2 pt-2 border-t border-border animate-in fade-in duration-200">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-success flex items-center gap-1.5">
                  <FileCode2 className="w-4 h-4" />
                  <span>{t('converted_output_label') || 'Converted Output:'}</span>
                </span>
                <div className="flex items-center gap-2">
                  <button
                    onClick={handleCopy}
                    className="flex items-center gap-1 px-3 py-1 bg-muted hover:bg-muted/90 text-foreground rounded-lg text-xs font-semibold transition"
                  >
                    {copied ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
                    <span>{copied ? t('copied') : t('copy')}</span>
                  </button>
                  <button
                    onClick={handleDownload}
                    className="flex items-center gap-1 px-3 py-1 bg-primary/80 hover:bg-primary text-primary-foreground rounded-lg text-xs font-semibold transition"
                  >
                    <Download className="w-3 h-3" />
                    <span>{t('download_file') || 'Download'}</span>
                  </button>
                </div>
              </div>

              <textarea
                readOnly
                rows={6}
                value={outputContent}
                className="w-full p-3 bg-background border border-border rounded-2xl font-mono text-xs text-success focus:outline-none select-all resize-y"
              />
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end p-4 border-t border-border bg-background">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-muted hover:bg-muted/90 text-muted-foreground rounded-xl text-xs font-semibold"
          >
            {t('close')}
          </button>
        </div>
      </ModalShell>
  )
}
