import { useState } from 'react'
import {
  Sparkles,
  Zap,
  Globe,
  Shield,
  Layers,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  Check,
  RefreshCw,
  X,
} from 'lucide-react'
import { api } from '../lib/api'
import { ModalShell } from '../components/ui/modal-shell'

interface Props {
  isOpen: boolean
  onClose: () => void
  onCompleted: () => void
}

export default function OnboardingWizardModal({ isOpen, onClose, onCompleted }: Props) {
  const [step, setStep] = useState<number>(1)
  const [subscriptionURL, setSubscriptionURL] = useState<string>('')
  const [routingMode, setRoutingMode] = useState<string>('bypass_iran')
  const [proxyMode, setProxyMode] = useState<string>('tun')
  const [isProcessing, setIsProcessing] = useState<boolean>(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  if (!isOpen) return null

  const handleNextStep1 = async () => {
    try {
      const s = await api.getSettings()
      await api.saveSettings({
        ...s,
        routing_mode: routingMode,
        proxy_mode: proxyMode,
      })
      setStep(2)
    } catch (err: any) {
      setErrorMsg(err?.message || 'Failed to save settings')
    }
  }

  const handleImportSubscription = async () => {
    if (!subscriptionURL.trim()) {
      setErrorMsg('Please enter a subscription link or generate Cloudflare WARP below')
      return
    }
    setIsProcessing(true)
    setErrorMsg(null)
    try {
      await api.importSubscription(subscriptionURL.trim(), 'Primary Subscription')
      setStep(3)
    } catch (err: any) {
      setErrorMsg(err?.message || 'Failed to fetch subscription. Check your link or try Cloudflare WARP.')
    } finally {
      setIsProcessing(false)
    }
  }

  const handleGenerateWarp = async () => {
    setIsProcessing(true)
    setErrorMsg(null)
    try {
      await api.generateWarpProfile('162.159.192.1', 2408)
      setStep(3)
    } catch (err: any) {
      setErrorMsg(err?.message || 'Failed to generate WARP node.')
    } finally {
      setIsProcessing(false)
    }
  }

  const handleFinish = () => {
    localStorage.setItem('nexvpn_onboarding_completed', 'true')
    onCompleted()
    onClose()
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
    >
        {/* Header */}
        <div className="p-6 border-b border-border bg-background flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl border border-border bg-muted flex items-center justify-center text-muted-foreground">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-foreground">Welcome to NexVPN</h3>
              <p className="text-xs text-muted-foreground">Step {step} of 3 • Quick Setup Guide</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-5">
          {errorMsg && (
            <div className="p-3.5 rounded-2xl bg-destructive/10 border border-destructive/20 text-destructive text-xs">
              {errorMsg}
            </div>
          )}

          {/* STEP 1: Routing & Mode */}
          {step === 1 && (
            <div className="space-y-4">
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-foreground">Choose Your Routing & Tunnel Preference</h4>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Select how your internet traffic should be handled for maximum speed and privacy.
                </p>
              </div>

              <div className="space-y-2.5">
                <div
                  onClick={() => setRoutingMode('bypass_iran')}
                  className={`p-3.5 rounded-2xl border cursor-pointer transition-all flex items-center justify-between ${
                    routingMode === 'bypass_iran'
                      ? 'bg-primary/10 border-primary/40 text-primary'
                      : 'bg-background border-border text-muted-foreground hover:border-border'
                  }`}
                >
                  <div className="space-y-0.5">
                    <div className="text-xs font-bold text-foreground flex items-center gap-1.5">
                      <Globe className="w-3.5 h-3.5 text-primary" />
                      Bypass Domestic Sites (Recommended)
                    </div>
                    <div className="text-[11px] text-muted-foreground">
                      Iranian bank websites and domestic portals open directly at full national speed.
                    </div>
                  </div>
                  {routingMode === 'bypass_iran' && <Check className="w-4 h-4 text-primary" />}
                </div>

                <div
                  onClick={() => setRoutingMode('global')}
                  className={`p-3.5 rounded-2xl border cursor-pointer transition-all flex items-center justify-between ${
                    routingMode === 'global'
                      ? 'bg-primary/10 border-primary/40 text-primary'
                      : 'bg-background border-border text-muted-foreground hover:border-border'
                  }`}
                >
                  <div className="space-y-0.5">
                    <div className="text-xs font-bold text-foreground flex items-center gap-1.5">
                      <Shield className="w-3.5 h-3.5 text-primary" />
                      Global Tunnel Mode (All Traffic via VPN)
                    </div>
                    <div className="text-[11px] text-muted-foreground">
                      Routes 100% of network traffic through the encrypted VPN tunnel.
                    </div>
                  </div>
                  {routingMode === 'global' && <Check className="w-4 h-4 text-primary" />}
                </div>
              </div>

              <div className="space-y-2 pt-2 border-t border-border">
                <label className="text-xs font-bold text-foreground flex items-center gap-1.5">
                  <Layers className="w-3.5 h-3.5 text-primary" />
                  Tunnel Interface Engine
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setProxyMode('tun')}
                    className={`p-3 rounded-2xl border text-start transition-all ${
                      proxyMode === 'tun'
                        ? 'bg-primary/10 border-primary/40 text-primary'
                        : 'bg-background border-border text-muted-foreground'
                    }`}
                  >
                    <div className="text-xs font-bold text-foreground">TUN Virtual Adapter</div>
                    <div className="text-[10px] text-muted-foreground mt-0.5">System-wide & Game UDP support</div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setProxyMode('system_proxy')}
                    className={`p-3 rounded-2xl border text-start transition-all ${
                      proxyMode === 'system_proxy'
                        ? 'bg-primary/10 border-primary/40 text-primary'
                        : 'bg-background border-border text-muted-foreground'
                    }`}
                  >
                    <div className="text-xs font-bold text-foreground">System Proxy</div>
                    <div className="text-[10px] text-muted-foreground mt-0.5">Standard HTTP / SOCKS5 proxy</div>
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* STEP 2: Import Subscription or WARP */}
          {step === 2 && (
            <div className="space-y-4">
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-foreground">Import Nodes or Create Free Cloudflare WARP</h4>
                <p className="text-xs text-muted-foreground">
                  Paste your V2Ray / Clash / Sub link, or generate an instant clean Cloudflare WARP tunnel.
                </p>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-semibold text-muted-foreground">Subscription Link</label>
                <input
                  type="text"
                  value={subscriptionURL}
                  onChange={e => setSubscriptionURL(e.target.value)}
                  placeholder="https://example.com/api/v1/client/subscribe?token=..."
                  className="w-full p-3.5 rounded-2xl bg-card/80 border border-border text-foreground font-mono text-xs focus:border-primary focus:outline-none"
                />
                <button
                  type="button"
                  onClick={handleImportSubscription}
                  disabled={isProcessing || !subscriptionURL.trim()}
                  className="w-full py-2.5 rounded-xl bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-xs transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  {isProcessing ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
                  Import Subscription
                </button>
              </div>

              <div className="relative py-2 flex items-center justify-center">
                <div className="border-t border-border w-full" />
                <span className="bg-card px-3 text-[11px] font-semibold text-muted-foreground uppercase absolute">
                  Or
                </span>
              </div>

              <div className="p-4 rounded-2xl bg-background border border-border flex items-center justify-between">
                <div className="space-y-0.5">
                  <div className="text-xs font-bold text-foreground">No subscription link?</div>
                  <div className="text-[11px] text-muted-foreground">Generate an unlimited Cloudflare WARP Clean IP node</div>
                </div>
                <button
                  type="button"
                  onClick={handleGenerateWarp}
                  disabled={isProcessing}
                  className="px-4 py-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground border border-border text-xs font-semibold flex items-center gap-1.5 transition-all"
                >
                  {isProcessing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
                  Create WARP
                </button>
              </div>
            </div>
          )}

          {/* STEP 3: Finished */}
          {step === 3 && (
            <div className="py-6 flex flex-col items-center justify-center text-center space-y-4">
              <div className="w-16 h-16 rounded-full bg-success/10 border border-success/20 flex items-center justify-center text-success animate-bounce">
                <CheckCircle2 className="w-8 h-8" />
              </div>
              <div className="space-y-1">
                <h4 className="text-lg font-bold text-foreground">You're Ready to Connect!</h4>
                <p className="text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  Your configuration is set up. You can now click Connect to activate your fast, anti-censorship tunnel.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border bg-background flex items-center justify-between">
          {step > 1 && step < 3 ? (
            <button
              onClick={() => setStep(step - 1)}
              className="px-4 py-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground text-xs font-semibold flex items-center gap-1.5 transition-all"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              Back
            </button>
          ) : (
            <div />
          )}

          {step === 1 && (
            <button
              onClick={handleNextStep1}
              className="px-5 py-2.5 rounded-xl bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-xs flex items-center gap-2 transition-all ms-auto"
            >
              Next Step
              <ArrowRight className="w-4 h-4" />
            </button>
          )}

          {step === 3 && (
            <button
              onClick={handleFinish}
              className="px-6 py-2.5 rounded-xl bg-primary hover:bg-primary/90 text-primary-foreground font-bold text-xs flex items-center gap-2 transition-all ms-auto shadow-sm"
            >
              Go to Dashboard
              <Check className="w-4 h-4" />
            </button>
          )}
        </div>
      </ModalShell>
  )
}
