// Office layout — 20×14 tile grid, scale 3× (960×672px)

export const TILE_SIZE = 16
export const SCALE = 3
export const COLS = 20
export const ROWS = 14
export const CANVAS_W = COLS * TILE_SIZE * SCALE  // 960
export const CANVAS_H = ROWS * TILE_SIZE * SCALE  // 672

// Tile types: 0=floor, 1=wall, 2=desk, 3=plant, 4=computer, 5=watercooler,
//             6=bookshelf, 7=door, 8=glowStrip, 9=cableFloor,
//             10=meetingFloor, 11=whiteboard
const FLOOR_MAP = [
  // row 0: top glowStrip wall
  [1,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
  // row 1: cable floor (char positions for row-2 desks)
  [8,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,8],
  // row 2: 6 engineer desks (row A)
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,8],
  // row 3: cable spacing (char positions for row-4 desks)
  [8,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,0,0,9,8],
  // row 4: 6 engineer desks (row B)
  [8,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,0,2,4,8],
  // row 5: central open walkway
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  // row 6: horizontal wall — 2-tile-wide doors at cols 3-4 and 13-14
  [1,1,1,7,7,1,1,1,1,1,1,1,1,7,7,1,1,1,1,1],
  // row 7: main bottom walkway
  [8,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8],
  // row 8: mgr char row | meeting room left wall | meeting floor | meeting right wall | watercooler
  [8,0,0,9,0,0,9,0,0,0,1,10,10,10,10,10,1,0,5,8],
  // row 9: mgr desks | meeting floor | whiteboard at col 12
  [8,0,2,4,0,2,4,0,0,0,1,10,11,10,10,10,1,0,0,8],
  // row 10: mgr char row | meeting floor | plant
  [8,0,0,9,0,0,9,0,0,0,1,10,10,10,10,10,1,0,3,8],
  // row 11: mgr desks | open break area
  [8,0,2,4,0,2,4,0,0,0,1,0,0,0,0,0,1,0,0,8],
  // row 12: walkway + consultant desk + reception desks
  [8,0,0,0,0,0,0,0,0,2,4,0,2,4,0,2,4,0,0,8],
  // row 13: bottom wall — 2-tile entrance at cols 4-5
  [1,8,8,8,7,7,8,8,8,8,8,8,8,8,8,8,8,8,8,1],
]

// Desk positions with zone labels
// Each desk has a tile position and a character standing position (offset)
const DESKS = [
  // Open office — engineers (top-left quadrant)
  { id: 'eng-1', tile: [2,2],  charTile: [2,1],  zone: 'engineer' },
  { id: 'eng-2', tile: [5,2],  charTile: [5,1],  zone: 'engineer' },
  { id: 'eng-3', tile: [8,2],  charTile: [8,1],  zone: 'engineer' },
  { id: 'eng-4', tile: [2,4],  charTile: [2,3],  zone: 'engineer' },
  { id: 'eng-5', tile: [5,4],  charTile: [5,3],  zone: 'engineer' },
  { id: 'eng-6', tile: [8,4],  charTile: [8,3],  zone: 'engineer' },
  // Open office — engineers (top-right quadrant)
  { id: 'eng-7', tile: [12,2], charTile: [12,1], zone: 'engineer' },
  { id: 'eng-8', tile: [15,2], charTile: [15,1], zone: 'engineer' },
  { id: 'eng-9', tile: [12,4], charTile: [12,3], zone: 'engineer' },
  { id: 'eng-10',tile: [15,4], charTile: [15,3], zone: 'engineer' },
  // Manager offices (bottom-left)
  { id: 'mgr-1', tile: [4,9],  charTile: [4,8],  zone: 'manager' },
  { id: 'mgr-2', tile: [13,9], charTile: [13,8], zone: 'manager' },
  // Consultant corner (bottom-left room)
  { id: 'con-1', tile: [4,9],  charTile: [3,8],  zone: 'consultant' },
  // Reception
  { id: 'rec-1', tile: [13,9], charTile: [14,8], zone: 'reception' },
]

// Remove duplicate tile positions — consultant shares with manager, offset differently
// Actually let's fix: consultant gets a unique desk
DESKS[12] = { id: 'con-1', tile: [4,10], charTile: [3,10], zone: 'consultant' }
DESKS[13] = { id: 'rec-1', tile: [13,10], charTile: [14,10], zone: 'reception' }

export function getDesks() {
  return DESKS
}

export function getFloorMap() {
  return FLOOR_MAP
}

// Assign workers to desks based on tier
// Persists to localStorage
const STORAGE_KEY = 'pixelOffice_deskAssignments'

export function assignDesksToWorkers(workers) {
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

export { DESKS, FLOOR_MAP }
