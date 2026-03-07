// Office layout — 24×16 tile grid, scale 3× (1152×768px)

export const TILE_SIZE = 16
export const SCALE = 3
export const COLS = 24
export const ROWS = 16
export const CANVAS_W = COLS * TILE_SIZE * SCALE  // 1152
export const CANVAS_H = ROWS * TILE_SIZE * SCALE  // 768

// Tile types: 0=floor, 1=wall, 2=desk, 3=plant, 4=computer, 5=watercooler,
//             6=bookshelf, 7=door, 8=baseboard, 9=rugPattern,
//             10=meetingFloor, 11=whiteboard,
//             12=coffeeBar, 13=sofa, 14=largePlant, 15=coffeeFloor
const FLOOR_MAP = [
  // row 0: top wall + baseboard
  [1,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
  // row 1: engineer walkway (charTile for row-2 desks)
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,8],
  // row 2: 7 engineer desks (row A) + 1 plant decoration
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,3,8],
  // row 3: engineer walkway (charTile for row-4 desks)
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,0,0,0,0,0,8],
  // row 4: 5 engineer desks (row B)
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,0,14,0,0,14,0,8],
  // row 5: central corridor + plants
  [8,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,0,0,8],
  // row 6: divider wall + 3 doorways (cols 3-4, 12-13, 20-21)
  [1,1,1,7,7,1,1,1,1,1,1,1,7,7,1,1,1,1,1,1,7,7,1,1],
  // row 7: main hallway
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  // row 8: left=mgr charTile | mid=meeting room | right=coffee bar area
  [8,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,12,15,15,8],
  // row 9: mgr desks | meeting floor + whiteboard | coffee floor
  [8,0,2,4,0,2,4,0,0,1,10,10,11,10,10,10,1,0,15,15,15,15,15,8],
  // row 10: mgr charTile | meeting floor | coffee floor
  [8,0,0,9,0,0,9,0,0,1,10,10,10,10,10,10,1,0,15,15,15,15,15,8],
  // row 11: mgr desks | meeting wall + door | sofa/rest area
  [8,0,2,4,0,2,4,0,0,1,1,1,7,7,1,1,1,0,15,13,15,13,15,8],
  // row 12: connecting corridor + meeting door area
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  // row 13: consultant desk + reception desks + watercooler
  [8,0,0,0,0,0,0,0,0,2,4,0,2,4,0,2,4,0,0,5,0,14,0,8],
  // row 14: bottom corridor + plants
  [8,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,8],
  // row 15: bottom wall + entrance door (cols 4-5)
  [1,8,8,8,7,7,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
]

// Desk positions — 19 total
// tile: [col, row] of the desk furniture tile (tile type 2)
// charTile: [col, row] where the character stands (one row above desk)
const DESKS = [
  // ── Open Office — Engineers row A (desks at row 2, chars at row 1) ─────
  { id: 'eng-1',  tile: [2,2],   charTile: [2,1],   zone: 'engineer' },
  { id: 'eng-2',  tile: [5,2],   charTile: [5,1],   zone: 'engineer' },
  { id: 'eng-3',  tile: [8,2],   charTile: [8,1],   zone: 'engineer' },
  { id: 'eng-4',  tile: [11,2],  charTile: [11,1],  zone: 'engineer' },
  { id: 'eng-5',  tile: [14,2],  charTile: [14,1],  zone: 'engineer' },
  { id: 'eng-6',  tile: [17,2],  charTile: [17,1],  zone: 'engineer' },
  { id: 'eng-7',  tile: [20,2],  charTile: [20,1],  zone: 'engineer' },
  // ── Open Office — Engineers row B (desks at row 4, chars at row 3) ─────
  { id: 'eng-8',  tile: [2,4],   charTile: [2,3],   zone: 'engineer' },
  { id: 'eng-9',  tile: [5,4],   charTile: [5,3],   zone: 'engineer' },
  { id: 'eng-10', tile: [8,4],   charTile: [8,3],   zone: 'engineer' },
  { id: 'eng-11', tile: [11,4],  charTile: [11,3],  zone: 'engineer' },
  { id: 'eng-12', tile: [14,4],  charTile: [14,3],  zone: 'engineer' },
  // ── Manager Alcoves — semi-private, cols 1-7 ───────────────────────────
  { id: 'mgr-1',  tile: [2,9],   charTile: [2,8],   zone: 'manager' },
  { id: 'mgr-2',  tile: [5,9],   charTile: [5,8],   zone: 'manager' },
  { id: 'mgr-3',  tile: [2,11],  charTile: [2,10],  zone: 'manager' },
  { id: 'mgr-4',  tile: [5,11],  charTile: [5,10],  zone: 'manager' },
  // ── Consultant Corner ──────────────────────────────────────────────────
  { id: 'con-1',  tile: [9,13],  charTile: [9,12],  zone: 'consultant' },
  // ── Reception ─────────────────────────────────────────────────────────
  { id: 'rec-1',  tile: [12,13], charTile: [12,12], zone: 'reception' },
  { id: 'rec-2',  tile: [15,13], charTile: [15,12], zone: 'reception' },
]

export function getDesks() {
  return DESKS
}

export function getFloorMap() {
  return FLOOR_MAP
}

// Assign workers to desks based on tier
// Persists to localStorage
const STORAGE_KEY = 'pixelOffice_deskAssignments'

export function assignDesksToWorkers(workers, relationships = null) {
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

  // Categorize workers by tier
  const byTier = { consultant: [], manager: [], engineer: [] }
  for (const w of workers) {
    const tier = (w.tier || 'engineer').toLowerCase()
    if (byTier[tier]) byTier[tier].push(w)
    else byTier.engineer.push(w)
  }

  // Sort engineers by relationship affinity if relationships are provided
  // Workers with high mutual affinity get assigned sequentially → adjacent desks
  if (relationships && byTier.engineer.length > 1) {
    byTier.engineer = _sortByAffinity(byTier.engineer, relationships)
  }

  // Get available desks per zone
  const assignedWorkers = new Set(Object.values(assignments))
  const desksByZone = {}
  for (const d of DESKS) {
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

// Sort workers so high-affinity pairs end up adjacent in the array
// (which maps to adjacent desks since desks are assigned sequentially)
function _sortByAffinity(engineers, relationships) {
  if (!relationships?.length) return engineers

  // Build affinity lookup
  const affinityMap = new Map()
  for (const r of relationships) {
    const key = r.workerA < r.workerB ? `${r.workerA}-${r.workerB}` : `${r.workerB}-${r.workerA}`
    affinityMap.set(key, r.affinity || 50)
  }

  function getAffinity(idA, idB) {
    const key = idA < idB ? `${idA}-${idB}` : `${idB}-${idA}`
    return affinityMap.get(key) ?? 50
  }

  // Greedy nearest-neighbor: start with first, pick highest affinity next
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
export function buildWorkerDeskMap(assignments) {
  const map = {}
  for (const desk of DESKS) {
    const workerId = assignments[desk.id]
    if (workerId) {
      map[workerId] = desk
    }
  }
  return map
}


// Zone boundary rectangles (inclusive col/row ranges)
const ZONE_BOUNDS = {
  openOffice:    { colMin: 0,  colMax: 23, rowMin: 0,  rowMax: 5  },
  meeting:       { colMin: 10, colMax: 16, rowMin: 8,  rowMax: 11 },
  breakArea:     { colMin: 17, colMax: 23, rowMin: 8,  rowMax: 12 },
  coffeeBar:     { colMin: 18, colMax: 22, rowMin: 8,  rowMax: 10 },
  restArea:      { colMin: 18, colMax: 22, rowMin: 11, rowMax: 12 },
  managerOffice: { colMin: 0,  colMax: 8,  rowMin: 8,  rowMax: 11 },
  reception:     { colMin: 8,  colMax: 23, rowMin: 12, rowMax: 14 },
}

// Returns [{col, row}, ...] for all non-wall tiles within a named zone.
export function getZoneTiles(zoneName) {
  const bounds = ZONE_BOUNDS[zoneName]
  if (!bounds) return []
  const { colMin, colMax, rowMin, rowMax } = bounds
  const result = []
  for (let row = rowMin; row <= rowMax; row++) {
    for (let col = colMin; col <= colMax; col++) {
      const tile = FLOOR_MAP[row]?.[col]
      if (tile !== undefined && tile !== 1) {
        result.push({ col, row })
      }
    }
  }
  return result
}

export { DESKS, FLOOR_MAP }
