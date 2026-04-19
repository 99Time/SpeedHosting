import type {
  AdminOverviewResponse,
  AdminUserResponse,
  AcquisitionPayload,
  AuthResponse,
  DashboardResponse,
  LogoutResponse,
  MeResponse,
  PlanResponse,
  ServerConfigMod,
  ServerConfig,
  ServerResponse,
  ServersResponse,
  UpdatesResponse,
} from '../types/api'

const defaultHeaders = {
  'Content-Type': 'application/json',
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      ...defaultHeaders,
      ...(init?.headers ?? {}),
    },
  })

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null
    throw new ApiError(response.status, body?.error ?? 'Request failed')
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export function registerUser(payload: {
  displayName: string
  email: string
  password: string
  acquisition?: AcquisitionPayload
}) {
  return apiFetch<AuthResponse>('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function loginUser(payload: { email: string; password: string; acquisition?: AcquisitionPayload }) {
  return apiFetch<AuthResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function logoutUser() {
  return apiFetch<LogoutResponse>('/api/v1/auth/logout', {
    method: 'POST',
  })
}

export function getMe() {
  return apiFetch<MeResponse>('/api/v1/auth/me')
}

export function getDashboard() {
  return apiFetch<DashboardResponse>('/api/v1/dashboard')
}

export function getPlan() {
  return apiFetch<PlanResponse>('/api/v1/account/plan')
}

export function getUpdates() {
  return apiFetch<UpdatesResponse>('/api/updates')
}

export function getServers() {
  return apiFetch<ServersResponse>('/api/v1/servers')
}

export function createServer(payload: {
  name: string
  desiredTickRate: number
  maxPlayers: number
  password?: string
  adminSteamIds?: string[]
  mods?: ServerConfigMod[]
  acquisition?: AcquisitionPayload
}) {
  return apiFetch<ServerResponse>('/api/v1/servers', {
    method: 'POST',
    body: JSON.stringify({
      ...payload,
      password: payload.password ?? '',
      adminSteamIds: payload.adminSteamIds ?? [],
      mods: payload.mods ?? [],
    }),
  })
}

export function runServerAction(serverId: number, action: 'start' | 'stop' | 'restart') {
  return apiFetch<ServerResponse>(`/api/v1/servers/${serverId}/actions`, {
    method: 'POST',
    body: JSON.stringify({ action }),
  })
}

export function updateServerConfig(serverId: number, config: ServerConfig) {
  return apiFetch<ServerResponse>(`/api/v1/servers/${serverId}/config`, {
    method: 'PATCH',
    body: JSON.stringify({ config }),
  })
}

export function deleteServer(serverId: number) {
  return apiFetch<void>(`/api/v1/servers/${serverId}`, {
    method: 'DELETE',
  })
}

export function getAdminOverview() {
  return apiFetch<AdminOverviewResponse>('/api/v1/admin/overview')
}

export function updateUserPlan(userId: number, planCode: string) {
  return apiFetch<AdminUserResponse>(`/api/v1/admin/users/${userId}/plan`, {
    method: 'PATCH',
    body: JSON.stringify({ planCode }),
  })
}

export function runAdminServerAction(serverId: number, action: 'start' | 'stop' | 'restart') {
  return apiFetch<ServerResponse>(`/api/v1/admin/servers/${serverId}/actions`, {
    method: 'POST',
    body: JSON.stringify({ action }),
  })
}

export function updateAdminServerConfig(serverId: number, config: ServerConfig) {
  return apiFetch<ServerResponse>(`/api/v1/admin/servers/${serverId}/config`, {
    method: 'PATCH',
    body: JSON.stringify({ config }),
  })
}

export function deleteAdminServer(serverId: number) {
  return apiFetch<void>(`/api/v1/admin/servers/${serverId}`, {
    method: 'DELETE',
  })
}
