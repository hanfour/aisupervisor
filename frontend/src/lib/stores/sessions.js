import { writable } from 'svelte/store'

export const sessions = writable([])

export async function loadSessions() {
  if (window.go && window.go.gui && window.go.gui.App) {
    const result = await window.go.gui.App.GetSessions()
    sessions.set(result || [])
  }
}
