type StatusBadgeProps = {
  status: string
}

function badgeLabel(status: string) {
  const normalized = status.toLowerCase()

  switch (normalized) {
    case 'running':
      return 'Running'
    case 'stopped':
      return 'Stopped'
    case 'starting':
    case 'activating':
      return 'Starting'
    case 'restarting':
      return 'Restarting'
    case 'error':
      return 'Error'
    default:
      return status.charAt(0).toUpperCase() + status.slice(1)
  }
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const normalized = status.toLowerCase()

  return (
    <span className={`status-badge status-badge--${normalized}`}>
      <span className="status-badge__dot" aria-hidden="true" />
      <span>{badgeLabel(status)}</span>
    </span>
  )
}
