import { writable } from 'svelte/store'

export const companyEvents = writable([])
export const companyStats = writable({ projects: 0, inProgress: 0, idleWorkers: 0 })
const MAX_EVENTS = 200

export function initCompanyStore() {
  if (window.runtime) {
    window.runtime.EventsOn('company:event', (event) => {
      companyEvents.update(list => {
        const updated = [event, ...list]
        return updated.slice(0, MAX_EVENTS)
      })
      // Auto-refresh stats on any company event
      loadCompanyStats()
    })
  }
}

export async function loadCompanyStats() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const projects = await window.go.gui.CompanyApp.ListProjects()
    const workers = await window.go.gui.CompanyApp.ListWorkers()
    const inProgress = (projects || []).filter(p => p.status === 'active').length
    const idleWorkers = (workers || []).filter(w => w.status === 'idle').length
    companyStats.set({
      projects: (projects || []).length,
      inProgress,
      idleWorkers,
    })
  } catch {
    // ignore
  }
}
