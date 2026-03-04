// Character movement controller for pixel office
// Speed: 3 tiles/sec; TILE_PX = TILE_SIZE(16) * SCALE(3) = 48px

import { findPath, isWalkable, getAdjacentWalkable, findNearestWalkable } from './pathfinding.js'
import { TILE_SIZE, SCALE } from './layout.js'

export const TILE_PX = TILE_SIZE * SCALE                // 48
const BASE_SPEED = (3 * TILE_PX) / 1000                 // 0.144 px/ms

function tileCenter(col, row) {
  return { x: col * TILE_PX + TILE_PX / 2, y: row * TILE_PX + TILE_PX / 2 }
}

// ── CharacterPosition ────────────────────────────────────────────────────────

export class CharacterPosition {
  constructor(col, row) {
    const { x, y } = tileCenter(col, row)
    this.deskCol = col
    this.deskRow = row
    this.currentCol = col
    this.currentRow = row
    this.pixelX = x
    this.pixelY = y
    this.targetPixelX = x
    this.targetPixelY = y
    this.path = []          // [{col, row}, …] remaining steps
    this.direction = null   // 'up'|'down'|'left'|'right'
    this.isMoving = false
  }
}

// ── MovementController ───────────────────────────────────────────────────────

// Mood → speed multiplier mapping
export const MOOD_SPEED = {
  excited:    1.4,
  happy:      1.15,
  neutral:    1.0,
  stressed:   1.1,
  frustrated: 0.8,
  tired:      0.65,
}

export class MovementController {
  constructor() {
    this._positions = new Map() // workerId → CharacterPosition
    this._speedMultipliers = new Map() // workerId → number
  }

  setSpeedMultiplier(workerId, mult) {
    this._speedMultipliers.set(workerId, mult)
  }

  registerWorker(workerId, col, row) {
    this._positions.set(workerId, new CharacterPosition(col, row))
  }

  removeWorker(workerId) {
    this._positions.delete(workerId)
  }

  startMovement(workerId, destCol, destRow) {
    const pos = this._positions.get(workerId)
    if (!pos) return

    const path = findPath(pos.currentCol, pos.currentRow, destCol, destRow)
    if (!path || path.length < 2) return   // already there or unreachable

    // path[0] is current tile; walk from path[1] onward
    pos.path = path.slice(1)
    pos.isMoving = true
    this._advanceToNext(pos)
  }

  startMovementToWorker(workerId, targetWorkerId) {
    const pos = this._positions.get(workerId)
    const target = this._positions.get(targetWorkerId)
    if (!pos || !target) return

    const neighbors = getAdjacentWalkable(target.currentCol, target.currentRow)
    if (!neighbors.length) return

    // Pick nearest neighbor to pos
    let best = null, bestDist = Infinity
    for (const n of neighbors) {
      const d = Math.abs(n.col - pos.currentCol) + Math.abs(n.row - pos.currentRow)
      if (d < bestDist) { bestDist = d; best = n }
    }
    if (best) this.startMovement(workerId, best.col, best.row)
  }

  returnToDesk(workerId) {
    const pos = this._positions.get(workerId)
    if (!pos) return

    let destCol = pos.deskCol, destRow = pos.deskRow
    if (!isWalkable(destCol, destRow)) {
      const nearest = findNearestWalkable(destCol, destRow)
      if (!nearest) return
      destCol = nearest.col
      destRow = nearest.row
    }
    this.startMovement(workerId, destCol, destRow)
  }

  update(deltaMs) {
    for (const [workerId, pos] of this._positions) {
      if (!pos.isMoving) continue

      const dx = pos.targetPixelX - pos.pixelX
      const dy = pos.targetPixelY - pos.pixelY
      const dist = Math.sqrt(dx * dx + dy * dy)
      const multiplier = this._speedMultipliers.get(workerId) ?? 1.0
      const step = BASE_SPEED * multiplier * deltaMs

      if (dist <= 1 || step >= dist) {
        // Snap to tile center
        pos.pixelX = pos.targetPixelX
        pos.pixelY = pos.targetPixelY
        pos.currentCol = Math.floor(pos.pixelX / TILE_PX)
        pos.currentRow = Math.floor(pos.pixelY / TILE_PX)

        if (pos.path.length > 0) {
          this._advanceToNext(pos)
        } else {
          pos.isMoving = false
          pos.direction = null
        }
      } else {
        const ratio = step / dist
        pos.pixelX += dx * ratio
        pos.pixelY += dy * ratio
      }
    }
  }

  getPosition(workerId) {
    return this._positions.get(workerId) ?? null
  }

  isMoving(workerId) {
    return this._positions.get(workerId)?.isMoving ?? false
  }

  // Returns 'walkDown'|'walkUp'|'walkLeft'|'walkRight' while moving, null otherwise.
  getWalkAnimation(workerId) {
    const pos = this._positions.get(workerId)
    if (!pos?.isMoving || !pos.direction) return null
    return { down: 'walkDown', up: 'walkUp', left: 'walkLeft', right: 'walkRight' }[pos.direction] ?? null
  }

  // ── private ──────────────────────────────────────────────────────────────

  _advanceToNext(pos) {
    const next = pos.path.shift()
    const { x, y } = tileCenter(next.col, next.row)

    const dx = x - pos.pixelX
    const dy = y - pos.pixelY
    pos.direction = Math.abs(dx) >= Math.abs(dy)
      ? (dx > 0 ? 'right' : 'left')
      : (dy > 0 ? 'down'  : 'up')

    pos.targetPixelX = x
    pos.targetPixelY = y
  }
}
