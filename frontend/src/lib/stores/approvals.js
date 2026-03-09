import { writable } from 'svelte/store'

export const pendingGates = writable([])

export async function loadPendingGates() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetPendingGateRequests()
    pendingGates.set(result || [])
  } catch {
    // ignore
  }
}

export async function respondToGate(id, status) {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.RespondToGateRequest(id, status)
  await loadPendingGates()
}
