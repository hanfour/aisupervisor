import { writable } from 'svelte/store'

export const companyEvents = writable([])
const MAX_EVENTS = 200

export function initCompanyStore() {
  if (window.runtime) {
    window.runtime.EventsOn('company:event', (event) => {
      companyEvents.update(list => {
        const updated = [event, ...list]
        return updated.slice(0, MAX_EVENTS)
      })
    })
  }
}
