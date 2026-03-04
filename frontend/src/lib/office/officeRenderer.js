// Canvas 2D rendering engine for Pixel Office — Hacker Base Edition

import { TILE_SIZE, SCALE, COLS, ROWS, CANVAS_W, CANVAS_H, getFloorMap, getDesks, buildWorkerDeskMap } from './layout.js'
import { prerenderCharacter, prerenderFurniture, prerenderEnvSprite, getCharacterType } from './sprites.js'
import { AnimationState, statusToAnim, ENV_ANIM } from './animation.js'
import { startAmbient, stopAmbient, playKeyClatter } from './sounds.js'
import { MovementController } from './movement.js'
import { BubbleManager } from './bubbles.js'

const TILE_PX = TILE_SIZE * SCALE  // 48

// ── Dark hacker-base floor/wall palette ─────────────────────────────────────
const FLOOR_COLORS = {
  0: '#1a1a2e',  // deep indigo floor
  1: '#0f0f1a',  // ultra-dark wall
  2: '#1a1a2e',  // desk (drawn as furniture)
  3: '#1a1a2e',  // plant/serverRack (drawn as furniture)
  4: '#1a1a2e',  // computer/holoDisplay (drawn as furniture)
  5: '#1a1a2e',  // watercooler/vendingMachine (drawn as furniture)
  6: '#1a1a2e',  // bookshelf/wallOfScreens (drawn as furniture)
  7: '#222233',  // door
  8: '#1a1a2e',  // glowStrip (drawn as env sprite)
  9: '#1a1a2e',  // cableFloor (drawn as env sprite)
  10: '#141430', // meetingFloor — deep indigo-purple, distinct from regular floor
  11: '#e8e8f0', // whiteboard — off-white face color
}

const TILE_TO_FURNITURE = {
  2: 'desk',
  3: 'plant',
  4: 'computer',
  5: 'watercooler',
  6: 'bookshelf',
  10: 'meetingTable',
  11: 'whiteboard',
}

const TILE_TO_ENV = {
  8: 'glowStrip',
  9: 'cableFloor',
}

const BUBBLE_MAP = {
  waiting: { text: '?', bg: '#ffdd57', fg: '#333' },
  working: { text: '...', bg: '#48f', fg: '#fff' },
  error:   { text: '!', bg: '#ff3860', fg: '#fff' },
  finished:{ text: '\u2605', bg: '#00ff41', fg: '#000' },
}

// ── Floating binary particle ────────────────────────────────────────────────
class BinaryParticle {
  constructor(x, y) {
    this.x = x + (Math.random() - 0.5) * TILE_PX
    this.y = y
    this.char = Math.random() > 0.5 ? '1' : '0'
    this.life = 1.0
    this.speed = 0.3 + Math.random() * 0.5
    this.drift = (Math.random() - 0.5) * 0.3
  }
  update(delta) {
    this.y -= this.speed * (delta / 16)
    this.x += this.drift * (delta / 16)
    this.life -= delta / 3000
  }
  get dead() { return this.life <= 0 }
}

export class OfficeRenderer {
  constructor(canvas) {
    this.canvas = canvas
    this.ctx = canvas.getContext('2d')
    canvas.width = CANVAS_W
    canvas.height = CANVAS_H
    this.floorMap = getFloorMap()
    this.desks = getDesks()

    // Static background layer (drawn once)
    this.bgCanvas = document.createElement('canvas')
    this.bgCanvas.width = CANVAS_W
    this.bgCanvas.height = CANVAS_H

    // Prerender furniture
    this.furnitureCache = {}
    for (const name of ['desk', 'computer', 'plant', 'watercooler', 'bookshelf', 'meetingTable', 'whiteboard']) {
      this.furnitureCache[name] = prerenderFurniture(name)
    }

    // Prerender env sprites
    this.envCache = {}
    for (const name of ['glowStrip', 'cableFloor']) {
      this.envCache[name] = prerenderEnvSprite(name)
    }

    // Character caches: charType → { state: [canvases] }
    this.charCache = {}

    // Movement and bubble subsystems
    this.movement = new MovementController()
    this.bubbles = new BubbleManager()
    this.simulation = null  // set via setSimulation()

    // Runtime state
    this.workers = []
    this.workerDeskMap = {}
    this.animStates = {}
    this.hoveredWorkerId = null

    // Screen tile positions (for glow effect)
    this.screenTiles = []

    // Floating particles
    this.particles = []

    // Data stream animation phase
    this.dataStreamPhase = 0

    // Global time for pulsing effects
    this.globalTime = 0

    this.lastTime = 0
    this.running = false

    // Pre-rendered scanline overlay (static, drawn once)
    this.scanlineCanvas = document.createElement('canvas')
    this.scanlineCanvas.width = CANVAS_W
    this.scanlineCanvas.height = CANVAS_H

    this._drawBackground()
    this._findScreenTiles()
    this._prerenderScanlines()
  }

  _findScreenTiles() {
    this.screenTiles = []
    for (let row = 0; row < ROWS; row++) {
      for (let col = 0; col < COLS; col++) {
        const tile = this.floorMap[row][col]
        if (tile === 4 || tile === 6) {  // computer/holoDisplay or bookshelf/wallOfScreens
          this.screenTiles.push({ x: col * TILE_PX, y: row * TILE_PX })
        }
      }
    }
  }

  _drawBackground() {
    const ctx = this.bgCanvas.getContext('2d')

    for (let row = 0; row < ROWS; row++) {
      for (let col = 0; col < COLS; col++) {
        const tile = this.floorMap[row][col]
        ctx.fillStyle = FLOOR_COLORS[tile] || FLOOR_COLORS[0]
        ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)

        // Subtle cyan grid lines for floor tiles
        if (tile === 0 || tile === 7 || tile === 8 || tile === 9 ||
            (tile >= 2 && tile <= 6) || tile === 10 || tile === 11) {
          ctx.strokeStyle = 'rgba(0,221,255,0.04)'
          ctx.strokeRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Wall: horizontal metal highlight lines
        if (tile === 1) {
          ctx.fillStyle = '#1a1a30'
          ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, 2)
          ctx.fillStyle = '#181828'
          ctx.fillRect(col * TILE_PX, row * TILE_PX + TILE_PX / 2, TILE_PX, 1)
        }

        // Door: neon accent strip
        if (tile === 7) {
          ctx.fillStyle = '#2a2a44'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, TILE_PX)
          ctx.fillStyle = '#00ff41'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, 2)
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX + TILE_PX - 2, TILE_PX - 8, 2)
        }

        // Draw env sprites
        const envName = TILE_TO_ENV[tile]
        if (envName && this.envCache[envName]) {
          ctx.imageSmoothingEnabled = false
          ctx.drawImage(this.envCache[envName], col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Draw furniture sprites on tiles
        const furnitureName = TILE_TO_FURNITURE[tile]
        if (furnitureName && this.furnitureCache[furnitureName]) {
          ctx.imageSmoothingEnabled = false
          ctx.drawImage(this.furnitureCache[furnitureName], col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }
      }
    }

    // Zone labels
    ctx.font = '10px "Press Start 2P", monospace'
    ctx.fillStyle = 'rgba(0,255,65,0.12)'
    ctx.fillText('OPEN OFFICE', 2 * TILE_PX, 5.5 * TILE_PX)
    ctx.fillText('MGR', 1 * TILE_PX, 11.5 * TILE_PX)
    ctx.fillText('MEETING', 11 * TILE_PX, 10.5 * TILE_PX)
    ctx.fillText('BREAK', 17 * TILE_PX, 11.5 * TILE_PX)
    ctx.fillText('REC', 12 * TILE_PX, 13 * TILE_PX)
  }

  // ── Profile data (mood indicators) ───────────────────────────────────────
  setProfiles(profileMap) {
    this.profiles = profileMap // Map<workerId, CharacterProfileDTO>
  }

  // ── SimulationEngine integration ──────────────────────────────────────────
  setSimulation(sim) { this.simulation = sim }

  // Movement facade (called by SimulationEngine)
  isWorkerMoving(id)              { return this.movement.isMoving(id) }
  moveWorkerTo(id, col, row)      { this.movement.startMovement(id, col, row) }
  moveWorkerToWorker(id, tid)     { this.movement.startMovementToWorker(id, tid) }
  returnWorkerToDesk(id)          { this.movement.returnToDesk(id) }

  // Bubble facade (called by SimulationEngine)
  showSpeech(id, text, dur)             { return this.bubbles.showSpeech(id, text, dur) }
  showThought(id, text, dur)            { return this.bubbles.showThought(id, text, dur) }
  showDiscussion(id, tid, topic, dur)   { return this.bubbles.showDiscussion(id, tid, topic, dur) }
  showMeeting(ids, topic, dur)          { return this.bubbles.showMeeting(ids, topic, dur) }
  clearBubble(bubbleId)                 { this.bubbles.clear(bubbleId) }
  clearWorkerBubbles(id)                { this.bubbles.clearWorker(id) }

  setWorkers(workers, assignments) {
    this.workers = workers || []
    this.workerDeskMap = buildWorkerDeskMap(assignments)

    for (const w of this.workers) {
      if (!this.animStates[w.id]) {
        this.animStates[w.id] = new AnimationState()
      }
      // Only update status-based animation when worker is stationary
      if (!this.movement.isMoving(w.id)) {
        const anim = statusToAnim(w.status)
        this.animStates[w.id].setState(anim)
      }

      const charType = getCharacterType(w)
      // Retry prerender if previous attempt returned null (sprites weren't ready)
      if (!this.charCache[charType]) {
        const result = prerenderCharacter(charType)
        if (result) this.charCache[charType] = result
      }
    }

    const ids = new Set(this.workers.map(w => w.id))
    for (const id of Object.keys(this.animStates)) {
      if (!ids.has(id)) delete this.animStates[id]
    }

    // Register workers with MovementController (starting position = desk charTile)
    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (desk && !this.movement.getPosition(w.id)) {
        this.movement.registerWorker(w.id, desk.charTile[0], desk.charTile[1])
      }
    }
    // Remove departed workers from movement
    for (const id of [...this.movement._positions.keys()]) {
      if (!ids.has(id)) this.movement.removeWorker(id)
    }

    // Notify simulation of updated workers
    this.simulation?.setWorkers(this.workers)
  }

  start() {
    if (this.running) return
    this.running = true
    this.lastTime = performance.now()
    startAmbient()
    this._loop()
  }

  stop() {
    this.running = false
    stopAmbient()
  }

  _loop() {
    if (!this.running) return
    const now = performance.now()
    const delta = now - this.lastTime
    this.lastTime = now
    this._update(delta)
    this._draw()
    requestAnimationFrame(() => this._loop())
  }

  _update(delta) {
    this.globalTime += delta
    this.dataStreamPhase = (this.globalTime % ENV_ANIM.dataStreamSpeed) / ENV_ANIM.dataStreamSpeed

    // Update subsystems
    this.movement.update(delta)
    this.bubbles.update(delta)
    this.simulation?.update(delta)

    // Single pass: update animation, apply walk override, trigger sounds
    let clattered = false
    for (const w of this.workers) {
      const anim = this.animStates[w.id]
      if (!anim) continue

      // Walk direction overrides status-based animation
      const walkAnim = this.movement.getWalkAnimation(w.id)
      if (walkAnim) {
        anim.setState(walkAnim)
      }

      const mood = this.profiles?.get(w.id)?.mood?.current || null
      anim.update(delta, mood)

      // Occasional key clatter for working workers
      if (!clattered && anim.state === 'working' && Math.random() < 0.001) {
        playKeyClatter()
        clattered = true
      }
    }

    // Spawn binary particles near screen tiles
    if (this.particles.length < ENV_ANIM.particleMaxCount && Math.random() < ENV_ANIM.particleSpawnRate) {
      const tile = this.screenTiles[Math.floor(Math.random() * this.screenTiles.length)]
      if (tile) {
        this.particles.push(new BinaryParticle(tile.x + TILE_PX / 2, tile.y))
      }
    }

    // Update particles
    for (let i = this.particles.length - 1; i >= 0; i--) {
      this.particles[i].update(delta)
      if (this.particles[i].dead) this.particles.splice(i, 1)
    }
  }

  _draw() {
    const ctx = this.ctx
    ctx.imageSmoothingEnabled = false

    // 1. Background
    ctx.drawImage(this.bgCanvas, 0, 0)

    // 2. Screen glow halos (pulsing)
    this._drawScreenGlow(ctx)

    // 3. Characters (shadow + sprite + name + bubble)
    const positionMap = {}
    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (!desk) continue
      const anim = this.animStates[w.id]
      if (!anim) continue

      const charType = getCharacterType(w)
      const cache = this.charCache[charType]
      if (!cache) continue

      const frames = cache[anim.state]
      if (!frames) continue
      const frame = frames[anim.getFrame()]
      if (!frame) continue

      // Dynamic position from MovementController (falls back to desk tile)
      const pos = this.movement.getPosition(w.id)
      let px, py
      if (pos) {
        px = pos.pixelX - TILE_PX / 2
        py = pos.pixelY - TILE_PX / 2
      } else {
        px = desk.charTile[0] * TILE_PX
        py = desk.charTile[1] * TILE_PX
      }

      positionMap[w.id] = { pixelX: px, pixelY: py }

      // Shadow ellipse under character
      ctx.save()
      ctx.globalAlpha = 0.3
      ctx.fillStyle = '#000'
      ctx.beginPath()
      ctx.ellipse(px + TILE_PX / 2, py + TILE_PX + 2, TILE_PX * 0.35, 4, 0, 0, Math.PI * 2)
      ctx.fill()
      ctx.restore()

      // Character sprite
      ctx.drawImage(frame, px, py, TILE_PX, TILE_PX)

      // Name label
      ctx.save()
      ctx.font = '7px "Press Start 2P", monospace'
      ctx.textAlign = 'center'
      const nameX = px + TILE_PX / 2
      ctx.fillStyle = 'rgba(0,0,0,0.7)'
      const nameW = ctx.measureText(w.name).width
      ctx.fillRect(nameX - nameW / 2 - 2, py + TILE_PX + 2, nameW + 4, 10)
      ctx.fillStyle = '#ddd'
      ctx.fillText(w.name, nameX, py + TILE_PX + 10)
      ctx.restore()

      // Mood indicator (above character name)
      if (this.profiles) {
        const profile = this.profiles.get(w.id)
        if (profile?.mood) {
          this._drawMoodIndicator(ctx, nameX, py - 12, profile.mood.current)
        }
      }

      // Hover highlight
      if (this.hoveredWorkerId === w.id) {
        ctx.strokeStyle = '#00ff41'
        ctx.lineWidth = 2
        ctx.strokeRect(px - 2, py - 2, TILE_PX + 4, TILE_PX + 4)
      }

      // Status bubble (only when stationary)
      const bubble = BUBBLE_MAP[anim.state]
      if (bubble && !this.movement.isMoving(w.id)) {
        this._drawPixelBubble(ctx, px + TILE_PX - 4, py - 10, bubble)
      }
    }

    // 4. BubbleManager overlay (speech/thought/discussion/meeting bubbles)
    this.bubbles.draw(ctx, positionMap)

    // 5. Floating binary particles
    this._drawParticles(ctx)

    // 7. CRT scanline overlay
    this._drawScanlines(ctx)
  }

  _drawScreenGlow(ctx) {
    const pulse = Math.sin(this.globalTime / ENV_ANIM.screenGlowPulse * Math.PI * 2) * 0.5 + 0.5
    const alpha = 0.08 + pulse * 0.07

    ctx.save()
    for (const tile of this.screenTiles) {
      const cx = tile.x + TILE_PX / 2
      const cy = tile.y + TILE_PX / 2
      const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, TILE_PX * 1.5)
      gradient.addColorStop(0, `rgba(0,221,255,${alpha})`)
      gradient.addColorStop(1, 'rgba(0,221,255,0)')
      ctx.fillStyle = gradient
      ctx.fillRect(tile.x - TILE_PX, tile.y - TILE_PX, TILE_PX * 3, TILE_PX * 3)
    }
    ctx.restore()
  }

  _drawMoodIndicator(ctx, x, y, mood) {
    if (!mood || mood === 'neutral') return

    const icons = {
      'happy': '\u{1F60A}',
      'stressed': '\u{1F4A2}',
      'frustrated': '\u{1F624}',
      'excited': '\u2B50',
      'tired': '\u{1F4A4}'
    }

    const icon = icons[mood]
    if (!icon) return

    ctx.font = '12px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(icon, x, y - 20)
  }

  _drawPixelBubble(ctx, x, y, bubble) {
    ctx.save()
    ctx.font = '8px "Press Start 2P", monospace'
    const tw = ctx.measureText(bubble.text).width
    const bw = tw + 8
    const bh = 14

    // Pixel box (no roundRect — sharp corners for pixel feel)
    ctx.fillStyle = bubble.bg
    ctx.fillRect(x, y, bw, bh)

    // 1px white outline
    ctx.strokeStyle = '#fff'
    ctx.lineWidth = 1
    ctx.strokeRect(x - 0.5, y - 0.5, bw + 1, bh + 1)

    // Tail (2px triangle pointing down-left)
    ctx.fillStyle = bubble.bg
    ctx.fillRect(x + 2, y + bh, 3, 3)
    ctx.fillStyle = '#fff'
    ctx.fillRect(x + 1, y + bh, 1, 3)
    ctx.fillRect(x + 2, y + bh + 2, 3, 1)

    // Text
    ctx.fillStyle = bubble.fg
    ctx.fillText(bubble.text, x + 4, y + 10)
    ctx.restore()
  }

  _drawParticles(ctx) {
    ctx.save()
    ctx.font = '8px monospace'
    for (const p of this.particles) {
      ctx.globalAlpha = p.life * 0.7
      ctx.fillStyle = '#00ff41'
      ctx.fillText(p.char, p.x, p.y)
    }
    ctx.restore()
  }

  _prerenderScanlines() {
    const ctx = this.scanlineCanvas.getContext('2d')
    ctx.fillStyle = 'rgba(0,0,0,0.06)'
    for (let y = 0; y < CANVAS_H; y += 3) {
      ctx.fillRect(0, y, CANVAS_W, 1)
    }
  }

  _drawScanlines(ctx) {
    ctx.drawImage(this.scanlineCanvas, 0, 0)
  }

  getWorkerAtPixel(x, y) {
    for (const w of this.workers) {
      // Use dynamic position if available, otherwise desk position
      const pos = this.movement.getPosition(w.id)
      let wx, wy
      if (pos) {
        wx = pos.pixelX - TILE_PX / 2
        wy = pos.pixelY - TILE_PX / 2
      } else {
        const desk = this.workerDeskMap[w.id]
        if (!desk) continue
        wx = desk.charTile[0] * TILE_PX
        wy = desk.charTile[1] * TILE_PX
      }
      // Hit test with some padding around the sprite
      if (x >= wx - TILE_PX / 2 && x <= wx + TILE_PX * 1.5 &&
          y >= wy - TILE_PX / 2 && y <= wy + TILE_PX * 1.5) {
        return w
      }
    }
    return null
  }

  setHoveredWorker(workerId) {
    this.hoveredWorkerId = workerId
  }

  destroy() {
    this.stop()
    this.bubbles.clearAll()
  }
}
