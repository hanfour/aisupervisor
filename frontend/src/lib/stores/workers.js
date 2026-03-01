import { writable } from 'svelte/store'

export const workers = writable([])

export async function loadWorkers() {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const result = await window.go.gui.CompanyApp.ListWorkers()
    workers.set(result || [])
  }
}

export async function createWorker(name, avatar) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const w = await window.go.gui.CompanyApp.CreateWorker(name, avatar)
    await loadWorkers()
    return w
  }
}
