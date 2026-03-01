import { writable } from 'svelte/store'

export const events = writable([])
const MAX_EVENTS = 200

export function initEventStore() {
  if (window.runtime) {
    window.runtime.EventsOn('supervisor:event', (event) => {
      events.update(list => {
        const updated = [event, ...list]
        return updated.slice(0, MAX_EVENTS)
      })
    })
  }
}
