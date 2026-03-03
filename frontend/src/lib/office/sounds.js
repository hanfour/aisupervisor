// 8-bit sound effects via Web Audio API

let audioCtx = null
let enabled = true

function getCtx() {
  if (!audioCtx) {
    try { audioCtx = new (window.AudioContext || window.webkitAudioContext)() }
    catch { return null }
  }
  return audioCtx
}

function playTone(freq, duration, type = 'square', startTime = 0) {
  const ctx = getCtx()
  if (!ctx || !enabled) return
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()
  osc.type = type
  osc.frequency.value = freq
  gain.gain.value = 0.08
  gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + startTime + duration)
  osc.connect(gain)
  gain.connect(ctx.destination)
  osc.start(ctx.currentTime + startTime)
  osc.stop(ctx.currentTime + startTime + duration)
}

export function playFinished() {
  // Ascending arpeggio C-E-G-C
  playTone(523, 0.12, 'square', 0)
  playTone(659, 0.12, 'square', 0.1)
  playTone(784, 0.12, 'square', 0.2)
  playTone(1047, 0.2, 'square', 0.3)
}

export function playError() {
  // Descending tones
  playTone(440, 0.15, 'square', 0)
  playTone(330, 0.15, 'square', 0.12)
  playTone(220, 0.3, 'square', 0.24)
}

export function playAssign() {
  // Short beep
  playTone(880, 0.08, 'square', 0)
  playTone(1100, 0.06, 'square', 0.08)
}

export function setSoundEnabled(val) {
  enabled = val
}

export function isSoundEnabled() {
  return enabled
}

// ── Ambient & interaction sounds (used by officeRenderer) ─────────────────

let ambientInterval = null

export function startAmbient() {
  if (ambientInterval) return
  ambientInterval = setInterval(() => {
    if (!enabled) return
    // Subtle low hum tick
    playTone(110, 0.04, 'sine', 0)
  }, 4000)
}

export function stopAmbient() {
  if (ambientInterval) {
    clearInterval(ambientInterval)
    ambientInterval = null
  }
}

let lastKeyClatter = 0

export function playKeyClatter() {
  const now = Date.now()
  if (now - lastKeyClatter < 300) return
  lastKeyClatter = now
  playTone(800 + Math.random() * 400, 0.03, 'square', 0)
  playTone(600 + Math.random() * 300, 0.03, 'square', 0.03)
}

let lastFootstep = 0

export function playFootstep() {
  const now = Date.now()
  if (now - lastFootstep < 300) return
  lastFootstep = now
  playTone(150 + Math.random() * 50, 0.05, 'triangle', 0)
}

export function playDiscussionStart() {
  playTone(660, 0.06, 'square', 0)
  playTone(880, 0.06, 'square', 0.06)
}

export function playMeetingBell() {
  playTone(1047, 0.15, 'sine', 0)
  playTone(1319, 0.15, 'sine', 0.12)
  playTone(1568, 0.2, 'sine', 0.24)
}

export function playWatercooler() {
  playTone(300, 0.08, 'sine', 0)
  playTone(350, 0.08, 'sine', 0.06)
  playTone(250, 0.1, 'sine', 0.12)
}

export function playApproval() {
  playTone(523, 0.1, 'square', 0)
  playTone(784, 0.15, 'square', 0.08)
}

export function playTypewriter() {
  const freq = 500 + Math.random() * 300
  playTone(freq, 0.02, 'square', 0)
}

export function playCelebration() {
  // Happy ascending fanfare
  playTone(523, 0.1, 'square', 0)
  playTone(659, 0.1, 'square', 0.1)
  playTone(784, 0.1, 'square', 0.2)
  playTone(1047, 0.2, 'square', 0.3)
}

export function playComfort() {
  // Gentle low tones
  playTone(330, 0.2, 'sine', 0)
  playTone(392, 0.2, 'sine', 0.2)
}

export function playPairProgramming() {
  // Two quick beeps
  playTone(660, 0.08, 'square', 0)
  playTone(880, 0.08, 'square', 0.12)
}
