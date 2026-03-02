# Expanded Office Map Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the 20×14 pixel office layout to support 19 workers with a meeting room, manager alcoves, consultant/reception area, and break area.

**Architecture:** Three files modified in sequence: `layout.js` (data layer — FLOOR_MAP, DESKS, getZoneTiles), `sprites.js` (art layer — meetingTable + whiteboard sprites), `officeRenderer.js` (render layer — new tile types, floor colors, zone labels). No new files needed.

**Tech Stack:** Vanilla JS canvas 2D, Svelte frontend, existing 16×16 pixel sprite system via FURNITURE_SPRITES.

**Design Reference:** `docs/plans/2026-03-02-expanded-office-map-design.md`

---

### Task 1: Replace FLOOR_MAP in layout.js

**Files:**
- Modify: `frontend/src/lib/office/layout.js:10-27`

**Step 1: Read the current file to understand exact line numbers**

```bash
grep -n "FLOOR_MAP\|const DESKS\|export" frontend/src/lib/office/layout.js
```

**Step 2: Replace the FLOOR_MAP constant**

Replace the entire `const FLOOR_MAP = [...]` block (lines 11–27) with:

```javascript
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
```

Also update the tile comment on line 10 to include the new tiles:

```javascript
// Tile types: 0=floor, 1=wall, 2=desk, 3=plant, 4=computer, 5=watercooler,
//             6=bookshelf, 7=door, 8=glowStrip, 9=cableFloor,
//             10=meetingFloor, 11=whiteboard
```

**Step 3: Verify the map dimensions**

Count rows (should be 14) and each row's length (should be 20). Quick check in browser console:
```javascript
import('/src/lib/office/layout.js').then(m => {
  const map = m.getFloorMap()
  console.log('rows:', map.length, 'cols:', map[0].length)
})
```
Expected: `rows: 14 cols: 20`

**Step 4: Commit**

```bash
git add frontend/src/lib/office/layout.js
git commit -m "feat(layout): redesign FLOOR_MAP with meeting room and 2-tile-wide doors"
```

---

### Task 2: Replace DESKS array in layout.js

**Files:**
- Modify: `frontend/src/lib/office/layout.js:31-56`

**Step 1: Replace the entire DESKS section**

Remove lines 31–56 (the `const DESKS = [...]` block and the two `DESKS[12]=` / `DESKS[13]=` mutation lines). Replace with:

```javascript
// Desk positions — 19 total
// tile: [col, row] of the desk furniture tile
// charTile: [col, row] where the character stands (always one row above desk)
const DESKS = [
  // ── Open Office — Engineers row A (desks at row 2, chars at row 1) ────────
  { id: 'eng-1',  tile: [2,2],   charTile: [2,1],   zone: 'engineer' },
  { id: 'eng-2',  tile: [5,2],   charTile: [5,1],   zone: 'engineer' },
  { id: 'eng-3',  tile: [8,2],   charTile: [8,1],   zone: 'engineer' },
  { id: 'eng-4',  tile: [11,2],  charTile: [11,1],  zone: 'engineer' },
  { id: 'eng-5',  tile: [14,2],  charTile: [14,1],  zone: 'engineer' },
  { id: 'eng-6',  tile: [17,2],  charTile: [17,1],  zone: 'engineer' },
  // ── Open Office — Engineers row B (desks at row 4, chars at row 3) ────────
  { id: 'eng-7',  tile: [2,4],   charTile: [2,3],   zone: 'engineer' },
  { id: 'eng-8',  tile: [5,4],   charTile: [5,3],   zone: 'engineer' },
  { id: 'eng-9',  tile: [8,4],   charTile: [8,3],   zone: 'engineer' },
  { id: 'eng-10', tile: [11,4],  charTile: [11,3],  zone: 'engineer' },
  { id: 'eng-11', tile: [14,4],  charTile: [14,3],  zone: 'engineer' },
  { id: 'eng-12', tile: [17,4],  charTile: [17,3],  zone: 'engineer' },
  // ── Manager Alcoves — semi-private, cols 1-7 ──────────────────────────────
  { id: 'mgr-1',  tile: [2,9],   charTile: [2,8],   zone: 'manager' },
  { id: 'mgr-2',  tile: [5,9],   charTile: [5,8],   zone: 'manager' },
  { id: 'mgr-3',  tile: [2,11],  charTile: [2,10],  zone: 'manager' },
  { id: 'mgr-4',  tile: [5,11],  charTile: [5,10],  zone: 'manager' },
  // ── Consultant Corner ─────────────────────────────────────────────────────
  { id: 'con-1',  tile: [9,12],  charTile: [9,11],  zone: 'consultant' },
  // ── Reception ────────────────────────────────────────────────────────────
  { id: 'rec-1',  tile: [12,12], charTile: [12,11], zone: 'reception' },
  { id: 'rec-2',  tile: [15,12], charTile: [15,11], zone: 'reception' },
]
```

**Step 2: Verify desk count**

```javascript
import('/src/lib/office/layout.js').then(m => {
  console.log('total desks:', m.getDesks().length)
  const zones = m.getDesks().reduce((acc, d) => {
    acc[d.zone] = (acc[d.zone] || 0) + 1; return acc
  }, {})
  console.log('by zone:', zones)
})
```
Expected: `total desks: 19`, `{engineer: 12, manager: 4, consultant: 1, reception: 2}`

**Step 3: Commit**

```bash
git add frontend/src/lib/office/layout.js
git commit -m "feat(layout): add 19-desk DESKS array with engineer/manager/consultant/reception zones"
```

---

### Task 3: Add getZoneTiles() to layout.js

**Files:**
- Modify: `frontend/src/lib/office/layout.js` — add after `export function buildWorkerDeskMap`

**Step 1: Add zone boundaries constant and function**

Insert before the final `export { DESKS, FLOOR_MAP }` line:

```javascript
// Zone boundary rectangles (inclusive)
const ZONE_BOUNDS = {
  openOffice:    { colMin: 0,  colMax: 19, rowMin: 0,  rowMax: 5  },
  meeting:       { colMin: 11, colMax: 15, rowMin: 8,  rowMax: 10 },
  breakArea:     { colMin: 17, colMax: 18, rowMin: 8,  rowMax: 12 },
  managerOffice: { colMin: 0,  colMax: 8,  rowMin: 8,  rowMax: 11 },
  reception:     { colMin: 8,  colMax: 19, rowMin: 11, rowMax: 13 },
}

// Returns [{col, row}, ...] for all non-wall tiles within a named zone.
// Zones: 'meeting', 'breakArea', 'openOffice', 'managerOffice', 'reception'
export function getZoneTiles(zoneName) {
  const bounds = ZONE_BOUNDS[zoneName]
  if (!bounds) return []
  const { colMin, colMax, rowMin, rowMax } = bounds
  const result = []
  for (let row = rowMin; row <= rowMax; row++) {
    for (let col = colMin; col <= colMax; col++) {
      const tile = FLOOR_MAP[row]?.[col]
      if (tile !== undefined && tile !== 1) {  // exclude walls
        result.push({ col, row })
      }
    }
  }
  return result
}
```

**Step 2: Verify the function**

```javascript
import('/src/lib/office/layout.js').then(m => {
  console.log('meeting tiles:', m.getZoneTiles('meeting').length)   // expect 15 (5×3)
  console.log('openOffice:', m.getZoneTiles('openOffice').length > 50)  // expect true
  console.log('unknown zone:', m.getZoneTiles('xyz'))  // expect []
})
```

**Step 3: Commit**

```bash
git add frontend/src/lib/office/layout.js
git commit -m "feat(layout): add getZoneTiles() with zone boundary constants"
```

---

### Task 4: Add meetingTable sprite to sprites.js

**Files:**
- Modify: `frontend/src/lib/office/sprites.js:423-513` — add to FURNITURE_SPRITES

**Step 1: Add `meetingTable` entry to FURNITURE_SPRITES**

Inside the `FURNITURE_SPRITES` object (after the last entry, before the closing `}`), add:

```javascript
  meetingTable: [  // conference table segment — tiles horizontally to form a long table
    '0777777777777770',
    '0788888888888870',
    '07b8888888888b70',
    '0788888888888870',
    '0777777777777770',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0777777777777770',
    '0788888888888870',
    '07b8888888888b70',
    '0788888888888870',
    '0777777777777770',
  ],
```

The sprite shows two parallel table edges (rows 0-4 and rows 11-15) with open space between them (chairs area), creating a top-down conference table section. Consecutive tiles appear as one long table.

**Step 2: Verify the sprite entry exists**

```javascript
import('/src/lib/office/sprites.js').then(m => {
  console.log('meetingTable rows:', m.FURNITURE_SPRITES.meetingTable.length)  // expect 16
  console.log('row 0 length:', m.FURNITURE_SPRITES.meetingTable[0].length)    // expect 16
})
```

**Step 3: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat(sprites): add meetingTable conference table segment sprite"
```

---

### Task 5: Add whiteboard sprite to sprites.js

**Files:**
- Modify: `frontend/src/lib/office/sprites.js` — add to FURNITURE_SPRITES

**Step 1: Add `whiteboard` entry to FURNITURE_SPRITES** (after meetingTable)

```javascript
  whiteboard: [  // wall-mounted whiteboard with decorative writing lines
    '7777777777777777',
    '7eeeeeeeeeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbbbbe7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbbbee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbbe ee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7777777777777777',
    '7000000000000007',
    '7700000000000077',
    '7777777777777777',
  ],
```

Note: `e` = `eyeHi` in COLOR_MAP which resolves to `#fff` via character palette. But FURNITURE_SPRITES use FURNITURE_PALETTE which doesn't have `e`. We need to add `'e'` to FURNITURE_PALETTE as white for the whiteboard face:

In FURNITURE_PALETTE (around line 557), add:
```javascript
  'e': '#e8e8f0',  // whiteboard face (off-white)
```

**Step 2: Verify**

```javascript
import('/src/lib/office/sprites.js').then(m => {
  console.log('whiteboard:', m.FURNITURE_SPRITES.whiteboard.length)  // expect 16
  console.log('palette e:', m.FURNITURE_PALETTE?.e || 'missing — check FURNITURE_PALETTE export')
})
```

Note: `FURNITURE_PALETTE` is not exported. The `renderSpriteToCanvas` function uses it internally. Just verify `FURNITURE_SPRITES.whiteboard` has 16 rows of length 16.

**Step 3: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat(sprites): add whiteboard sprite and off-white palette entry"
```

---

### Task 6: Update officeRenderer.js — new tile types

**Files:**
- Modify: `frontend/src/lib/office/officeRenderer.js:11-35`

**Step 1: Add entries to FLOOR_COLORS**

In the `FLOOR_COLORS` object (around line 11), add after the existing entries:

```javascript
  10: '#141430', // meetingFloor — deep indigo-purple, distinct from regular floor
  11: '#e8e8f0', // whiteboard — off-white face color
```

**Step 2: Add entries to TILE_TO_FURNITURE**

In the `TILE_TO_FURNITURE` object (around line 24), add:

```javascript
  10: 'meetingTable',
  11: 'whiteboard',
```

**Step 3: Prerender new furniture in constructor**

In the constructor at line 84, the furniture prerender loop is:
```javascript
for (const name of ['desk', 'computer', 'plant', 'watercooler', 'bookshelf']) {
```
Add the new sprites:
```javascript
for (const name of ['desk', 'computer', 'plant', 'watercooler', 'bookshelf', 'meetingTable', 'whiteboard']) {
```

**Step 4: Update grid line rendering for new tile types**

In `_drawBackground` around line 144, the grid line condition checks walkable tile types:
```javascript
if (tile === 0 || tile === 7 || tile === 8 || tile === 9 ||
    (tile >= 2 && tile <= 6)) {
```
Update to include tiles 10 and 11:
```javascript
if (tile === 0 || tile === 7 || tile === 8 || tile === 9 ||
    (tile >= 2 && tile <= 6) || tile === 10 || tile === 11) {
```

**Step 5: Commit**

```bash
git add frontend/src/lib/office/officeRenderer.js
git commit -m "feat(renderer): add meetingFloor and whiteboard tile types to FLOOR_COLORS and TILE_TO_FURNITURE"
```

---

### Task 7: Update zone labels in officeRenderer.js

**Files:**
- Modify: `frontend/src/lib/office/officeRenderer.js:183-189` — zone labels in `_drawBackground`

**Step 1: Replace zone label block**

Find the current zone labels section (around line 183):
```javascript
ctx.fillText('OPEN OFFICE', 2 * TILE_PX, 6.5 * TILE_PX)
ctx.fillText('MGR', 2 * TILE_PX, 12.5 * TILE_PX)
ctx.fillText('CON', 2 * TILE_PX, 11 * TILE_PX)
ctx.fillText('REC', 12 * TILE_PX, 12.5 * TILE_PX)
```

Replace with:
```javascript
ctx.fillText('OPEN OFFICE', 2 * TILE_PX, 5.5 * TILE_PX)
ctx.fillText('MGR', 1 * TILE_PX, 11.5 * TILE_PX)
ctx.fillText('MEETING', 11 * TILE_PX, 10.5 * TILE_PX)
ctx.fillText('BREAK', 17 * TILE_PX, 11.5 * TILE_PX)
ctx.fillText('REC', 12 * TILE_PX, 13 * TILE_PX)
```

**Step 2: Commit**

```bash
git add frontend/src/lib/office/officeRenderer.js
git commit -m "feat(renderer): update zone labels for new layout areas"
```

---

### Task 8: Fix pathfinding for new tile types

**Files:**
- Modify: `frontend/src/lib/office/pathfinding.js`

**Step 1: Read pathfinding.js to find walkability check**

```bash
grep -n "walkable\|wall\|tile === 1\|FLOOR_MAP" frontend/src/lib/office/pathfinding.js
```

**Step 2: Ensure tiles 10 and 11 are walkable**

Find where the code determines if a tile is walkable (typically checks `tile !== 1` or a list of walkable tiles). If the check is `tile !== 1` (exclude walls only), tiles 10 and 11 are already walkable — no change needed.

If there's an explicit whitelist of walkable tiles, add 10 and 11 to it.

**Step 3: Verify visually in browser**

Start the dev server:
```bash
cd frontend && npm run dev
```

Navigate to the Pixel Office page. Verify:
- [ ] 20 columns, 14 rows visible (no clipping)
- [ ] Row 6 has 2-tile-wide doors at positions 3-4 and 13-14
- [ ] Meeting room tiles (rows 8-10, cols 11-15) appear in deep purple-indigo color
- [ ] Whiteboard at col 12, row 9 shows the whiteboard sprite
- [ ] Meeting table sprites visible along meeting room rows
- [ ] Watercooler at col 18, row 8
- [ ] Plant at col 18, row 10
- [ ] 12 engineer desks visible in top section (2 rows of 6)
- [ ] 4 manager desks visible in alcoves
- [ ] Zone labels: OPEN OFFICE, MGR, MEETING, BREAK, REC

**Step 4: Commit (if pathfinding changed)**

```bash
git add frontend/src/lib/office/pathfinding.js
git commit -m "fix(pathfinding): ensure meeting floor tiles 10-11 are walkable"
```

---

### Task 9: Verify worker assignment with 19+ workers

**Files:**
- Read: `frontend/src/lib/office/layout.js` — `assignDesksToWorkers`

**Step 1: Check assignDesksToWorkers handles new zones**

The existing function assigns workers to desks by zone priority:
```javascript
assignTier(byTier.consultant, ['consultant', 'manager', 'engineer'])
assignTier(byTier.manager, ['manager', 'engineer'])
assignTier(byTier.engineer, ['engineer', 'manager', 'reception'])
```

This already handles `reception` zone. No change needed unless there are bugs.

**Step 2: Test with 19 workers**

If the app has a way to add test workers, verify all 19 desks get assigned. Alternatively, check the workers store.

**Step 3: Final commit and summary**

```bash
git log --oneline -10
```

Should show all tasks committed. If anything was missed:
```bash
git add -p  # stage specific changes
git commit -m "feat(office): <describe remaining changes>"
```

---

## Verification Checklist

Before marking done:

- [ ] `getFloorMap()` returns 14-row, 20-col array
- [ ] `getDesks()` returns exactly 19 desks (12 eng, 4 mgr, 1 con, 2 rec)
- [ ] `getZoneTiles('meeting')` returns 15 tiles (5 cols × 3 rows)
- [ ] `getZoneTiles('xyz')` returns `[]`
- [ ] FURNITURE_SPRITES has `meetingTable` and `whiteboard` (16 rows each)
- [ ] FLOOR_COLORS has entries for 10 and 11
- [ ] TILE_TO_FURNITURE has entries for 10 and 11
- [ ] Browser: meeting room visible with distinct floor color
- [ ] Browser: whiteboard sprite renders at [12,9]
- [ ] Browser: zone labels show MEETING and BREAK
- [ ] Browser: all worker characters assigned to desks (no unassigned workers)
