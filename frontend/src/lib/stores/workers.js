import { writable } from 'svelte/store'

export const workers = writable([])
export const hierarchy = writable({ consultant: [], manager: [], engineer: [] })

export async function loadWorkers() {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const result = await window.go.gui.CompanyApp.ListWorkers()
    workers.set(result || [])
  }
}

export async function loadHierarchy() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetHierarchy()
    hierarchy.set(result || { consultant: [], manager: [], engineer: [] })
  } catch {
    // ignore
  }
}

export async function createWorker(name, avatar) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const w = await window.go.gui.CompanyApp.CreateWorker(name, avatar)
    await loadWorkers()
    return w
  }
}

export async function createWorkerWithTier(name, avatar, tier, parentID, backendID, cliTool) {
  if (!window.go?.gui?.CompanyApp) return
  const w = await window.go.gui.CompanyApp.CreateWorkerWithTier(name, avatar, tier, parentID, backendID, cliTool)
  await loadWorkers()
  await loadHierarchy()
  return w
}

export async function promoteWorker(workerID, newTier) {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.PromoteWorker(workerID, newTier)
  await loadWorkers()
  await loadHierarchy()
}
