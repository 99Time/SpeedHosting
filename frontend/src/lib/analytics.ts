import type { AcquisitionPayload } from '../types/api'

import { buildAcquisitionPayload, getAnalyticsSessionId } from './attribution'

type TrackEventOptions = {
  source?: string
  route?: string
  metadata?: Record<string, unknown>
  acquisition?: AcquisitionPayload
}

export async function trackEvent(name: string, options: TrackEventOptions = {}) {
  if (typeof window === 'undefined') {
    return
  }

  const acquisition = options.acquisition ?? buildAcquisitionPayload(undefined, options.source)
  const route = options.route ?? `${window.location.pathname}${window.location.search}`

  try {
    await fetch('/api/v1/analytics/events', {
      method: 'POST',
      credentials: 'include',
      keepalive: true,
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name,
        source: options.source ?? acquisition.source,
        route,
        landingPath: acquisition.landingPath,
        fullUrl: acquisition.fullUrl,
        sessionId: acquisition.sessionId ?? getAnalyticsSessionId(),
        timestamp: new Date().toISOString(),
        acquisition,
        metadata: options.metadata ?? {},
      }),
    })
  } catch {
    return
  }
}