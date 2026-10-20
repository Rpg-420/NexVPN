import { useState, useEffect, useCallback, useRef } from 'react'
import { api } from '../lib/api'
import type { SpeedTestProgress } from '../types'

export type SpeedTestState = 'idle' | 'running' | 'completed' | 'error'

export interface SpeedTestMetrics extends SpeedTestProgress {
  state: SpeedTestState
  gaugeAngle: number // 0 to 180 degrees
}

const INITIAL_METRICS: SpeedTestMetrics = {
  state: 'idle',
  phase: 'ping',
  download_mbps: 0,
  upload_mbps: 0,
  ping_ms: 0,
  jitter_ms: 0,
  progress: 0,
  gaugeAngle: 0,
}

function calculateGaugeAngle(mbps: number): number {
  if (mbps <= 0) return 0
  if (mbps >= 100) return 180
  return Math.min(180, Math.round((Math.sqrt(mbps) / 10) * 180))
}

export function useSpeedTest(isOpen: boolean) {
  const [metrics, setMetrics] = useState<SpeedTestMetrics>(INITIAL_METRICS)
  const mockIntervalRef = useRef<number | null>(null)

  const stopMockSimulation = useCallback(() => {
    if (mockIntervalRef.current) {
      clearInterval(mockIntervalRef.current)
      mockIntervalRef.current = null
    }
  }, [])

  const startTest = useCallback(async () => {
    stopMockSimulation()
    setMetrics({
      ...INITIAL_METRICS,
      state: 'running',
      phase: 'ping',
      progress: 5,
    })

    const isWailsAvailable =
      typeof window !== 'undefined' &&
      (window as any).go?.main?.App?.StartRealSpeedTest

    if (isWailsAvailable) {
      try {
        await api.startRealSpeedTest()
      } catch (err: any) {
        setMetrics(prev => ({
          ...prev,
          state: 'error',
          phase: 'error',
          error_msg: err?.message || 'Failed to start speed test',
        }))
      }
    } else if ((import.meta as any)?.env?.DEV) {
      // Dev-only mock stream. NEVER reachable in a production build: if the real
      // binding is missing there we must surface an error, not invented numbers.
      let tick = 0
      mockIntervalRef.current = window.setInterval(() => {
        tick++
        if (tick <= 4) {
          // Ping Phase
          setMetrics(prev => ({
            ...prev,
            phase: 'ping',
            ping_ms: 32 + Math.floor(Math.random() * 12),
            jitter_ms: 2 + Math.floor(Math.random() * 3),
            progress: tick * 5,
          }))
        } else if (tick <= 18) {
          // Download Phase
          const speed = +(35 + Math.random() * 55).toFixed(1)
          setMetrics(prev => ({
            ...prev,
            phase: 'download',
            download_mbps: speed,
            gaugeAngle: calculateGaugeAngle(speed),
            progress: 20 + Math.floor(((tick - 4) / 14) * 50),
          }))
        } else if (tick <= 28) {
          // Upload Phase
          const speed = +(18 + Math.random() * 30).toFixed(1)
          setMetrics(prev => ({
            ...prev,
            phase: 'upload',
            upload_mbps: speed,
            gaugeAngle: calculateGaugeAngle(speed),
            progress: 70 + Math.floor(((tick - 18) / 10) * 30),
          }))
        } else {
          // Complete
          stopMockSimulation()
          setMetrics(prev => ({
            ...prev,
            state: 'completed',
            phase: 'complete',
            progress: 100,
            gaugeAngle: calculateGaugeAngle(prev.download_mbps),
          }))
        }
      }, 200)
    } else {
      // Production without the Wails binding: fail loudly rather than fabricate.
      setMetrics(prev => ({
        ...prev,
        state: 'error',
        phase: 'error',
        progress: 0,
        error_msg: 'Speed test backend is unavailable (Wails binding missing).',
      }))
    }
  }, [stopMockSimulation])

  const stopTest = useCallback(async () => {
    stopMockSimulation()
    setMetrics(prev => ({
      ...prev,
      state: prev.state === 'running' ? 'idle' : prev.state,
      phase: prev.state === 'running' ? 'complete' : prev.phase,
    }))
    try {
      await api.stopRealSpeedTest()
    } catch {
      // Ignore
    }
  }, [stopMockSimulation])

  useEffect(() => {
    if (!isOpen) {
      stopMockSimulation()
      return
    }

    const unsubProg = api.onEvent('speedtest:progress', (p: SpeedTestProgress) => {
      const activeMbps = p.phase === 'upload' ? p.upload_mbps : p.download_mbps
      setMetrics(prev => ({
        ...prev,
        ...p,
        state: p.phase === 'complete' ? 'completed' : p.phase === 'error' ? 'error' : 'running',
        gaugeAngle: calculateGaugeAngle(activeMbps),
      }))
    })

    const unsubDone = api.onEvent('speedtest:complete', (res: SpeedTestProgress) => {
      setMetrics(prev => ({
        ...prev,
        ...res,
        state: 'completed',
        phase: 'complete',
        progress: 100,
        gaugeAngle: calculateGaugeAngle(res.download_mbps),
      }))
    })

    const unsubErr = api.onEvent('speedtest:error', (err: string) => {
      setMetrics(prev => ({
        ...prev,
        state: 'error',
        phase: 'error',
        error_msg: err,
      }))
    })

    return () => {
      stopTest()
      if (typeof unsubProg === 'function') unsubProg()
      if (typeof unsubDone === 'function') unsubDone()
      if (typeof unsubErr === 'function') unsubErr()
    }
  }, [isOpen, stopTest, stopMockSimulation])

  return {
    metrics,
    isRunning: metrics.state === 'running',
    startTest,
    stopTest,
    reset: () => setMetrics(INITIAL_METRICS),
  }
}
