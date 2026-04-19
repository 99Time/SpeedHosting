import type { AcquisitionPayload, User } from '../types/api'

const attributionStorageKey = 'speedhosting.puckAttribution'
const sessionStorageKey = 'speedhosting.analyticsSessionId'

type StoredAttribution = {
  version: 1
  firstSource: string
  latestSource: string
  firstTimestamp: string
  latestTimestamp: string
  landingPath: string
  fullUrl: string
  sessionId: string
}

const memoryStore = new Map<string, string>()

function readStorage(storageType: 'localStorage' | 'sessionStorage', key: string) {
  if (typeof window === 'undefined') {
    return memoryStore.get(`${storageType}:${key}`) ?? null
  }

  try {
    return window[storageType].getItem(key)
  } catch {
    return memoryStore.get(`${storageType}:${key}`) ?? null
  }
}

function writeStorage(storageType: 'localStorage' | 'sessionStorage', key: string, value: string) {
  if (typeof window === 'undefined') {
    memoryStore.set(`${storageType}:${key}`, value)
    return
  }

  try {
    window[storageType].setItem(key, value)
  } catch {
    memoryStore.set(`${storageType}:${key}`, value)
  }
}

function createSessionId() {
  return `sh_${Math.random().toString(36).slice(2, 10)}${Date.now().toString(36)}`
}

export function getAnalyticsSessionId() {
  const existing = readStorage('sessionStorage', sessionStorageKey)
  if (existing) {
    return existing
  }

  const next = createSessionId()
  writeStorage('sessionStorage', sessionStorageKey, next)
  return next
}

export function getStoredAttribution() {
  const raw = readStorage('localStorage', attributionStorageKey)
  if (!raw) {
    return null
  }

  try {
    return JSON.parse(raw) as StoredAttribution
  } catch {
    return null
  }
}

export function capturePuckAttribution(source: string, landingPath: string, fullUrl: string, explicitSource: boolean) {
  const current = getStoredAttribution()
  const now = new Date().toISOString()
  const sessionId = current?.sessionId ?? getAnalyticsSessionId()

  if (!explicitSource && current) {
    return current
  }

  const next: StoredAttribution = {
    version: 1,
    firstSource: current?.firstSource ?? source,
    latestSource: source,
    firstTimestamp: current?.firstTimestamp ?? now,
    latestTimestamp: now,
    landingPath,
    fullUrl,
    sessionId,
  }

  writeStorage('localStorage', attributionStorageKey, JSON.stringify(next))
  return next
}

export function buildAcquisitionPayload(user?: User | null, sourceOverride?: string): AcquisitionPayload {
  const stored = getStoredAttribution()
  const route = typeof window === 'undefined' ? '/' : `${window.location.pathname}${window.location.search}`

  return {
    source: sourceOverride ?? user?.latestAcquisitionSource ?? stored?.latestSource ?? 'direct',
    timestamp: stored?.latestTimestamp ?? new Date().toISOString(),
    landingPath: stored?.landingPath ?? route,
    fullUrl: stored?.fullUrl ?? (typeof window === 'undefined' ? route : window.location.href),
    sessionId: stored?.sessionId ?? getAnalyticsSessionId(),
    route,
  }
}

export function getPreferredAttributionSource(user?: User | null) {
  const stored = getStoredAttribution()
  return user?.latestAcquisitionSource ?? stored?.latestSource ?? 'direct'
}