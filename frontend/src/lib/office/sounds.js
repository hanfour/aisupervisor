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
