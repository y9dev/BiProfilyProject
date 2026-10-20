// Renders the data states for a useAsync() result:
//   loading -> skeleton (or `skeleton` prop) | error -> ErrorState+retry | empty -> EmptyState | ready -> children
import EmptyState from './EmptyState'
import ErrorState from './ErrorState'
import Skeleton from './Skeleton'

export default function Async({ status, onRetry, isEmpty = false, empty, skeleton, children }) {
  if (status === 'loading') {
    return (
      skeleton || (
        <div className="space-y-3" aria-busy="true">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      )
    )
  }
  if (status === 'error') return <ErrorState onRetry={onRetry} />
  if (isEmpty) return <EmptyState {...empty} />
  return children
}
