import React, { useState } from 'react'
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-lg animate-fade-in">
      <div className="relative w-full max-w-xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="p-6 border-b border-neutral-800 bg-neutral-950/40 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white">Welcome to NexVPN</h3>
              <p className="text-xs text-neutral-400">Step {step} of 3 • Quick Setup Guide</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-5">
          {errorMsg && (
            <div className="p-3.5 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
              {errorMsg}
            </div>
          )}

          {/* STEP 1: Routing & Mode */}
          {step === 1 && (
            <div className="space-y-4">
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-white">Choose Your Routing & Tunnel Preference</h4>
                <p className="text-xs text-neutral-400 leading-relaxed">
                  Select how your internet traffic should be handled for maximum speed and privacy.
                </p>
              </div>

              <div className="space-y-2.5">
                <div
                  onClick={() => setRoutingMode('bypass_iran')}
                  className={`p-3.5 rounded-2xl border cursor-pointer transition-all flex items-center justify-between ${
                    routingMode === 'bypass_iran'
                      ? 'bg-cyan-500/10 border-cyan-500/40 text-cyan-300'
                      : 'bg-neutral-950/40 border-neutral-800 text-neutral-400 hover:border-neutral-700'
                  }`}
                >
                  <div className="space-y-0.5">
                    <div className="text-xs font-bold text-white flex items-center gap-1.5">
                      <Globe className="w-3.5 h-3.5 text-cyan-400" />
                      Bypass Domestic Sites (Recommended)
                    </div>
                    <div className="text-[11px] text-neutral-400">
                      Iranian bank websites and domestic portals open directly at full national speed.
                    </div>
                  </div>
                  {routingMode === 'bypass_iran' && <Check className="w-4 h-4 text-cyan-400" />}
                </div>

                <div
                  onClick={() => setRoutingMode('global')}
                  className={`p-3.5 rounded-2xl border cursor-pointer transition-all flex items-center justify-between ${
                    routingMode === 'global'
                      ? 'bg-cyan-500/10 border-cyan-500/40 text-cyan-300'
                      : 'bg-neutral-950/40 border-neutral-800 text-neutral-400 hover:border-neutral-700'
                  }`}
                >
                  <div className="space-y-0.5">
                    <div className="text-xs font-bold text-white flex items-center gap-1.5">
                      <Shield className="w-3.5 h-3.5 text-blue-400" />
                      Global Tunnel Mode (All Traffic via VPN)
                    </div>
                    <div className="text-[11px] text-neutral-400">
                      Routes 100% of network traffic through the encrypted VPN tunnel.
                    </div>
                  </div>
                  {routingMode === 'global' && <Check className="w-4 h-4 text-cyan-400" />}
                </div>
              </div>

              <div className="space-y-2 pt-2 border-t border-neutral-800/80">
                <label className="text-xs font-bold text-white flex items-center gap-1.5">
                  <Layers className="w-3.5 h-3.5 text-purple-400" />
                  Tunnel Interface Engine
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setProxyMode('tun')}
                    className={`p-3 rounded-2xl border text-left transition-all ${
                      proxyMode === 'tun'
                        ? 'bg-purple-500/10 border-purple-500/40 text-purple-300'
                        : 'bg-neutral-950/40 border-neutral-800 text-neutral-400'
                    }`}
                  >
                    <div className="text-xs font-bold text-white">TUN Virtual Adapter</div>
                    <div className="text-[10px] text-neutral-400 mt-0.5">System-wide & Game UDP support</div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setProxyMode('system_proxy')}
                    className={`p-3 rounded-2xl border text-left transition-all ${
                      proxyMode === 'system_proxy'
                        ? 'bg-purple-500/10 border-purple-500/40 text-purple-300'
                        : 'bg-neutral-950/40 border-neutral-800 text-neutral-400'
                    }`}
                  >
                    <div className="text-xs font-bold text-white">System Proxy</div>
                    <div className="text-[10px] text-neutral-400 mt-0.5">Standard HTTP / SOCKS5 proxy</div>
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* STEP 2: Import Subscription or WARP */}
          {step === 2 && (
            <div className="space-y-4">
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-white">Import Nodes or Create Free Cloudflare WARP</h4>
                <p className="text-xs text-neutral-400">
                  Paste your V2Ray / Clash / Sub link, or generate an instant clean Cloudflare WARP tunnel.
                </p>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-semibold text-neutral-300">Subscription Link</label>
                <input
                  type="text"
                  value={subscriptionURL}
                  onChange={e => setSubscriptionURL(e.target.value)}
                  placeholder="https://example.com/api/v1/client/subscribe?token=..."
                  className="w-full p-3.5 rounded-2xl bg-neutral-950/80 border border-neutral-800 text-white font-mono text-xs focus:border-cyan-500 focus:outline-none"
                />
                <button
                  type="button"
                  onClick={handleImportSubscription}
                  disabled={isProcessing || !subscriptionURL.trim()}
                  className="w-full py-2.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-semibold text-xs transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  {isProcessing ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
                  Import Subscription
                </button>
              </div>

              <div className="relative py-2 flex items-center justify-center">
                <div className="border-t border-neutral-800 w-full" />
                <span className="bg-neutral-900 px-3 text-[11px] font-semibold text-neutral-500 uppercase absolute">
                  Or
                </span>
              </div>

              <div className="p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800 flex items-center justify-between">
                <div className="space-y-0.5">
                  <div className="text-xs font-bold text-white">No subscription link?</div>
                  <div className="text-[11px] text-neutral-400">Generate an unlimited Cloudflare WARP Clean IP node</div>
                </div>
                <button
                  type="button"
                  onClick={handleGenerateWarp}
                  disabled={isProcessing}
                  className="px-4 py-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-cyan-300 border border-cyan-500/30 text-xs font-semibold flex items-center gap-1.5 transition-all"
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
              <div className="w-16 h-16 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 animate-bounce">
                <CheckCircle2 className="w-8 h-8" />
              </div>
              <div className="space-y-1">
                <h4 className="text-lg font-bold text-white">You're Ready to Connect!</h4>
                <p className="text-xs text-neutral-400 max-w-sm mx-auto leading-relaxed">
                  Your configuration is set up. You can now click Connect to activate your fast, anti-censorship tunnel.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800 bg-neutral-950/60 flex items-center justify-between">
          {step > 1 && step < 3 ? (
            <button
              onClick={() => setStep(step - 1)}
              className="px-4 py-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-semibold flex items-center gap-1.5 transition-all"
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
              className="px-5 py-2.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-semibold text-xs flex items-center gap-2 transition-all ml-auto"
            >
              Next Step
              <ArrowRight className="w-4 h-4" />
            </button>
          )}

          {step === 3 && (
            <button
              onClick={handleFinish}
              className="px-6 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs flex items-center gap-2 transition-all ml-auto shadow-lg shadow-emerald-500/20"
            >
              Go to Dashboard
              <Check className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
