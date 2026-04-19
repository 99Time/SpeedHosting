import type { ServerConfigMod } from '../types/api'

export const SPEEDRANKEDS_WORKSHOP_ID = '3691658485'

function normalizeMods(mods: ServerConfigMod[]) {
  return mods.filter((mod) => mod.workshopId.trim().length > 0)
}

export function hasWorkshopMod(mods: ServerConfigMod[], workshopId: string) {
  return normalizeMods(mods).some((mod) => mod.workshopId === workshopId && mod.enabled)
}

export function toggleWorkshopMod(mods: ServerConfigMod[], workshopId: string, enabled: boolean) {
  const nextMods = normalizeMods(mods).filter((mod) => mod.workshopId !== workshopId)

  if (!enabled) {
    return nextMods
  }

  return [
    ...nextMods,
    {
      workshopId,
      enabled: true,
      clientRequired: false,
    },
  ]
}