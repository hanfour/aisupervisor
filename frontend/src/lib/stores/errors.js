import { writable } from 'svelte/store'

export const errors = writable([])

let nextId = 0

export function addError(message, duration = 5000) {
  const id = ++nextId
  errors.update(list => [...list, { id, message }])
  if (duration > 0) {
    setTimeout(() => dismissError(id), duration)
  }
}

export function dismissError(id) {
  errors.update(list => list.filter(e => e.id !== id))
}
