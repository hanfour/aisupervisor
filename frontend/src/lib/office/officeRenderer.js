// Canvas 2D rendering engine for Pixel Office — Warm Bright Edition

import { TILE_SIZE, SCALE, getFloorMap, getDesks, buildWorkerDeskMap, getLayoutDimensions, getCurrentLayoutId } from './layout.js'
import { prerenderCharacter, prerenderCharacterFromAppearance, prerenderFurniture, prerenderEnvSprite, getCharacterType, SKILL_PROFILE_COLORS } from './sprites.js'
import { AnimationState, statusToAnim, ENV_ANIM } from './animation.js'
import { startAmbient, stopAmbient, playKeyClatter } from './sounds.js'
import { MovementController } from './movement.js'
import { BubbleManager } from './bubbles.js'

let TILE_PX = TILE_SIZE * SCALE  // 48

// ── Warm bright floor/wall palette ───────────────────────────────────────────
const FLOOR_COLORS = {
  0: '#e8d5b7',  // warm wood floor
  1: '#d4c4a8',  // beige wall
  2: '#e8d5b7',  // desk (drawn as furniture)
  3: '#e8d5b7',  // plant (drawn as furniture)
  4: '#e8d5b7',  // computer (drawn as furniture)
  5: '#e8d5b7',  // watercooler (drawn as furniture)
  6: '#e8d5b7',  // bookshelf (drawn as furniture)
  7: '#f0e6d3',  // door — light cream
  8: '#e8d5b7',  // baseboard (drawn as env sprite)
  9: '#d4a76a',  // rugPattern — warm brown
  10: '#f5efe6', // meetingFloor — clean cream
  11: '#e8e8f0', // whiteboard — off-white face color
  12: '#e8d5b7', // coffeeBar (drawn as furniture)
  13: '#e8d5b7', // sofa (drawn as furniture)
  14: '#e8d5b7', // largePlant (drawn as furniture)
  15: '#f0e0c8', // coffeeFloor — warm peach
}

const TILE_TO_FURNITURE = {
  2: 'desk',
  3: 'plant',
  4: 'computer',
  5: 'watercooler',
  6: 'bookshelf',
  10: 'meetingTable',
  11: 'whiteboard',
  12: 'coffeeBar',
  13: 'sofa',
  14: 'largePlant',
}

const TILE_TO_ENV = {
  8: 'baseboard',
  9: 'rugPattern',
}

const BUBBLE_MAP = {
  waiting: { text: '?', bg: '#ffdd57', fg: '#333' },
  working: { text: '...', bg: '#5bbad5', fg: '#fff' },
  error:   { text: '!', bg: '#e07050', fg: '#fff' },
  finished:{ text: '\u2605', bg: '#6bb87b', fg: '#000' },
}

// ── Warm dust mote particle ─────────────────────────────────────────────────
class DustMote {
  constructor(canvasW, canvasH) {
    this.x = Math.random() * canvasW
    this.y = Math.random() * canvasH
    this.life = 1.0
    this.speed = 0.1 + Math.random() * 0.2
    this.drift = (Math.random() - 0.5) * 0.15
    this.size = 1 + Math.random() * 2
  }
  update(delta) {
    this.y -= this.speed * (delta / 16)
    this.x += this.drift * (delta / 16)
    this.life -= delta / 5000
  }
  get dead() { return this.life <= 0 }
}

export class OfficeRenderer {
  constructor(canvas) {
    this.canvas = canvas
    this.ctx = canvas.getContext('2d')

    const dims = getLayoutDimensions()
    this.layoutId = getCurrentLayoutId()
    this.COLS = dims.cols
    this.ROWS = dims.rows
    canvas.width = dims.canvasW
    canvas.height = dims.canvasH
    this.floorMap = getFloorMap()
    this.desks = getDesks()

    // Static background layer (drawn once)
    this.bgCanvas = document.createElement('canvas')
    this.bgCanvas.width = dims.canvasW
    this.bgCanvas.height = dims.canvasH

    // Prerender furniture
    this.furnitureCache = {}
    for (const name of ['desk', 'computer', 'plant', 'watercooler', 'bookshelf', 'meetingTable', 'whiteboard', 'coffeeBar', 'sofa', 'largePlant']) {
      this.furnitureCache[name] = prerenderFurniture(name)
    }

    // Prerender env sprites
    this.envCache = {}
    for (const name of ['baseboard', 'rugPattern']) {
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

    // Floating dust mote particles
    this.particles = []

    // Global time for pulsing effects
    this.globalTime = 0

    this.lastTime = 0
    this.running = false

    this._drawBackground()
  }

  _drawBackground() {
    const ctx = this.bgCanvas.getContext('2d')

    for (let row = 0; row < this.ROWS; row++) {
      for (let col = 0; col < this.COLS; col++) {
        const tile = this.floorMap[row][col]
        ctx.fillStyle = FLOOR_COLORS[tile] || FLOOR_COLORS[0]
        ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)

        // Subtle warm grid lines for floor tiles
        if (tile === 0 || tile === 7 || tile === 8 || tile === 9 ||
            (tile >= 2 && tile <= 6) || tile === 10 || tile === 11 ||
            tile === 12 || tile === 13 || tile === 14 || tile === 15) {
          ctx.strokeStyle = 'rgba(160,130,90,0.06)'
          ctx.strokeRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Wall: warm wood highlight lines
        if (tile === 1) {
          ctx.fillStyle = '#c9b896'
          ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, 2)
          ctx.fillStyle = '#bfad88'
          ctx.fillRect(col * TILE_PX, row * TILE_PX + TILE_PX / 2, TILE_PX, 1)
        }

        // Door: warm wood accent
        if (tile === 7) {
          ctx.fillStyle = '#dcc8a0'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, TILE_PX)
          ctx.fillStyle = '#c9b896'
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

    // Zone labels (layout-specific)
    ctx.font = '10px "Press Start 2P", monospace'
    ctx.fillStyle = 'rgba(140,110,70,0.15)'
    const ZONE_LABELS = {
      standard: [
        ['OPEN OFFICE', 2, 5.5], ['MGR', 1, 11.5], ['MEETING', 11, 10.5],
        ['COFFEE', 19, 10.5], ['REST', 19, 12], ['REC', 12, 14],
      ],
      startup: [
        ['OPEN OFFICE', 2, 3.5], ['MGR', 1, 9.5], ['COFFEE', 12, 9.5],
      ],
      enterprise: [
        ['OPEN OFFICE', 2, 8.5], ['MGR', 1, 14.5], ['MEETING', 13, 13.5],
        ['COFFEE', 22, 12.5], ['REST', 22, 14.5], ['REC', 6, 16.5],
      ],
    }
    const labels = ZONE_LABELS[this.layoutId] || ZONE_LABELS.standard
    for (const [text, col, row] of labels) {
      ctx.fillText(text, col * TILE_PX, row * TILE_PX)
    }
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
        // Custom appearance from backend
        const result = w.appearance
          ? prerenderCharacterFromAppearance(w.appearance)
          : prerenderCharacter(charType)
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

    // Spawn warm dust motes
    if (this.particles.length < ENV_ANIM.dustMoteMaxCount && Math.random() < ENV_ANIM.dustMoteSpawnRate) {
      this.particles.push(new DustMote(this.canvas.width, this.canvas.height))
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

    // 2. Characters (shadow + sprite + skill color band + name + bubble)
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
      ctx.globalAlpha = 0.2
      ctx.fillStyle = '#8b7355'
      ctx.beginPath()
      ctx.ellipse(px + TILE_PX / 2, py + TILE_PX + 2, TILE_PX * 0.35, 4, 0, 0, Math.PI * 2)
      ctx.fill()
      ctx.restore()

      // Personalized skill color band at desk edge
      if (!this.movement.isMoving(w.id)) {
        const skillProfile = w.skillProfile || w.avatar || ''
        const bandColor = SKILL_PROFILE_COLORS[skillProfile]
        if (bandColor) {
          const deskTile = desk.tile
          ctx.fillStyle = bandColor
          ctx.fillRect(deskTile[0] * TILE_PX, deskTile[1] * TILE_PX, TILE_PX, 3)
        }
      }

      // Character sprite
      ctx.drawImage(frame, px, py, TILE_PX, TILE_PX)

      // Name label
      ctx.save()
      ctx.font = '7px "Press Start 2P", monospace'
      ctx.textAlign = 'center'
      const nameX = px + TILE_PX / 2
      ctx.fillStyle = 'rgba(60,40,20,0.7)'
      const nameW = ctx.measureText(w.name).width
      ctx.fillRect(nameX - nameW / 2 - 2, py + TILE_PX + 2, nameW + 4, 10)
      ctx.fillStyle = '#f5efe6'
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
        ctx.strokeStyle = '#e8a855'
        ctx.lineWidth = 2
        ctx.strokeRect(px - 2, py - 2, TILE_PX + 4, TILE_PX + 4)
      }

      // Status bubble (only when stationary)
      const bubble = BUBBLE_MAP[anim.state]
      if (bubble && !this.movement.isMoving(w.id)) {
        this._drawPixelBubble(ctx, px + TILE_PX - 4, py - 10, bubble)
      }
    }

    // 3. BubbleManager overlay (speech/thought/discussion/meeting bubbles)
    this.bubbles.draw(ctx, positionMap)

    // 4. Floating warm dust motes
    this._drawParticles(ctx)
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

    // 1px warm outline
    ctx.strokeStyle = '#c9a868'
    ctx.lineWidth = 1
    ctx.strokeRect(x - 0.5, y - 0.5, bw + 1, bh + 1)

    // Tail (2px triangle pointing down-left)
    ctx.fillStyle = bubble.bg
    ctx.fillRect(x + 2, y + bh, 3, 3)
    ctx.fillStyle = '#c9a868'
    ctx.fillRect(x + 1, y + bh, 1, 3)
    ctx.fillRect(x + 2, y + bh + 2, 3, 1)

    // Text
    ctx.fillStyle = bubble.fg
    ctx.fillText(bubble.text, x + 4, y + 10)
    ctx.restore()
  }

  _drawParticles(ctx) {
    ctx.save()
    for (const p of this.particles) {
      ctx.globalAlpha = p.life * 0.4
      ctx.fillStyle = '#d4a76a'
      ctx.beginPath()
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
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

  // Switch to a different office layout
  switchLayout(layoutId) {
    const dims = getLayoutDimensions(layoutId)
    this.layoutId = layoutId
    this.COLS = dims.cols
    this.ROWS = dims.rows
    this.canvas.width = dims.canvasW
    this.canvas.height = dims.canvasH
    this.floorMap = getFloorMap(layoutId)
    this.desks = getDesks(layoutId)

    this.bgCanvas.width = dims.canvasW
    this.bgCanvas.height = dims.canvasH
    this._drawBackground()

    // Reset movement positions (workers will be re-registered in setWorkers)
    this.movement = new MovementController()
    this.animStates = {}
  }

  // Invalidate character cache for a worker (call after appearance change)
  invalidateCharCache(workerId) {
    delete this.charCache[`custom_${workerId}`]
  }

  setHoveredWorker(workerId) {
    this.hoveredWorkerId = workerId
  }

  destroy() {
    this.stop()
    this.bubbles.clearAll()
  }
}
