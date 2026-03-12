// Office layout — multi-layout system with preset layouts

export const TILE_SIZE = 16
export const SCALE = 3

// ── Standard layout (24×16) — the original ──────────────────────────────────

const STANDARD_FLOOR_MAP = [
  [1,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,3,8],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,0,0,0,0,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,0,14,0,0,14,0,8],
  [8,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,0,0,8],
  [1,1,1,7,7,1,1,1,1,1,1,1,7,7,1,1,1,1,1,1,7,7,1,1],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,12,15,15,8],
  [8,0,2,4,0,2,4,0,0,1,10,10,11,10,10,10,1,0,15,15,15,15,15,8],
  [8,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,15,15,15,8],
  [8,0,2,4,0,2,4,0,0,1,1,1,7,7,1,1,1,0,15,13,15,13,15,8],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,0,0,0,0,0,0,2,4,0,2,4,0,2,4,0,0,5,0,14,0,8],
  [8,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,8],
  [1,8,8,8,7,7,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
]

const STANDARD_DESKS = [
  { id: 'eng-1',  tile: [2,2],   charTile: [3,1],   zone: 'engineer' },
  { id: 'eng-2',  tile: [5,2],   charTile: [6,1],   zone: 'engineer' },
  { id: 'eng-3',  tile: [8,2],   charTile: [9,1],   zone: 'engineer' },
  { id: 'eng-4',  tile: [11,2],  charTile: [12,1],  zone: 'engineer' },
  { id: 'eng-5',  tile: [14,2],  charTile: [15,1],  zone: 'engineer' },
  { id: 'eng-6',  tile: [17,2],  charTile: [18,1],  zone: 'engineer' },
  { id: 'eng-7',  tile: [20,2],  charTile: [21,1],  zone: 'engineer' },
  { id: 'eng-8',  tile: [2,4],   charTile: [3,3],   zone: 'engineer' },
  { id: 'eng-9',  tile: [5,4],   charTile: [6,3],   zone: 'engineer' },
  { id: 'eng-10', tile: [8,4],   charTile: [9,3],   zone: 'engineer' },
  { id: 'eng-11', tile: [11,4],  charTile: [12,3],  zone: 'engineer' },
  { id: 'eng-12', tile: [14,4],  charTile: [15,3],  zone: 'engineer' },
  { id: 'mgr-1',  tile: [2,9],   charTile: [3,8],   zone: 'manager' },
  { id: 'mgr-2',  tile: [5,9],   charTile: [6,8],   zone: 'manager' },
  { id: 'mgr-3',  tile: [2,11],  charTile: [3,10],  zone: 'manager' },
  { id: 'mgr-4',  tile: [5,11],  charTile: [6,10],  zone: 'manager' },
  { id: 'con-1',  tile: [9,13],  charTile: [10,12], zone: 'consultant' },
  { id: 'rec-1',  tile: [12,13], charTile: [13,12], zone: 'reception' },
  { id: 'rec-2',  tile: [15,13], charTile: [16,12], zone: 'reception' },
]

const STANDARD_ZONES = {
  openOffice:    { colMin: 0,  colMax: 23, rowMin: 0,  rowMax: 5  },
  meeting:       { colMin: 10, colMax: 16, rowMin: 8,  rowMax: 11 },
  breakArea:     { colMin: 17, colMax: 23, rowMin: 8,  rowMax: 12 },
  coffeeBar:     { colMin: 18, colMax: 22, rowMin: 8,  rowMax: 10 },
  restArea:      { colMin: 18, colMax: 22, rowMin: 11, rowMax: 12 },
  managerOffice: { colMin: 0,  colMax: 8,  rowMin: 8,  rowMax: 11 },
  reception:     { colMin: 8,  colMax: 23, rowMin: 12, rowMax: 14 },
}

// ── Startup layout (16×12) — compact open space ─────────────────────────────

const STARTUP_FLOOR_MAP = [
  [1,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,0,8],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,9,0,0,9,0,0,0,3,0,0,14,0,8],
  [8,0,2,4,0,2,4,0,0,0,0,0,0,0,0,8],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [1,1,1,7,7,1,1,1,7,7,1,1,1,1,1,1],
  [8,0,0,0,0,0,0,0,0,0,0,15,15,12,0,8],
  [8,0,2,4,0,2,4,0,0,5,0,15,15,15,0,8],
  [8,0,0,0,3,0,0,0,0,0,0,15,13,15,0,8],
  [1,8,8,8,7,7,8,8,8,8,8,8,8,8,8,1],
]

const STARTUP_DESKS = [
  { id: 'eng-1',  tile: [2,2],   charTile: [3,1],   zone: 'engineer' },
  { id: 'eng-2',  tile: [5,2],   charTile: [6,1],   zone: 'engineer' },
  { id: 'eng-3',  tile: [8,2],   charTile: [9,1],   zone: 'engineer' },
  { id: 'eng-4',  tile: [11,2],  charTile: [12,1],  zone: 'engineer' },
  { id: 'eng-5',  tile: [2,5],   charTile: [3,4],   zone: 'engineer' },
  { id: 'eng-6',  tile: [5,5],   charTile: [6,4],   zone: 'engineer' },
  { id: 'mgr-1',  tile: [2,9],   charTile: [3,8],   zone: 'manager' },
  { id: 'mgr-2',  tile: [5,9],   charTile: [6,8],   zone: 'manager' },
]

const STARTUP_ZONES = {
  openOffice:    { colMin: 0,  colMax: 15, rowMin: 0,  rowMax: 6  },
  breakArea:     { colMin: 10, colMax: 14, rowMin: 8,  rowMax: 10 },
  coffeeBar:     { colMin: 11, colMax: 13, rowMin: 8,  rowMax: 9  },
  restArea:      { colMin: 11, colMax: 13, rowMin: 10, rowMax: 10 },
  managerOffice: { colMin: 0,  colMax: 7,  rowMin: 8,  rowMax: 10 },
}

// ── Enterprise layout (28×18) — large office building ───────────────────────

const ENTERPRISE_FLOOR_MAP = [
  [1,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,3,8],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,0,0,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,0,14,0,0,8],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,0,3,0,0,14,0,0,3,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,0,0,0,0,0,0,8],
  [1,1,1,7,7,1,1,1,1,1,1,1,7,7,1,1,1,1,1,1,1,1,7,7,1,1,1,1],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,9,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,12,15,15,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,0,1,10,10,11,10,10,10,1,0,15,15,15,15,15,0,8],
  [8,0,0,9,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,15,15,15,0,8],
  [8,0,2,4,0,2,4,0,2,4,0,0,1,1,1,7,7,1,1,1,0,15,13,15,13,15,0,8],
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  [8,0,0,0,0,2,4,0,2,4,0,2,4,0,2,4,0,0,5,0,0,6,0,0,14,0,0,8],
  [1,8,8,8,7,7,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
]

const ENTERPRISE_DESKS = [
  // Row A — 8 engineer desks
  { id: 'eng-1',  tile: [2,2],   charTile: [3,1],   zone: 'engineer' },
  { id: 'eng-2',  tile: [5,2],   charTile: [6,1],   zone: 'engineer' },
  { id: 'eng-3',  tile: [8,2],   charTile: [9,1],   zone: 'engineer' },
  { id: 'eng-4',  tile: [11,2],  charTile: [12,1],  zone: 'engineer' },
  { id: 'eng-5',  tile: [14,2],  charTile: [15,1],  zone: 'engineer' },
  { id: 'eng-6',  tile: [17,2],  charTile: [18,1],  zone: 'engineer' },
  { id: 'eng-7',  tile: [20,2],  charTile: [21,1],  zone: 'engineer' },
  { id: 'eng-8',  tile: [23,2],  charTile: [24,1],  zone: 'engineer' },
  // Row B — 7 engineer desks
  { id: 'eng-9',  tile: [2,4],   charTile: [3,3],   zone: 'engineer' },
  { id: 'eng-10', tile: [5,4],   charTile: [6,3],   zone: 'engineer' },
  { id: 'eng-11', tile: [8,4],   charTile: [9,3],   zone: 'engineer' },
  { id: 'eng-12', tile: [11,4],  charTile: [12,3],  zone: 'engineer' },
  { id: 'eng-13', tile: [14,4],  charTile: [15,3],  zone: 'engineer' },
  { id: 'eng-14', tile: [17,4],  charTile: [18,3],  zone: 'engineer' },
  { id: 'eng-15', tile: [20,4],  charTile: [21,3],  zone: 'engineer' },
  // Row C — 5 engineer desks
  { id: 'eng-16', tile: [2,7],   charTile: [3,6],   zone: 'engineer' },
  { id: 'eng-17', tile: [5,7],   charTile: [6,6],   zone: 'engineer' },
  { id: 'eng-18', tile: [8,7],   charTile: [9,6],   zone: 'engineer' },
  { id: 'eng-19', tile: [11,7],  charTile: [12,6],  zone: 'engineer' },
  { id: 'eng-20', tile: [14,7],  charTile: [15,6],  zone: 'engineer' },
  // Manager offices — 6 desks
  { id: 'mgr-1',  tile: [2,12],  charTile: [3,11],  zone: 'manager' },
  { id: 'mgr-2',  tile: [5,12],  charTile: [6,11],  zone: 'manager' },
  { id: 'mgr-3',  tile: [8,12],  charTile: [9,11],  zone: 'manager' },
  { id: 'mgr-4',  tile: [2,14],  charTile: [3,13],  zone: 'manager' },
  { id: 'mgr-5',  tile: [5,14],  charTile: [6,13],  zone: 'manager' },
  { id: 'mgr-6',  tile: [8,14],  charTile: [9,13],  zone: 'manager' },
  // Consultant + reception — 5 desks
  { id: 'con-1',  tile: [5,16],  charTile: [6,15],  zone: 'consultant' },
  { id: 'rec-1',  tile: [8,16],  charTile: [9,15],  zone: 'reception' },
  { id: 'rec-2',  tile: [11,16], charTile: [12,15], zone: 'reception' },
  { id: 'rec-3',  tile: [14,16], charTile: [15,15], zone: 'reception' },
]

const ENTERPRISE_ZONES = {
  openOffice:    { colMin: 0,  colMax: 27, rowMin: 0,  rowMax: 8  },
  meeting:       { colMin: 12, colMax: 19, rowMin: 11, rowMax: 14 },
  breakArea:     { colMin: 20, colMax: 26, rowMin: 11, rowMax: 15 },
  coffeeBar:     { colMin: 21, colMax: 25, rowMin: 11, rowMax: 12 },
  restArea:      { colMin: 21, colMax: 25, rowMin: 14, rowMax: 15 },
  managerOffice: { colMin: 0,  colMax: 11, rowMin: 11, rowMax: 14 },
  reception:     { colMin: 0,  colMax: 27, rowMin: 15, rowMax: 16 },
}

// ── Layouts registry ────────────────────────────────────────────────────────

export const OFFICE_LAYOUTS = {
  standard: {
    id: 'standard',
    nameKey: 'office.standard',
    cols: 24,
    rows: 16,
    floorMap: STANDARD_FLOOR_MAP,
    desks: STANDARD_DESKS,
    zoneBounds: STANDARD_ZONES,
  },
  startup: {
    id: 'startup',
    nameKey: 'office.startup',
    cols: 16,
    rows: 12,
    floorMap: STARTUP_FLOOR_MAP,
    desks: STARTUP_DESKS,
    zoneBounds: STARTUP_ZONES,
  },
  enterprise: {
    id: 'enterprise',
    nameKey: 'office.enterprise',
    cols: 28,
    rows: 18,
    floorMap: ENTERPRISE_FLOOR_MAP,
    desks: ENTERPRISE_DESKS,
    zoneBounds: ENTERPRISE_ZONES,
  },
}

// ── Layout state ────────────────────────────────────────────────────────────

const LAYOUT_STORAGE_KEY = 'pixelOffice_layoutId'
let _currentLayoutId = null

export function getCurrentLayoutId() {
  if (!_currentLayoutId) {
    _currentLayoutId = localStorage.getItem(LAYOUT_STORAGE_KEY) || 'standard'
  }
  return _currentLayoutId
}

export function setCurrentLayoutId(id) {
  if (!OFFICE_LAYOUTS[id]) return
  _currentLayoutId = id
  localStorage.setItem(LAYOUT_STORAGE_KEY, id)
}

function currentLayout() {
  return OFFICE_LAYOUTS[getCurrentLayoutId()]
}

// ── Backward-compatible exports ─────────────────────────────────────────────
// These now delegate to the current layout

export const COLS = 24   // default, but use getLayoutDimensions() for dynamic
export const ROWS = 16
export const CANVAS_W = COLS * TILE_SIZE * SCALE
export const CANVAS_H = ROWS * TILE_SIZE * SCALE

export function getLayoutDimensions(layoutId) {
  const layout = OFFICE_LAYOUTS[layoutId || getCurrentLayoutId()]
  return {
    cols: layout.cols,
    rows: layout.rows,
    canvasW: layout.cols * TILE_SIZE * SCALE,
    canvasH: layout.rows * TILE_SIZE * SCALE,
  }
}

export function getDesks(layoutId) {
  const layout = OFFICE_LAYOUTS[layoutId || getCurrentLayoutId()]
  return layout.desks
}

export function getFloorMap(layoutId) {
  const layout = OFFICE_LAYOUTS[layoutId || getCurrentLayoutId()]
  return layout.floorMap
}

// Keep old named exports for backward compat
const FLOOR_MAP = STANDARD_FLOOR_MAP
const DESKS = STANDARD_DESKS

// ── Desk assignment ─────────────────────────────────────────────────────────

const STORAGE_KEY = 'pixelOffice_deskAssignments'

export function assignDesksToWorkers(workers, relationships = null, layoutId = null) {
  const desks = getDesks(layoutId)

  // Try to restore from localStorage
  const saved = localStorage.getItem(STORAGE_KEY)
  let assignments = saved ? JSON.parse(saved) : {}

  // Remove assignments for workers that no longer exist
  const workerIds = new Set(workers.map(w => w.id))
  for (const deskId of Object.keys(assignments)) {
    if (!workerIds.has(assignments[deskId])) {
      delete assignments[deskId]
    }
  }

  // Remove assignments for desks that don't exist in current layout
  const deskIds = new Set(desks.map(d => d.id))
  for (const deskId of Object.keys(assignments)) {
    if (!deskIds.has(deskId)) {
      delete assignments[deskId]
    }
  }

  // Categorize workers by tier
  const byTier = { consultant: [], manager: [], engineer: [] }
  for (const w of workers) {
    const tier = (w.tier || 'engineer').toLowerCase()
    if (byTier[tier]) byTier[tier].push(w)
    else byTier.engineer.push(w)
  }

  if (relationships && byTier.engineer.length > 1) {
    byTier.engineer = _sortByAffinity(byTier.engineer, relationships)
  }

  const assignedWorkers = new Set(Object.values(assignments))
  const desksByZone = {}
  for (const d of desks) {
    if (!desksByZone[d.zone]) desksByZone[d.zone] = []
    desksByZone[d.zone].push(d)
  }

  function assignTier(tierWorkers, zones) {
    for (const w of tierWorkers) {
      if (assignedWorkers.has(w.id)) continue
      for (const zone of zones) {
        const available = (desksByZone[zone] || []).find(d => !assignments[d.id])
        if (available) {
          assignments[available.id] = w.id
          assignedWorkers.add(w.id)
          break
        }
      }
    }
  }

  assignTier(byTier.consultant, ['consultant', 'manager', 'engineer'])
  assignTier(byTier.manager, ['manager', 'engineer'])
  assignTier(byTier.engineer, ['engineer', 'manager', 'reception'])

  localStorage.setItem(STORAGE_KEY, JSON.stringify(assignments))
  return assignments
}

function _sortByAffinity(engineers, relationships) {
  if (!relationships?.length) return engineers

  const affinityMap = new Map()
  for (const r of relationships) {
    const key = r.workerA < r.workerB ? `${r.workerA}-${r.workerB}` : `${r.workerB}-${r.workerA}`
    affinityMap.set(key, r.affinity || 50)
  }

  function getAffinity(idA, idB) {
    const key = idA < idB ? `${idA}-${idB}` : `${idB}-${idA}`
    return affinityMap.get(key) ?? 50
  }

  const result = []
  const remaining = new Set(engineers.map((_, i) => i))
  let current = 0
  remaining.delete(0)
  result.push(engineers[0])

  while (remaining.size > 0) {
    let bestIdx = -1
    let bestAff = -1
    for (const idx of remaining) {
      const aff = getAffinity(engineers[current].id, engineers[idx].id)
      if (aff > bestAff) {
        bestAff = aff
        bestIdx = idx
      }
    }
    remaining.delete(bestIdx)
    result.push(engineers[bestIdx])
    current = bestIdx
  }

  return result
}

// Build a lookup: workerId → desk object
export function buildWorkerDeskMap(assignments, layoutId = null) {
  const desks = getDesks(layoutId)
  const map = {}
  for (const desk of desks) {
    const workerId = assignments[desk.id]
    if (workerId) {
      map[workerId] = desk
    }
  }
  return map
}

// Returns [{col, row}, ...] for all non-wall tiles within a named zone.
export function getZoneTiles(zoneName, layoutId = null) {
  const layout = OFFICE_LAYOUTS[layoutId || getCurrentLayoutId()]
  const bounds = layout.zoneBounds[zoneName]
  if (!bounds) return []
  const { colMin, colMax, rowMin, rowMax } = bounds
  const floorMap = layout.floorMap
  const result = []
  for (let row = rowMin; row <= rowMax; row++) {
    for (let col = colMin; col <= colMax; col++) {
      const tile = floorMap[row]?.[col]
      if (tile !== undefined && tile !== 1) {
        result.push({ col, row })
      }
    }
  }
  return result
}

// Clear desk assignments (used when switching layouts)
export function clearDeskAssignments() {
  localStorage.removeItem(STORAGE_KEY)
}

export { DESKS, FLOOR_MAP }
