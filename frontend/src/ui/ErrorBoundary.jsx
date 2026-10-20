// Catches render errors in the subtree and shows a localized fallback instead of a white screen.
// `resetKey` (e.g. the route path) clears the error on navigation. Errors are logged with the route.
import { Component } from 'react'
import { useTranslation } from '../i18n'
import ErrorState from './ErrorState'

function Fallback({ onReset }) {
  const { t } = useTranslation()
  return (
    <ErrorState
      title={t('errors.boundary_title')}
      description={t('errors.boundary_desc')}
      onRetry={onReset}
      className="py-20"
    />
  )
}

export default class ErrorBoundary extends Component {
  state = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch(error, info) {
    console.error('[ErrorBoundary]', window.location.pathname, error, info?.componentStack)
  }

  componentDidUpdate(prev) {
    if (this.state.hasError && prev.resetKey !== this.props.resetKey) this.reset()
  }

  reset = () => this.setState({ hasError: false })

  render() {
    if (this.state.hasError) return <Fallback onReset={this.reset} />
    return this.props.children
  }
}

