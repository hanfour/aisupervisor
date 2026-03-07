import { writable } from 'svelte/store'

export const simulationSpeed = writable(1.0)
export const simulationPaused = writable(false)
export const activityLog = writable([])
export const simulationEngine = writable(null)

// Game clock stores
export const gameTimeString = writable('08:50')
export const currentPhase = writable('morning_arrival')
export const gameClockSpeed = writable(1)
export const gameDayCount = writable(0)
