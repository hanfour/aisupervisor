// Canvas 2D rendering engine for Pixel Office

import { TILE_SIZE, SCALE, COLS, ROWS, CANVAS_W, CANVAS_H, getFloorMap, getDesks, buildWorkerDeskMap } from './layout.js'
import { prerenderCharacter, prerenderFurniture, getCharacterType } from './sprites.js'
import { AnimationState, statusToAnim } from './animation.js'

const TILE_PX = TILE_SIZE * SCALE  // 48

const FLOOR_COLORS = {
  0: '#3a3a5c',  // floor
  1: '#2a2a3e',  // wall
  2: '#8B7355',  // desk (drawn as furniture)
  3: '#3a3a5c',  // plant (drawn as furniture)
  4: '#3a3a5c',  // computer (drawn as furniture)
  5: '#3a3a5c',  // watercooler (drawn as furniture)
  6: '#3a3a5c',  // bookshelf (drawn as furniture)
  7: '#4a4a3e',  // door
}

const TILE_TO_FURNITURE = {
  2: 'desk',
  3: 'plant',
  4: 'computer',
  5: 'watercooler',
  6: 'bookshelf',
}

const BUBBLE_MAP = {
  waiting: { text: '?', bg: '#ffdd57', fg: '#333' },
  working: { text: '...', bg: '#48f', fg: '#fff' },
  error:   { text: '!', bg: '#ff3860', fg: '#fff' },
  finished:{ text: '\u2605', bg: '#00ff41', fg: '#000' },
}

const TIER_COLORS = {
  consultant: '#f0c040',
  manager: '#60a0ff',
  engineer: '#a0a0a0',
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
    for (const name of ['desk', 'computer', 'plant', 'watercooler', 'bookshelf']) {
      this.furnitureCache[name] = prerenderFurniture(name)
    }

    // Character caches: charType → { state: [canvases] }
    this.charCache = {}

    // Runtime state
    this.workers = []
    this.workerDeskMap = {}  // workerId → desk
    this.animStates = {}     // workerId → AnimationState
    this.hoveredWorkerId = null

    this.lastTime = 0
    this.running = false

    this._drawBackground()
  }

  _drawBackground() {
    const ctx = this.bgCanvas.getContext('2d')

    // Draw floor tiles
    for (let row = 0; row < ROWS; row++) {
      for (let col = 0; col < COLS; col++) {
        const tile = this.floorMap[row][col]
        ctx.fillStyle = FLOOR_COLORS[tile] || FLOOR_COLORS[0]
        ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)

        // Grid lines for floor
        if (tile === 0 || tile === 7) {
          ctx.strokeStyle = 'rgba(255,255,255,0.03)'
          ctx.strokeRect(col * TILE_PX, row * TILE_PX, TILE_PX, TILE_PX)
        }

        // Wall top highlight
        if (tile === 1) {
          ctx.fillStyle = '#353550'
          ctx.fillRect(col * TILE_PX, row * TILE_PX, TILE_PX, 2)
        }

        // Door marking
        if (tile === 7) {
          ctx.fillStyle = '#6a6a4e'
          ctx.fillRect(col * TILE_PX + 4, row * TILE_PX, TILE_PX - 8, TILE_PX)
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
    ctx.fillStyle = 'rgba(255,255,255,0.15)'
    ctx.fillText('OPEN OFFICE', 2 * TILE_PX, 6.5 * TILE_PX)
    ctx.fillText('MGR', 2 * TILE_PX, 12.5 * TILE_PX)
    ctx.fillText('CON', 2 * TILE_PX, 11 * TILE_PX)
    ctx.fillText('REC', 12 * TILE_PX, 12.5 * TILE_PX)
  }

  setWorkers(workers, assignments) {
    this.workers = workers || []
    this.workerDeskMap = buildWorkerDeskMap(assignments)

    // Init animation states for new workers
    for (const w of this.workers) {
      if (!this.animStates[w.id]) {
        this.animStates[w.id] = new AnimationState()
      }
      const anim = statusToAnim(w.status)
      this.animStates[w.id].setState(anim)

      // Ensure character sprites are cached
      const charType = getCharacterType(w, this.workers.indexOf(w))
      if (!this.charCache[charType]) {
        this.charCache[charType] = prerenderCharacter(charType)
      }
    }

    // Cleanup removed workers
    const ids = new Set(this.workers.map(w => w.id))
    for (const id of Object.keys(this.animStates)) {
      if (!ids.has(id)) delete this.animStates[id]
    }
  }

  start() {
    if (this.running) return
    this.running = true
    this.lastTime = performance.now()
    this._loop()
  }

  stop() {
    this.running = false
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
    for (const w of this.workers) {
      const anim = this.animStates[w.id]
      if (anim) anim.update(delta)
    }
  }

  _draw() {
    const ctx = this.ctx
    ctx.imageSmoothingEnabled = false

    // Draw cached background
    ctx.drawImage(this.bgCanvas, 0, 0)

    // Draw hierarchy lines first (behind characters)
    this._drawHierarchyLines(ctx)

    // Draw characters
    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (!desk) continue
      const anim = this.animStates[w.id]
      if (!anim) continue

      const charType = getCharacterType(w, this.workers.indexOf(w))
      const cache = this.charCache[charType]
      if (!cache) continue

      const frames = cache[anim.state]
      if (!frames) continue
      const frame = frames[anim.getFrame()]
      if (!frame) continue

      const px = desk.charTile[0] * TILE_PX
      const py = desk.charTile[1] * TILE_PX

      ctx.drawImage(frame, px, py, TILE_PX, TILE_PX)

      // Hover highlight
      if (this.hoveredWorkerId === w.id) {
        ctx.strokeStyle = '#00ff41'
        ctx.lineWidth = 2
        ctx.strokeRect(px - 2, py - 2, TILE_PX + 4, TILE_PX + 4)

        // Name tag
        ctx.font = '8px "Press Start 2P", monospace'
        ctx.fillStyle = '#000'
        const nameW = ctx.measureText(w.name).width
        ctx.fillRect(px - 2, py - 16, nameW + 6, 14)
        ctx.fillStyle = '#00ff41'
        ctx.fillText(w.name, px + 1, py - 5)
      }

      // Status bubble
      const animState = anim.state
      const bubble = BUBBLE_MAP[animState]
      if (bubble) {
        const bx = px + TILE_PX - 4
        const by = py - 8
        ctx.fillStyle = bubble.bg
        const tw = ctx.measureText(bubble.text).width
        ctx.beginPath()
        ctx.roundRect(bx, by, tw + 8, 14, 3)
        ctx.fill()
        ctx.fillStyle = bubble.fg
        ctx.font = '8px "Press Start 2P", monospace'
        ctx.fillText(bubble.text, bx + 4, by + 10)
      }
    }
  }

  _drawHierarchyLines(ctx) {
    const workerMap = {}
    for (const w of this.workers) workerMap[w.id] = w

    for (const w of this.workers) {
      if (!w.parentID) continue
      const parent = workerMap[w.parentID]
      if (!parent) continue

      const childDesk = this.workerDeskMap[w.id]
      const parentDesk = this.workerDeskMap[parent.id]
      if (!childDesk || !parentDesk) continue

      const cx = childDesk.charTile[0] * TILE_PX + TILE_PX / 2
      const cy = childDesk.charTile[1] * TILE_PX + TILE_PX / 2
      const px = parentDesk.charTile[0] * TILE_PX + TILE_PX / 2
      const py = parentDesk.charTile[1] * TILE_PX + TILE_PX / 2

      const tier = (parent.tier || 'engineer').toLowerCase()
      ctx.strokeStyle = TIER_COLORS[tier] || '#666'
      ctx.setLineDash([4, 4])
      ctx.lineWidth = 1
      ctx.globalAlpha = 0.4
      ctx.beginPath()
      ctx.moveTo(px, py)
      ctx.lineTo(cx, cy)
      ctx.stroke()
      ctx.setLineDash([])
      ctx.globalAlpha = 1
    }
  }

  getWorkerAtPixel(x, y) {
    // Convert pixel coords to tile (with ±1 tolerance)
    const tileX = Math.floor(x / TILE_PX)
    const tileY = Math.floor(y / TILE_PX)

    for (const w of this.workers) {
      const desk = this.workerDeskMap[w.id]
      if (!desk) continue
      const [dx, dy] = desk.charTile
      if (Math.abs(tileX - dx) <= 1 && Math.abs(tileY - dy) <= 1) {
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
  }
}
