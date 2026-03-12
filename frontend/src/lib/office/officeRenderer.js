// Canvas 2D rendering engine for Pixel Office — Warm Bright Edition

import { TILE_SIZE, SCALE, getFloorMap, getDesks, buildWorkerDeskMap, getLayoutDimensions, getCurrentLayoutId } from './layout.js'
import { prerenderCharacter, prerenderCharacterFromAppearance, prerenderFurniture, prerenderEnvSprite, prerenderWallVariants, getCharacterType, getWorkerDecorations, drawDeskDecoration, SKILL_PROFILE_COLORS } from './sprites.js'
import { AnimationState, statusToAnim, ENV_ANIM } from './animation.js'
import { PHASES } from './gameClock.js'
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

const TALL_FURNITURE_TILES = new Set([3, 5, 6, 12, 13, 14])

// Time-of-day lighting overlay configs
const PHASE_LIGHTING = {
  [PHASES.MORNING_ARRIVAL]: { color: 'rgba(255,230,150,0.04)', screenGlow: false },
  [PHASES.WORK_MORNING]:    { color: null, screenGlow: false },                     // brightest, no overlay
  [PHASES.LUNCH]:           { color: 'rgba(255,240,200,0.03)', screenGlow: false },
  [PHASES.WORK_AFTERNOON]:  { color: null, screenGlow: false },
  [PHASES.TEA_BREAK]:       { color: 'rgba(255,220,150,0.05)', screenGlow: false },
  [PHASES.WORK_LATE]:       { color: 'rgba(255,180,100,0.08)', screenGlow: false },
  [PHASES.OVERTIME]:        { color: 'rgba(20,30,80,0.15)', screenGlow: true },
  [PHASES.NIGHT]:           { color: 'rgba(10,15,50,0.25)', screenGlow: true },
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

// ── Confetti particle for celebrations ───────────────────────────────────────
const CONFETTI_COLORS = ['#cc4444', '#e8a855', '#5cb86e', '#5bbad5', '#b088d0', '#ff69b4', '#ffcc66']

class ConfettiParticle {
  constructor(x, y) {
    this.x = x + (Math.random() - 0.5) * 20
    this.y = y
    this.vx = (Math.random() - 0.5) * 3
    this.vy = -2 - Math.random() * 3
    this.gravity = 0.06
    this.rotation = Math.random() * Math.PI * 2
    this.rotSpeed = (Math.random() - 0.5) * 0.2
    this.size = 3 + Math.random() * 3
    this.color = CONFETTI_COLORS[Math.floor(Math.random() * CONFETTI_COLORS.length)]
    this.life = 1.0
  }
  update(delta) {
    const dt = delta / 16
    this.vy += this.gravity * dt
    this.x += this.vx * dt
    this.y += this.vy * dt
    this.rotation += this.rotSpeed * dt
    this.life -= delta / 3000
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

    // Prerender wall variants (16 bitmask combos)
    this.wallVariants = prerenderWallVariants()

    // Prerender chair
    this.furnitureCache['chair'] = prerenderFurniture('chair')

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

    // Build tall furniture list for Z-Sort
    this._buildTallFurniture()
    this.chairs = []

    // Runtime state
    this.workers = []
    this.workerDeskMap = {}
    this.animStates = {}
    this.hoveredWorkerId = null

    // Floating dust mote particles
    this.particles = []
    // Confetti particles (celebrations)
    this.confetti = []

    // Global time for pulsing effects
    this.globalTime = 0

    this.lastTime = 0
    this.running = false

    this._drawBackground()
  }

  _getWorkerDecorations(worker) {
    if (!this._decoCache) this._decoCache = {}
    if (!this._decoCache[worker.id]) {
      this._decoCache[worker.id] = getWorkerDecorations(worker.name, worker.skillProfile)
    }
    return this._decoCache[worker.id]
  }

  _buildTallFurniture() {
    this.tallFurniture = []
    const floor = this.floorMap
    for (let r = 0; r < floor.length; r++) {
      for (let c = 0; c < floor[r].length; c++) {
        if (TALL_FURNITURE_TILES.has(floor[r][c])) {
          const name = TILE_TO_FURNITURE[floor[r][c]]
          this.tallFurniture.push({ name, col: c, row: r, y: (r + 1) * TILE_PX })
        }
      }
    }
  }

  _buildChairList() {
    this.chairs = []
    for (const [wid, desk] of Object.entries(this.workerDeskMap)) {
      const [col, row] = desk.charTile
      this.chairs.push({ col, row, y: (row + 1) * TILE_PX, workerId: wid })
    }
  }

  _drawBackground() {
    const ctx = this.bgCanvas.getContext('2d')

    // First pass: base floor tiles
    for (let row = 0; row < this.ROWS; row++) {
      for (let col = 0; col < this.COLS; col++) {
        const tile = this.floorMap[row][col]
        ctx.fillStyle = FLOOR_COLORS[tile] || FLOOR_COLORS[0]
        ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)

        // Subtle wood plank lines for floor tiles
        if (tile === 0 || tile === 7 || tile === 8 || tile === 9 ||
            (tile >= 2 && tile <= 6) || tile === 10 || tile === 11 ||
            tile === 12 || tile === 13 || tile === 14 || tile === 15) {
          // Plank grain lines (horizontal, staggered by row)
          ctx.fillStyle = 'rgba(160,130,90,0.07)'
          const offset = (row % 2) * (TILE_PX / 2)
          ctx.fillRect(col * TILE_PX, row * TILE_PX + 12, TILE_PX, 1)
          ctx.fillRect(col * TILE_PX, row * TILE_PX + 36, TILE_PX, 1)
          // Vertical seam
          ctx.fillRect(col * TILE_PX + offset + 20, row * TILE_PX, 1, TILE_PX)
          // Subtle grid
          ctx.strokeStyle = 'rgba(160,130,90,0.04)'
          ctx.strokeRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Wall: auto-tiled using bitmask variants
        if (tile === 1) {
          const floor = this.floorMap
          const rows = floor.length, cols = floor[0].length
          const hasN = row > 0 && floor[row - 1][col] === 1 ? 1 : 0
          const hasE = col < cols - 1 && floor[row][col + 1] === 1 ? 2 : 0
          const hasS = row < rows - 1 && floor[row + 1][col] === 1 ? 4 : 0
          const hasW = col > 0 && floor[row][col - 1] === 1 ? 8 : 0
          const mask = hasN | hasE | hasS | hasW
          ctx.drawImage(this.wallVariants[mask], col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Door: carved wood panel door
        if (tile === 7) {
          ctx.fillStyle = '#dcc8a0'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, TILE_PX)
          // Door panels (2 inset rectangles)
          ctx.fillStyle = '#c9b896'
          ctx.fillRect(col * TILE_PX + 8, row * TILE_PX + 4, TILE_PX - 16, TILE_PX / 2 - 6)
          ctx.fillRect(col * TILE_PX + 8, row * TILE_PX + TILE_PX / 2 + 2, TILE_PX - 16, TILE_PX / 2 - 6)
          // Door handle
          ctx.fillStyle = '#e8a855'
          ctx.fillRect(col * TILE_PX + TILE_PX - 14, row * TILE_PX + TILE_PX / 2 - 2, 3, 4)
          // Frame lines
          ctx.fillStyle = '#bfad88'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, 2)
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX + TILE_PX - 2, TILE_PX - 8, 2)
        }

        // Draw env sprites
        const envName = TILE_TO_ENV[tile]
        if (envName && this.envCache[envName]) {
          ctx.imageSmoothingEnabled = false
          ctx.drawImage(this.envCache[envName], col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Draw furniture sprites on tiles (skip tall furniture — handled by Z-Sort)
        const furnitureName = TILE_TO_FURNITURE[tile]
        if (furnitureName && this.furnitureCache[furnitureName] && !TALL_FURNITURE_TILES.has(tile)) {
          ctx.imageSmoothingEnabled = false
          ctx.drawImage(this.furnitureCache[furnitureName], col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }
      }
    }

    // Second pass: ambient lighting overlays

    // Ceiling lights (every 4-6 tiles in open areas)
    for (let row = 0; row < this.ROWS; row++) {
      for (let col = 0; col < this.COLS; col++) {
        const tile = this.floorMap[row][col]
        // Place ceiling lights on floor tiles at regular intervals
        if ((tile === 0 || tile === 10 || tile === 15) && col % 5 === 2 && row % 4 === 1) {
          // Warm light pool on the floor
          const cx = col * TILE_PX + TILE_PX / 2
          const cy = row * TILE_PX + TILE_PX / 2
          const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, TILE_PX * 1.5)
          grad.addColorStop(0, 'rgba(255,240,200,0.08)')
          grad.addColorStop(1, 'rgba(255,240,200,0)')
          ctx.fillStyle = grad
          ctx.fillRect(cx - TILE_PX * 1.5, cy - TILE_PX * 1.5, TILE_PX * 3, TILE_PX * 3)
          // Light fixture dot on ceiling
          ctx.fillStyle = 'rgba(255,248,220,0.25)'
          ctx.fillRect(cx - 2, cy - TILE_PX / 2, 4, 3)
        }
      }
    }

    // Window light beams on the top edge walls
    for (let col = 0; col < this.COLS; col++) {
      if (this.floorMap[0]?.[col] === 1 && this.floorMap[1]?.[col] !== 1) {
        // Sunlight beam from window
        const wx = col * TILE_PX
        const wy = TILE_PX  // starts just below wall
        if (col % 3 === 0) {
          const grad = ctx.createLinearGradient(wx, wy, wx + TILE_PX, wy + TILE_PX * 3)
          grad.addColorStop(0, 'rgba(255,245,200,0.06)')
          grad.addColorStop(1, 'rgba(255,245,200,0)')
          ctx.fillStyle = grad
          ctx.fillRect(wx, wy, TILE_PX * 2, TILE_PX * 3)
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

  // Spawn confetti burst at a worker's position
  spawnConfetti(workerId, count = 25) {
    const pos = this.movement.getPosition(workerId)
    if (!pos) return
    for (let i = 0; i < count; i++) {
      this.confetti.push(new ConfettiParticle(pos.pixelX, pos.pixelY - TILE_PX / 2))
    }
  }

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

    // Build chair list from desk assignments
    this._buildChairList()

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

    // Update confetti
    for (let i = this.confetti.length - 1; i >= 0; i--) {
      this.confetti[i].update(delta)
      if (this.confetti[i].dead) this.confetti.splice(i, 1)
    }
  }

  _drawCharacter(ctx, w, px, py) {
    const desk = this.workerDeskMap[w.id]
    const anim = this.animStates[w.id]
    if (!anim) return

    const charType = getCharacterType(w)
    const cache = this.charCache[charType]
    if (!cache) return

    const frames = cache[anim.state]
    if (!frames) return
    const frame = frames[anim.getFrame()]
    if (!frame) return

    // Shadow ellipse under character
    ctx.save()
    ctx.globalAlpha = 0.2
    ctx.fillStyle = '#8b7355'
    ctx.beginPath()
    ctx.ellipse(px + TILE_PX / 2, py + TILE_PX + 2, TILE_PX * 0.35, 4, 0, 0, Math.PI * 2)
    ctx.fill()
    ctx.restore()

    // Personalized skill color band at desk edge
    if (desk && !this.movement.isMoving(w.id)) {
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

    // Mood indicator (above character)
    if (this.profiles) {
      const profile = this.profiles.get(w.id)
      if (profile?.mood) {
        this._drawMoodIndicator(ctx, px + TILE_PX / 2, py - 12, profile.mood.current)
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

  _draw() {
    const ctx = this.ctx
    ctx.imageSmoothingEnabled = false

    // 1. Background (floor + low furniture)
    ctx.drawImage(this.bgCanvas, 0, 0)

    // 2. Desk decorations (drawn on desk surface, below Z-sorted items)
    for (const [wid, desk] of Object.entries(this.workerDeskMap)) {
      const worker = this.workers.find(w => w.id === wid)
      if (!worker) continue
      const decos = this._getWorkerDecorations(worker)
      const dx = desk.tile[0] * TILE_PX
      const dy = desk.tile[1] * TILE_PX
      // Position decorations on desk surface (top portion of tile)
      const positions = [[dx + 30, dy + 4], [dx + 4, dy + 4], [dx + 18, dy + 2]]
      for (let i = 0; i < decos.length && i < positions.length; i++) {
        drawDeskDecoration(ctx, decos[i], positions[i][0], positions[i][1])
      }
    }

    // 3. Collect drawables for Z-Sort (tall furniture + chairs + characters)
    const drawables = []
    const positionMap = {}

    // Tall furniture
    for (const f of this.tallFurniture) {
      drawables.push({ type: 'furniture', name: f.name, x: f.col * TILE_PX, y: f.y, row: f.row })
    }

    // Chairs (sort Y biased upward so chair draws BEHIND seated character)
    for (const chair of this.chairs) {
      drawables.push({ type: 'furniture', name: 'chair', x: chair.col * TILE_PX, y: chair.y - TILE_PX * 0.4, row: chair.row })
    }

    // Characters
    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (!desk) continue

      const pos = this.movement.getPosition(w.id)
      let px, py
      if (pos) {
        px = pos.pixelX - TILE_PX / 2
        py = pos.pixelY - TILE_PX / 2
      } else {
        px = desk.charTile[0] * TILE_PX
        py = desk.charTile[1] * TILE_PX
      }

      // Seated offset: -3px when at desk and not moving
      const isSeated = !this.movement.isMoving(w.id)
      const seatOffset = isSeated ? -3 : 0

      positionMap[w.id] = { pixelX: px, pixelY: py + seatOffset }
      drawables.push({ type: 'character', worker: w, px, py: py + seatOffset, y: py + TILE_PX + seatOffset })
    }

    // Sort by Y (bottom edge), furniture before characters at same Y
    drawables.sort((a, b) => a.y - b.y || (a.type === 'furniture' ? -1 : 1))

    // Draw all sorted drawables
    for (const d of drawables) {
      if (d.type === 'furniture') {
        const img = this.furnitureCache[d.name]
        if (!img) continue
        ctx.drawImage(img, d.x, d.row * TILE_PX, TILE_PX, TILE_PX)
      } else {
        this._drawCharacter(ctx, d.worker, d.px, d.py)
      }
    }

    // 4. Code review status flags at desks
    this._drawReviewFlags(ctx)

    // 5. Pair programming connecting lines
    this._drawPairProgrammingLines(ctx, positionMap)

    // 6. Confetti particles
    this._drawConfetti(ctx)

    // 7. BubbleManager overlay (speech/thought/discussion/meeting bubbles)
    this.bubbles.draw(ctx, positionMap)

    // 8. Floating warm dust motes
    this._drawParticles(ctx)

    // 9. Time-of-day lighting overlay
    this._drawTimeOverlay(ctx)

    // 10. Subtle vignette overlay for depth
    this._drawVignette(ctx)
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
      ctx.globalAlpha = p.life * 0.35
      // Warm golden dust motes with glow
      const grad = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.size * 2)
      grad.addColorStop(0, 'rgba(255,230,160,0.6)')
      grad.addColorStop(0.5, 'rgba(212,167,106,0.3)')
      grad.addColorStop(1, 'rgba(212,167,106,0)')
      ctx.fillStyle = grad
      ctx.beginPath()
      ctx.arc(p.x, p.y, p.size * 2, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
  }

  _drawReviewFlags(ctx) {
    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (!desk) continue
      const status = w.status
      let flagColor = null
      if (status === 'code_review' || status === 'reviewing') flagColor = '#e8a855' // yellow = pending
      else if (status === 'review_approved') flagColor = '#5cb86e' // green
      else if (status === 'review_rejected') flagColor = '#cc4444' // red
      if (!flagColor) continue

      const fx = desk.tile[0] * TILE_PX + TILE_PX - 8
      const fy = desk.tile[1] * TILE_PX - 2
      // Flag pole
      ctx.strokeStyle = '#666'
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(fx, fy)
      ctx.lineTo(fx, fy + 14)
      ctx.stroke()
      // Flag
      ctx.fillStyle = flagColor
      ctx.beginPath()
      ctx.moveTo(fx, fy)
      ctx.lineTo(fx + 8, fy + 3)
      ctx.lineTo(fx, fy + 6)
      ctx.closePath()
      ctx.fill()
    }
  }

  _drawPairProgrammingLines(ctx, positionMap) {
    if (!this.simulation?.workerStates) return
    const drawn = new Set()
    for (const [wid, ws] of this.simulation.workerStates) {
      if (ws.data?.activity !== 'pair_programming') continue
      const tid = ws.data?.targetId
      if (!tid || drawn.has(`${tid}-${wid}`)) continue
      drawn.add(`${wid}-${tid}`)
      const posA = positionMap[wid]
      const posB = positionMap[tid]
      if (!posA || !posB) continue
      // Dotted connecting line
      ctx.save()
      ctx.strokeStyle = '#88aa88'
      ctx.lineWidth = 1.5
      ctx.setLineDash([4, 4])
      ctx.beginPath()
      ctx.moveTo(posA.pixelX + TILE_PX / 2, posA.pixelY + TILE_PX / 2)
      ctx.lineTo(posB.pixelX + TILE_PX / 2, posB.pixelY + TILE_PX / 2)
      ctx.stroke()
      ctx.restore()
      // Small code icon at midpoint
      const mx = (posA.pixelX + posB.pixelX) / 2 + TILE_PX / 2
      const my = (posA.pixelY + posB.pixelY) / 2 + TILE_PX / 2 - 10
      ctx.font = '10px sans-serif'
      ctx.textAlign = 'center'
      ctx.fillText('\u{1F4BB}', mx, my)
    }
  }

  _drawConfetti(ctx) {
    if (this.confetti.length === 0) return
    ctx.save()
    for (const c of this.confetti) {
      ctx.globalAlpha = Math.max(0, c.life)
      ctx.fillStyle = c.color
      ctx.save()
      ctx.translate(c.x, c.y)
      ctx.rotate(c.rotation)
      ctx.fillRect(-c.size / 2, -c.size / 2, c.size, c.size * 0.6)
      ctx.restore()
    }
    ctx.restore()
  }

  _drawTimeOverlay(ctx) {
    const phase = this.simulation?.gameClock?.getCurrentPhase()
    if (!phase) return
    const lighting = PHASE_LIGHTING[phase]
    if (!lighting) return

    const w = this.canvas.width
    const h = this.canvas.height

    // Global color overlay
    if (lighting.color) {
      ctx.fillStyle = lighting.color
      ctx.fillRect(0, 0, w, h)
    }

    // Screen glow + desk lamp halos during overtime/night
    if (lighting.screenGlow) {
      for (const [wid, desk] of Object.entries(this.workerDeskMap)) {
        const worker = this.workers.find(wr => wr.id === wid)
        if (!worker) continue
        const isWorking = worker.status === 'working' || worker.status === 'busy'
        const dx = desk.tile[0] * TILE_PX + TILE_PX / 2
        const dy = desk.tile[1] * TILE_PX + TILE_PX / 2

        if (isWorking) {
          // Screen blue glow
          const scrGrad = ctx.createRadialGradient(dx, dy, 0, dx, dy, TILE_PX * 1.2)
          scrGrad.addColorStop(0, 'rgba(100,160,255,0.12)')
          scrGrad.addColorStop(1, 'rgba(100,160,255,0)')
          ctx.fillStyle = scrGrad
          ctx.fillRect(dx - TILE_PX * 1.2, dy - TILE_PX * 1.2, TILE_PX * 2.4, TILE_PX * 2.4)
        }

        // Warm desk lamp halo (all occupied desks)
        const lampGrad = ctx.createRadialGradient(dx - 8, dy - 8, 0, dx - 8, dy - 8, TILE_PX * 0.8)
        lampGrad.addColorStop(0, 'rgba(255,220,150,0.1)')
        lampGrad.addColorStop(1, 'rgba(255,220,150,0)')
        ctx.fillStyle = lampGrad
        ctx.fillRect(dx - TILE_PX, dy - TILE_PX, TILE_PX * 2, TILE_PX * 2)
      }
    }
  }

  _drawVignette(ctx) {
    const w = this.canvas.width
    const h = this.canvas.height
    const grad = ctx.createRadialGradient(w / 2, h / 2, Math.min(w, h) * 0.35, w / 2, h / 2, Math.max(w, h) * 0.7)
    grad.addColorStop(0, 'rgba(0,0,0,0)')
    grad.addColorStop(1, 'rgba(40,30,15,0.12)')
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, w, h)
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
    this._buildTallFurniture()
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
