// Web Audio API Futuristic Sound Synthesizer (Zero External MP3 Assets Required)

class SoundFX {
  private ctx: AudioContext | null = null
  private enabled: boolean = true

  constructor() {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('nexvpn_sound_enabled')
      this.enabled = saved !== 'false'
    }
  }

  public setEnabled(val: boolean) {
    this.enabled = val
    if (typeof window !== 'undefined') {
      localStorage.setItem('nexvpn_sound_enabled', val ? 'true' : 'false')
    }
  }

  public isEnabled(): boolean {
    return this.enabled
  }

  private getContext(): AudioContext | null {
    if (!this.enabled) return null
    if (!this.ctx && typeof window !== 'undefined') {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext
      if (AudioCtx) {
        this.ctx = new AudioCtx()
      }
    }
    if (this.ctx && this.ctx.state === 'suspended') {
      this.ctx.resume()
    }
    return this.ctx
  }

  // Futuristic Cyberpunk Power-Up Sound (Connected)
  public playConnect() {
    const ctx = this.getContext()
    if (!ctx) return

    try {
      const now = ctx.currentTime

      // Main Oscillator (Upward chirp)
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      osc.type = 'sine'
      osc.frequency.setValueAtTime(440, now)
      osc.frequency.exponentialRampToValueAtTime(880, now + 0.18)

      gain.gain.setValueAtTime(0.01, now)
      gain.gain.linearRampToValueAtTime(0.12, now + 0.05)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.3)

      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(now)
      osc.stop(now + 0.3)

      // Harmonics Sub-oscillator
      const osc2 = ctx.createOscillator()
      const gain2 = ctx.createGain()
      osc2.type = 'triangle'
      osc2.frequency.setValueAtTime(660, now + 0.08)
      osc2.frequency.exponentialRampToValueAtTime(1320, now + 0.28)

      gain2.gain.setValueAtTime(0.01, now + 0.08)
      gain2.gain.linearRampToValueAtTime(0.08, now + 0.15)
      gain2.gain.exponentialRampToValueAtTime(0.001, now + 0.4)

      osc2.connect(gain2)
      gain2.connect(ctx.destination)
      osc2.start(now + 0.08)
      osc2.stop(now + 0.4)
    } catch {
      // Ignore audio context errors
    }
  }

  // Futuristic Power-Down Sound (Disconnected)
  public playDisconnect() {
    const ctx = this.getContext()
    if (!ctx) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      osc.type = 'sine'
      osc.frequency.setValueAtTime(750, now)
      osc.frequency.exponentialRampToValueAtTime(220, now + 0.25)

      gain.gain.setValueAtTime(0.1, now)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.28)

      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(now)
      osc.stop(now + 0.28)
    } catch {
      // Ignore audio context errors
    }
  }

  // Radar Ping / Test Blip
  public playPing() {
    const ctx = this.getContext()
    if (!ctx) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      osc.type = 'sine'
      osc.frequency.setValueAtTime(1050, now)

      gain.gain.setValueAtTime(0.06, now)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.09)

      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(now)
      osc.stop(now + 0.09)
    } catch {
      // Ignore audio context errors
    }
  }

  // Success / Action Complete Chime
  public playSuccess() {
    const ctx = this.getContext()
    if (!ctx) return

    try {
      const now = ctx.currentTime
      const freqs = [523.25, 659.25, 783.99] // C5, E5, G5
      freqs.forEach((f, i) => {
        const osc = ctx.createOscillator()
        const gain = ctx.createGain()
        const startTime = now + i * 0.07

        osc.type = 'sine'
        osc.frequency.setValueAtTime(f, startTime)

        gain.gain.setValueAtTime(0.01, startTime)
        gain.gain.linearRampToValueAtTime(0.07, startTime + 0.02)
        gain.gain.exponentialRampToValueAtTime(0.001, startTime + 0.2)

        osc.connect(gain)
        gain.connect(ctx.destination)
        osc.start(startTime)
        osc.stop(startTime + 0.2)
      })
    } catch {
      // Ignore audio context errors
    }
  }
}

export const sound = new SoundFX()
