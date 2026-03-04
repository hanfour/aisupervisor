// Animation state machine for pixel office characters

const ANIM_CONFIG = {
  idle:     { frameCount: 2, interval: 500, loop: true },
  working:  { frameCount: 3, interval: 250, loop: true },
  waiting:  { frameCount: 2, interval: 700, loop: true },
  error:    { frameCount: 1, interval: 400, loop: false, playCount: 2, fallback: 'idle' },
  finished: { frameCount: 3, interval: 300, loop: false, playCount: 2, fallback: 'idle' },
  walkDown:  { frameCount: 3, interval: 180, loop: true },
  walkUp:    { frameCount: 3, interval: 180, loop: true },
  walkLeft:  { frameCount: 3, interval: 180, loop: true },
  walkRight: { frameCount: 3, interval: 180, loop: true },
}

// Map worker.status to animation state
const STATUS_TO_ANIM = {
  'idle': 'idle',
  'working': 'working',
  'busy': 'working',
  'waiting': 'waiting',
  'queued': 'waiting',
  'error': 'error',
  'failed': 'error',
  'finished': 'finished',
  'done': 'finished',
  'completed': 'finished',
}

// Mood-based animation interval modifiers (applied to idle/working/waiting states)
const MOOD_ANIM_MODIFIERS = {
  stressed:   { idle: 250, working: 200, waiting: 400 },   // fidgety
  tired:      { idle: 900, working: 400, waiting: 1000 },  // sluggish
  excited:    { idle: 300, working: 200, waiting: 450 },   // bouncy
  frustrated: { idle: 650, working: 300, waiting: 800 },   // heavy
  happy:      { idle: 450, working: 230, waiting: 650 },   // slightly upbeat
}

export function getAnimInterval(state, mood) {
  const config = ANIM_CONFIG[state]
  if (!config) return 500
  const moodMod = mood && MOOD_ANIM_MODIFIERS[mood]
  if (moodMod && moodMod[state] !== undefined) return moodMod[state]
  return config.interval
}

export function statusToAnim(workerStatus) {
  return STATUS_TO_ANIM[workerStatus] || 'idle'
}

export class AnimationState {
  constructor() {
    this.state = 'idle'
    this.frame = 0
    this.elapsed = 0
    this.playCount = 0
  }

  setState(newState) {
    if (newState === this.state) return
    this.state = newState
    this.frame = 0
    this.elapsed = 0
    this.playCount = 0
  }

  update(deltaMs, mood) {
    const config = ANIM_CONFIG[this.state]
    if (!config) return

    const interval = getAnimInterval(this.state, mood)
    this.elapsed += deltaMs
    if (this.elapsed >= interval) {
      this.elapsed -= interval
      this.frame++

      if (this.frame >= config.frameCount) {
        if (config.loop) {
          this.frame = 0
        } else {
          this.playCount++
          if (this.playCount >= (config.playCount || 1)) {
            // Transition to fallback
            this.state = config.fallback || 'idle'
            this.frame = 0
            this.elapsed = 0
            this.playCount = 0
          } else {
            this.frame = 0
          }
        }
      }
    }
  }

  getFrame() {
    const config = ANIM_CONFIG[this.state]
    if (!config) return 0
    return Math.min(this.frame, config.frameCount - 1)
  }
}

export const ENV_ANIM = {
  neonPulseSpeed: 500,
  particleSpawnRate: 0.02,
  particleMaxCount: 30,
  dataStreamSpeed: 2000,
  screenGlowPulse: 3000,
}

export { ANIM_CONFIG }
