// Animation state machine for pixel office characters

const ANIM_CONFIG = {
  idle:     { frameCount: 2, interval: 500, loop: true },
  working:  { frameCount: 3, interval: 250, loop: true },
  waiting:  { frameCount: 2, interval: 700, loop: true },
  error:    { frameCount: 1, interval: 400, loop: false, playCount: 2, fallback: 'idle' },
  finished: { frameCount: 3, interval: 300, loop: false, playCount: 2, fallback: 'idle' },
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

  update(deltaMs) {
    const config = ANIM_CONFIG[this.state]
    if (!config) return

    this.elapsed += deltaMs
    if (this.elapsed >= config.interval) {
      this.elapsed -= config.interval
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

export { ANIM_CONFIG }
