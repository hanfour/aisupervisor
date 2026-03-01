import { writable, derived } from 'svelte/store'

// All discussion events, keyed by discussionId
export const discussionEvents = writable({})

export function initDiscussionStore() {
  if (window.runtime) {
    window.runtime.EventsOn('discussion:event', (event) => {
      discussionEvents.update(map => {
        const id = event.discussionId
        const existing = map[id] || []
        return { ...map, [id]: [...existing, event] }
      })
    })
  }
}

// Active discussions list (grouped)
export const activeDiscussions = derived(discussionEvents, ($events) => {
  return Object.entries($events).map(([id, evts]) => ({
    id,
    groupId: evts[0]?.groupId || '',
    sessionId: evts[0]?.sessionId || '',
    events: evts,
    latestPhase: evts[evts.length - 1]?.phase || 'opinion',
  }))
})
