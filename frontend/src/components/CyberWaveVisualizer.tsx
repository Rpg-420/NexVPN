import { useEffect, useRef } from 'react'
import { resolveToken, resolveTokenRgba } from '../lib/theme'

/**
 * The canvas cannot read CSS variables, so the theme colours are sampled from
 * the live stylesheet. Sampled once per theme change rather than per frame —
 * `getComputedStyle` in a 60fps loop is a forced style recalculation each time.
 */
function readWaveColors() {
  return {
    primary: resolveToken('--primary', '#6366f1'),
    primaryFill: resolveTokenRgba('--primary', 0.25, '#6366f1'),
    primaryFillFaint: resolveTokenRgba('--primary', 0.05, '#6366f1'),
    primaryBars: resolveTokenRgba('--primary', 0.08, '#6366f1'),
    success: resolveToken('--success', '#10b981'),
    idle: resolveTokenRgba('--muted-foreground', 0.3, '#71717a'),
    idleBars: resolveTokenRgba('--foreground', 0.02, '#fafafa'),
    axis: resolveTokenRgba('--foreground', 0.06, '#fafafa'),
  }
}

interface Props {
  connected: boolean
  downloadSpeed: number // bytes per second
  uploadSpeed: number // bytes per second
  className?: string
}

export default function CyberWaveVisualizer({
  connected,
  downloadSpeed,
  uploadSpeed,
  className = '',
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const animFrameRef = useRef<number | null>(null)

  // Kept in refs (rather than effect deps) so the render loop always reads the latest
  // values without tearing down and recreating the requestAnimationFrame loop on every
  // stats tick -- restarting the loop reset `phase` to 0 constantly, causing the wave to
  // visibly stutter/jump instead of animating smoothly.
  const connectedRef = useRef(connected)
  const downloadSpeedRef = useRef(downloadSpeed)
  const uploadSpeedRef = useRef(uploadSpeed)
  connectedRef.current = connected
  downloadSpeedRef.current = downloadSpeed
  uploadSpeedRef.current = uploadSpeed

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let phase = 0
    let colors = readWaveColors()
    const handleThemeChange = () => { colors = readWaveColors() }
    window.addEventListener('nexvpn:theme-changed', handleThemeChange)

    const render = () => {
      const connected = connectedRef.current
      const downloadSpeed = downloadSpeedRef.current
      const uploadSpeed = uploadSpeedRef.current
      const width = canvas.width
      const height = canvas.height
      ctx.clearRect(0, 0, width, height)

      // Speed in MB/s
      const dlMB = downloadSpeed / (1024 * 1024)
      const ulMB = uploadSpeed / (1024 * 1024)
      const totalSpeed = dlMB + ulMB

      // Dynamic amplitude and frequency scaling
      const baseAmp = connected ? Math.min(height * 0.42, 6 + totalSpeed * 3.5) : 3
      const freqFactor = connected ? Math.min(0.06, 0.015 + totalSpeed * 0.006) : 0.01

      phase += freqFactor

      // 1. Draw Background Grid & Spectrum Bars
      const numBars = 32
      const barWidth = width / numBars
      for (let i = 0; i < numBars; i++) {
        const x = i * barWidth + barWidth * 0.2
        const barH = connected
          ? Math.sin(i * 0.4 + phase * 2) * (baseAmp * 0.7) + (baseAmp * 0.8)
          : 2
        
        ctx.fillStyle = connected ? colors.primaryBars : colors.idleBars
        ctx.fillRect(x, height / 2 - barH / 2, barWidth * 0.6, barH)
      }

      // 2. Download Wave with Gradient Glow Fill
      const dlGrad = ctx.createLinearGradient(0, 0, 0, height)
      dlGrad.addColorStop(0, colors.primaryFill)
      dlGrad.addColorStop(0.5, colors.primaryFillFaint)
      dlGrad.addColorStop(1, 'transparent')

      ctx.beginPath()
      ctx.moveTo(0, height / 2)
      for (let x = 0; x <= width; x += 3) {
        const y =
          height / 2 +
          Math.sin(x * 0.02 + phase) * baseAmp +
          Math.cos(x * 0.012 - phase * 0.7) * (baseAmp * 0.45)
        ctx.lineTo(x, y)
      }
      ctx.lineTo(width, height)
      ctx.lineTo(0, height)
      ctx.closePath()
      ctx.fillStyle = dlGrad
      ctx.fill()

      // Download Line Stroke
      ctx.beginPath()
      ctx.lineWidth = connected ? 2.5 : 1.5
      ctx.strokeStyle = connected ? colors.primary : colors.idle
      ctx.shadowColor = connected ? colors.primary : 'transparent'
      ctx.shadowBlur = connected ? 10 : 0

      for (let x = 0; x <= width; x += 3) {
        const y =
          height / 2 +
          Math.sin(x * 0.02 + phase) * baseAmp +
          Math.cos(x * 0.012 - phase * 0.7) * (baseAmp * 0.45)
        if (x === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.stroke()

      // 3. Upload Wave with Success Glow
      if (connected) {
        ctx.beginPath()
        ctx.lineWidth = 2.0
        ctx.strokeStyle = colors.success
        ctx.shadowColor = colors.success
        ctx.shadowBlur = 8

        for (let x = 0; x <= width; x += 3) {
          const y =
            height / 2 +
            Math.sin(x * 0.028 - phase * 1.3) * (baseAmp * 0.65) +
            Math.cos(x * 0.018 + phase * 0.9) * (baseAmp * 0.35)
          if (x === 0) ctx.moveTo(x, y)
          else ctx.lineTo(x, y)
        }
        ctx.stroke()
      }

      // 4. Center Reference Axis
      ctx.shadowBlur = 0
      ctx.strokeStyle = colors.axis
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(0, height / 2)
      ctx.lineTo(width, height / 2)
      ctx.stroke()

      animFrameRef.current = requestAnimationFrame(render)
    }

    render()

    return () => {
      window.removeEventListener('nexvpn:theme-changed', handleThemeChange)
      if (animFrameRef.current) cancelAnimationFrame(animFrameRef.current)
    }
  }, [])

  return (
    <div className={`relative w-full h-full overflow-hidden rounded-xl bg-card/40 border border-border ${className}`}>
      <canvas
        ref={canvasRef}
        width={480}
        height={90}
        className="w-full h-full block"
      />
    </div>
  )
}
