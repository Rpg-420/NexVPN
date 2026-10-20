import { useState, useEffect } from 'react'
import { X, Wifi, Smartphone, Tv, Gamepad2, Copy, Check, Shield, QrCode } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import { generateQRCodeSVG } from '../lib/qrcode'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function LANShareModal({ isOpen, onClose }: Props) {
  const [localIP, setLocalIP] = useState('192.168.1.100')
  const [allowLan, setAllowLan] = useState(true)
  const [copiedType, setCopiedType] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'iphone' | 'android' | 'tv' | 'console'>('iphone')
  const [showQR, setShowQR] = useState(false)

  useEffect(() => {
    if (isOpen) {
      api.getLocalIPAddress().then(ip => {
        if (ip) setLocalIP(ip)
      }).catch(err => console.error('Get local IP error:', err))

      api.getSettings().then(s => {
        if (s && s.allow_lan !== undefined) setAllowLan(s.allow_lan)
      }).catch(err => console.error('Get settings error:', err))
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleCopy = (text: string, type: string) => {
    navigator.clipboard.writeText(text)
    setCopiedType(type)
    sound.playSuccess()
    setTimeout(() => setCopiedType(null), 2000)
  }

  const handleToggleLan = async (enabled: boolean) => {
    setAllowLan(enabled)
    try {
      await api.toggleAllowLan(enabled)
      sound.playSuccess()
    } catch (err) {
      console.error('Toggle allow lan error:', err)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
      bodyClassName="space-y-4 p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-primary/10 border border-primary/30 flex items-center justify-center text-primary shadow-inner">
              <Wifi size={18} />
            </div>
            <div>
              <h3 className="font-bold text-foreground text-sm">{t('lan_share_title')}</h3>
              <p className="text-xs text-muted-foreground">{t('lan_share_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Toggle & Status Banner */}
        <div className="flex items-center justify-between bg-muted p-3.5 rounded-xl border border-border">
          <div className="flex items-center gap-2.5">
            <Shield size={16} className={allowLan ? 'text-success' : 'text-muted-foreground'} />
            <div>
              <span className="text-xs font-bold text-foreground block">{t('lan_allow_title')}</span>
              <span className="text-[11px] text-muted-foreground">
                {allowLan ? t('lan_allow_desc_on') : t('lan_allow_desc_off')}
              </span>
            </div>
          </div>
          <label className="toggle">
            <input
              type="checkbox"
              checked={allowLan}
              onChange={e => handleToggleLan(e.target.checked)}
            />
            <span className="toggle-slider" />
          </label>
        </div>

        {/* IP & Port Details Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {/* HTTP Proxy */}
          <div className="bg-background border border-border rounded-xl p-3 space-y-1.5">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span className="font-semibold text-primary">{t('lan_http_title')}</span>
              <button
                onClick={() => handleCopy(`${localIP}:10809`, 'http')}
                className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-[11px]"
              >
                {copiedType === 'http' ? <Check size={12} className="text-success" /> : <Copy size={12} />}
                <span>{t('lan_copy_btn')}</span>
              </button>
            </div>
            <div className="font-mono text-sm font-bold text-foreground bg-card px-2.5 py-1 rounded border border-border flex items-center justify-between">
              <span>{localIP}</span>
              <span className="text-primary">{t('port_label')}: 10809</span>
            </div>
          </div>

          {/* SOCKS5 Proxy */}
          <div className="bg-background border border-border rounded-xl p-3 space-y-1.5">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span className="font-semibold text-primary">{t('lan_socks_title')}</span>
              <button
                onClick={() => handleCopy(`${localIP}:10808`, 'socks5')}
                className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-[11px]"
              >
                {copiedType === 'socks5' ? <Check size={12} className="text-success" /> : <Copy size={12} />}
                <span>{t('lan_copy_btn')}</span>
              </button>
            </div>
            <div className="font-mono text-sm font-bold text-foreground bg-card px-2.5 py-1 rounded border border-border flex items-center justify-between">
              <span>{localIP}</span>
              <span className="text-primary">{t('port_label')}: 10808</span>
            </div>
          </div>
        </div>

        {/* Platform Guides */}
        <div className="space-y-2 pt-1">
          <div className="flex items-center gap-1 border-b border-border pb-2 text-xs">
            <button
              onClick={() => setActiveTab('iphone')}
              className={`px-3 py-1 rounded-lg font-bold flex items-center gap-1.5 transition ${
                activeTab === 'iphone' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
              }`}
            >
              <Smartphone size={13} />
              <span>iPhone / iPad</span>
            </button>
            <button
              onClick={() => setActiveTab('android')}
              className={`px-3 py-1 rounded-lg font-bold flex items-center gap-1.5 transition ${
                activeTab === 'android' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
              }`}
            >
              <Smartphone size={13} />
              <span>Android</span>
            </button>
            <button
              onClick={() => setActiveTab('tv')}
              className={`px-3 py-1 rounded-lg font-bold flex items-center gap-1.5 transition ${
                activeTab === 'tv' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
              }`}
            >
              <Tv size={13} />
              <span>Smart TV</span>
            </button>
            <button
              onClick={() => setActiveTab('console')}
              className={`px-3 py-1 rounded-lg font-bold flex items-center gap-1.5 transition ${
                activeTab === 'console' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
              }`}
            >
              <Gamepad2 size={13} />
              <span>PS5 / Xbox</span>
            </button>
          </div>

          <div className="bg-background p-3 rounded-xl border border-border text-xs text-muted-foreground leading-relaxed min-h-[70px] flex items-center">
            {activeTab === 'iphone' && (
              <p>{t('lan_guide_iphone', { ip: localIP })}</p>
            )}
            {activeTab === 'android' && (
              <p>{t('lan_guide_android', { ip: localIP })}</p>
            )}
            {activeTab === 'tv' && (
              <p>{t('lan_guide_tv', { ip: localIP })}</p>
            )}
            {activeTab === 'console' && (
              <p>{t('lan_guide_console', { ip: localIP })}</p>
            )}
          </div>
        </div>

        {/* QR Code Section */}
        {showQR && (
          <div className="p-4 bg-background rounded-2xl border border-border flex flex-col items-center gap-3 animate-in fade-in duration-200">
            <div className="p-3 bg-card rounded-xl shadow-inner inline-block">
              <img src={generateQRCodeSVG(`http://${localIP}:10809`)} alt="QR Code" className="w-[180px] h-[180px]" />
            </div>
            <span className="text-[11px] font-mono text-muted-foreground">
              Scan with phone camera or proxy app: http://{localIP}:10809
            </span>
          </div>
        )}

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button variant="secondary" size="sm" className="text-xs flex items-center gap-1.5 text-muted-foreground hover:text-primary"
            type="button"
            onClick={() => setShowQR(q => !q)}
          >
            <QrCode size={13} />
            <span>{showQR ? 'Hide QR Code' : 'Show Setup QR Code'}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs" onClick={onClose}>
            {t('close')}
          </Button>
        </div>
      </ModalShell>
  )
}
