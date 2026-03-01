import { writable } from 'svelte/store'

export const roles = writable([])

export async function loadRoles() {
  if (window.go && window.go.gui && window.go.gui.App) {
    const result = await window.go.gui.App.GetRoles()
    roles.set(result || [])
  }
}
