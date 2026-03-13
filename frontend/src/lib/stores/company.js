import { writable } from 'svelte/store'
import { loadSessions } from './sessions.js'

export const companyEvents = writable([])
export const companyStats = writable({ projects: 0, inProgress: 0, idleWorkers: 0, reviewsPending: 0, trainingPairs: 0 })
export const reviewQueue = writable([])
export const trainingStats = writable({ totalPairs: 0, accepted: 0, rejected: 0, approvalRate: 0 })
export const dashboardAlerts = writable({ stuckWorkers: 0, escalatedTasks: 0, pendingApprovals: 0 })
export const budgetSummary = writable({ currentMonth: '', tokenBudget: 0, tokensUsed: 0, taskCount: 0, usagePercent: 0 })
export const objectivesList = writable([])
const MAX_EVENTS = 200

export function initCompanyStore() {
  if (window.runtime) {
    window.runtime.EventsOn('company:event', (event) => {
      companyEvents.update(list => {
        const updated = [event, ...list]
        return updated.slice(0, MAX_EVENTS)
      })
      // Auto-refresh all data on any company event
      loadCompanyStats()
      loadReviewQueue()
      loadTrainingStats()
      loadDashboardAlerts()
      loadSessions()
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

    let reviewsPending = 0
    let trainingPairs = 0
    try {
      const rq = await window.go.gui.CompanyApp.GetReviewQueue()
      reviewsPending = (rq || []).length
    } catch {}
    try {
      const ts = await window.go.gui.CompanyApp.GetTrainingStats()
      trainingPairs = ts?.totalPairs || 0
    } catch {}

    companyStats.set({
      projects: (projects || []).length,
      inProgress,
      idleWorkers,
      reviewsPending,
      trainingPairs,
    })
  } catch {
    // ignore
  }
}

export async function loadReviewQueue() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetReviewQueue()
    reviewQueue.set(result || [])
  } catch {
    // ignore
  }
}

export async function loadTrainingStats() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetTrainingStats()
    trainingStats.set(result || { totalPairs: 0, accepted: 0, rejected: 0, approvalRate: 0 })
  } catch {
    // ignore
  }
}

export async function loadDashboardAlerts() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetDashboardAlerts()
    dashboardAlerts.set(result || { stuckWorkers: 0, escalatedTasks: 0, pendingApprovals: 0 })
  } catch {
    // ignore
  }
}

export async function loadBudgetSummary() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.GetBudgetSummary()
    budgetSummary.set(result || { currentMonth: '', tokenBudget: 0, tokensUsed: 0, taskCount: 0, usagePercent: 0 })
  } catch {
    // ignore
  }
}

export async function loadObjectivesList() {
  if (!window.go?.gui?.CompanyApp) return
  try {
    const result = await window.go.gui.CompanyApp.ListObjectives()
    objectivesList.set(result || [])
  } catch {
    // ignore
  }
}

export async function drainReviewQueue() {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.DrainReviewQueue()
  await loadReviewQueue()
}

export async function resetWorker(workerID) {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.ResetWorker(workerID)
  await loadCompanyStats()
}
