import { Component, ErrorInfo, ReactNode } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { t } from '../lib/i18n'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  }

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Uncaught React Error in NexVPN UI:', error, errorInfo)
  }

  private handleReload = () => {
    window.location.reload()
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen w-full bg-card text-foreground flex flex-col items-center justify-center p-6 text-center select-none">
          <div className="max-w-md w-full bg-card/90 border border-destructive/30 rounded-3xl p-6 shadow-2xl backdrop-blur-xl">
            <div className="w-14 h-14 rounded-2xl bg-destructive/20 border border-destructive/30 flex items-center justify-center mx-auto mb-4 text-destructive">
              <AlertTriangle size={28} />
            </div>

            <h2 className="text-base font-bold text-foreground mb-2">
              {t('error_boundary_title')}
            </h2>

            <p className="text-xs text-muted-foreground mb-4 leading-relaxed font-sans">
              {t('error_boundary_desc')}
            </p>

            {this.state.error && (
              <div className="bg-card/80 p-3 rounded-xl border border-border text-[11px] font-mono text-destructive/90 text-start mb-4 overflow-x-auto max-h-32" dir="ltr">
                {this.state.error.toString()}
              </div>
            )}

            <button
              onClick={this.handleReload}
              className="bg-primary w-full py-2.5 rounded-xl text-primary-foreground text-xs font-bold shadow-sm flex items-center justify-center gap-2 transition"
            >
              <RefreshCw size={14} />
              <span>{t('error_boundary_reload')}</span>
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
