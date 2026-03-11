// A* pathfinding engine for the Pixel Office tile grid
import { getFloorMap, getLayoutDimensions, getCurrentLayoutId } from './layout.js'

const WALKABLE_TILES = new Set([0, 7, 9, 10, 11, 15]) // floor, door, rugPattern, meetingRoomFloor, whiteboard, coffeeFloor

let grid = null // cached walkability grid (gridRows × gridCols booleans)
let gridCols = 0
let gridRows = 0

function ensureGrid() {
  if (grid) return grid
  const dims = getLayoutDimensions()
  gridCols = dims.cols
  gridRows = dims.rows
  const map = getFloorMap()
  grid = new Array(gridRows)
  for (let r = 0; r < gridRows; r++) {
    grid[r] = new Array(gridCols)
    for (let c = 0; c < gridCols; c++) {
      grid[r][c] = WALKABLE_TILES.has(map[r][c])
    }
  }
  return grid
}

export function rebuildGrid() {
  grid = null
}

export function isWalkable(col, row) {
  ensureGrid()
  if (col < 0 || col >= gridCols || row < 0 || row >= gridRows) return false
  return grid[row][col]
}

const DIRS = [
  { dc: 0, dr: -1 }, // up
  { dc: 1, dr: 0 },  // right
  { dc: 0, dr: 1 },  // down
  { dc: -1, dr: 0 }, // left
]

export function getAdjacentWalkable(col, row) {
  ensureGrid()
  const neighbors = []
  for (const { dc, dr } of DIRS) {
    const nc = col + dc
    const nr = row + dr
    if (nc >= 0 && nc < gridCols && nr >= 0 && nr < gridRows && grid[nr][nc]) {
      neighbors.push({ col: nc, row: nr })
    }
  }
  return neighbors
}

export function findNearestWalkable(col, row) {
  ensureGrid()
  if (col >= 0 && col < gridCols && row >= 0 && row < gridRows && grid[row][col]) {
    return { col, row }
  }
  // BFS outward
  const visited = new Set()
  const key = (c, r) => r * gridCols + c
  const queue = [{ col, row }]
  visited.add(key(col, row))

  while (queue.length > 0) {
    const cur = queue.shift()
    for (const { dc, dr } of DIRS) {
      const nc = cur.col + dc
      const nr = cur.row + dr
      if (nc < 0 || nc >= gridCols || nr < 0 || nr >= gridRows) continue
      const k = key(nc, nr)
      if (visited.has(k)) continue
      visited.add(k)
      if (grid[nr][nc]) return { col: nc, row: nr }
      queue.push({ col: nc, row: nr })
    }
  }
  return null
}

// A* with Manhattan heuristic, 4-directional movement
export function findPath(startCol, startRow, endCol, endRow) {
  ensureGrid()

  if (!isWalkable(startCol, startRow) || !isWalkable(endCol, endRow)) return []
  if (startCol === endCol && startRow === endRow) return [{ col: startCol, row: startRow }]

  const key = (c, r) => r * gridCols + c
  const startKey = key(startCol, startRow)
  const endKey = key(endCol, endRow)

  const gScore = new Map()
  const fScore = new Map()
  const cameFrom = new Map()
  gScore.set(startKey, 0)
  fScore.set(startKey, Math.abs(endCol - startCol) + Math.abs(endRow - startRow))

  // Min-heap via sorted insertion (sufficient for a 24×16 grid)
  const open = [{ key: startKey, col: startCol, row: startRow }]
  const inOpen = new Set([startKey])
  const closed = new Set()

  while (open.length > 0) {
    // Pop node with lowest fScore
    let bestIdx = 0
    let bestF = fScore.get(open[0].key)
    for (let i = 1; i < open.length; i++) {
      const f = fScore.get(open[i].key)
      if (f < bestF) { bestF = f; bestIdx = i }
    }
    const cur = open[bestIdx]
    open[bestIdx] = open[open.length - 1]
    open.pop()
    inOpen.delete(cur.key)

    if (cur.key === endKey) {
      // Reconstruct path
      const path = []
      let k = endKey
      while (k !== undefined) {
        const r = (k / gridCols) | 0
        const c = k % gridCols
        path.push({ col: c, row: r })
        k = cameFrom.get(k)
      }
      path.reverse()
      return path
    }

    closed.add(cur.key)

    for (const { dc, dr } of DIRS) {
      const nc = cur.col + dc
      const nr = cur.row + dr
      if (nc < 0 || nc >= gridCols || nr < 0 || nr >= gridRows) continue
      if (!grid[nr][nc]) continue
      const nk = key(nc, nr)
      if (closed.has(nk)) continue

      const tentG = gScore.get(cur.key) + 1
      if (tentG < (gScore.get(nk) ?? Infinity)) {
        cameFrom.set(nk, cur.key)
        gScore.set(nk, tentG)
        fScore.set(nk, tentG + Math.abs(endCol - nc) + Math.abs(endRow - nr))
        if (!inOpen.has(nk)) {
          open.push({ key: nk, col: nc, row: nr })
          inOpen.add(nk)
        }
      }
    }
  }

  return [] // no path found
}
