// Game clock — maps real time to simulated office day
// Pure frontend visual simulation; does not affect backend events

export const PHASES = {
  MORNING_ARRIVAL: 'morning_arrival',
  WORK_MORNING:    'work_morning',
  LUNCH:           'lunch',
  WORK_AFTERNOON:  'work_afternoon',
  TEA_BREAK:       'tea_break',
  WORK_LATE:       'work_late',
  OVERTIME:         'overtime',
  NIGHT:           'night',
}

// Phase boundaries in game-minutes from midnight
const PHASE_RANGES = [
  { phase: PHASES.MORNING_ARRIVAL, start: 540,  end: 570  }, // 09:00-09:30
  { phase: PHASES.WORK_MORNING,    start: 570,  end: 720  }, // 09:30-12:00
  { phase: PHASES.LUNCH,           start: 720,  end: 780  }, // 12:00-13:00
  { phase: PHASES.WORK_AFTERNOON,  start: 780,  end: 900  }, // 13:00-15:00
  { phase: PHASES.TEA_BREAK,       start: 900,  end: 915  }, // 15:00-15:15
  { phase: PHASES.WORK_LATE,       start: 915,  end: 1080 }, // 15:15-18:00
  { phase: PHASES.OVERTIME,        start: 1080, end: 1260 }, // 18:00-21:00
  { phase: PHASES.NIGHT,           start: 1260, end: 1440 }, // 21:00-24:00
]

const DAY_START_MINUTES = 530 // 08:50 — workers start arriving before 09:00
const DAY_LENGTH_MINUTES = 1440 // 24 hours

// How many game-minutes pass per real-millisecond at 1x speed
// At 1x: 1 real second = 1 game minute → full day in 24 real minutes
const GAME_MINS_PER_REAL_MS = 1 / 1000

export class GameClock {
  constructor() {
    this.gameMinutes = DAY_START_MINUTES
    this.speed = 1
    this._listeners = []
    this._prevPhase = null
    this._dayCount = 0
  }

  update(realDeltaMs) {
    const prevMinutes = this.gameMinutes
    this.gameMinutes += GAME_MINS_PER_REAL_MS * realDeltaMs * this.speed

    // Day rollover
    if (this.gameMinutes >= DAY_LENGTH_MINUTES) {
      this.gameMinutes = DAY_START_MINUTES
      this._dayCount++
      this._emit('day_start', { day: this._dayCount })
    }

    // Phase change detection
    const currentPhase = this.getCurrentPhase()
    if (currentPhase !== this._prevPhase) {
      this._emit('phase_change', { phase: currentPhase, prevPhase: this._prevPhase })
      this._prevPhase = currentPhase
    }
  }

  setSpeed(speed) {
    this.speed = Math.max(1, Math.min(8, speed))
  }

  getTimeString() {
    const h = Math.floor(this.gameMinutes / 60) % 24
    const m = Math.floor(this.gameMinutes % 60)
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
  }

  getGameMinutes() {
    return this.gameMinutes
  }

  getDayCount() {
    return this._dayCount
  }

  getCurrentPhase() {
    const mins = this.gameMinutes % DAY_LENGTH_MINUTES
    for (const { phase, start, end } of PHASE_RANGES) {
      if (mins >= start && mins < end) return phase
    }
    // Before 09:00 or after midnight edge cases
    if (mins < 540) return PHASES.MORNING_ARRIVAL
    return PHASES.NIGHT
  }

  onEvent(fn) {
    this._listeners.push(fn)
    return () => {
      this._listeners = this._listeners.filter(l => l !== fn)
    }
  }

  _emit(type, data) {
    for (const fn of this._listeners) {
      try { fn({ type, ...data }) } catch (e) { /* ignore */ }
    }
  }
}
