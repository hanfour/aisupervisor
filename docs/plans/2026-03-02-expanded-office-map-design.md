# Expanded Office Map Design

**Date:** 2026-03-02
**Branch:** ai/p1772437859967-1/t1772437944548-1-expanded-office-map-with-meeti
**Status:** Approved

## Overview

Redesign the 20×14 pixel office layout to support 19+ workers with a meeting room, manager alcoves, consultant corner, and break area — while maintaining the hacker-base aesthetic.

## FLOOR_MAP (20×14)

```
     0  1  2  3  4  5  6  7  8  9  10 11 12 13 14 15 16 17 18 19
R0:  1  8  8  8  8  8  8  8  8  8  8  8  8  8  8  8  8  8  8  1   top glowStrip wall
R1:  8  0  9  0  0  9  0  0  9  0  0  9  0  0  9  0  0  9  0  8   cable floor
R2:  8  0  2  4  0  2  4  0  2  4  0  2  4  0  2  4  0  2  4  8   6 eng desks (row A)
R3:  8  0  0  9  0  0  9  0  0  9  0  0  9  0  0  9  0  0  9  8   cable spacing
R4:  8  0  2  4  0  2  4  0  2  4  0  2  4  0  2  4  0  2  4  8   6 eng desks (row B)
R5:  8  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  8   central walkway
R6:  1  1  1  7  7  1  1  1  1  1  1  1  1  7  7  1  1  1  1  1   horizontal wall + 2-tile doors x2
R7:  8  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  8   main bottom walkway
R8:  8  0  0  9  0  0  9  0  0  0  1  10 10 10 10 10 1  0  5  8   mgr chars | meeting room | watercooler
R9:  8  0  2  4  0  2  4  0  0  0  1  10 11 10 10 10 1  0  0  8   mgr desks  | whiteboard
R10: 8  0  0  9  0  0  9  0  0  0  1  10 10 10 10 10 1  0  3  8   mgr chars  | meeting room | plant
R11: 8  0  2  4  0  2  4  0  0  0  1  0  0  0  0  0  1  0  0  8   mgr desks  | open break area
R12: 8  0  0  0  0  0  0  0  0  2  4  0  2  4  0  2  4  0  0  8   con + reception desks
R13: 1  8  8  8  7  7  8  8  8  8  8  8  8  8  8  8  8  8  8  1   bottom wall + 2-tile entrance
```

### New Tile Types

| Tile | Name | Description |
|------|------|-------------|
| 10 | meetingFloor | Meeting room floor (deep purple-indigo `#141430`) |
| 11 | whiteboard | Wall-mounted whiteboard (`#e8e8f0`) |

### Door Changes

All doors are **2 tiles wide** (removed single-tile door bottlenecks):
- Row 6, cols 3-4: door to open office (left passage)
- Row 6, cols 13-14: door to open office (right passage)
- Row 13, cols 4-5: main entrance

## DESKS Array (19 desks)

### Engineers (12, zone: 'engineer')

| ID | tile [col,row] | charTile [col,row] |
|----|---------------|-------------------|
| eng-1 | [2,2] | [2,1] |
| eng-2 | [5,2] | [5,1] |
| eng-3 | [8,2] | [8,1] |
| eng-4 | [11,2] | [11,1] |
| eng-5 | [14,2] | [14,1] |
| eng-6 | [17,2] | [17,1] |
| eng-7 | [2,4] | [2,3] |
| eng-8 | [5,4] | [5,3] |
| eng-9 | [8,4] | [8,3] |
| eng-10 | [11,4] | [11,3] |
| eng-11 | [14,4] | [14,3] |
| eng-12 | [17,4] | [17,3] |

### Managers (4, zone: 'manager') — semi-private alcoves cols 1-7

| ID | tile | charTile | Notes |
|----|------|---------|-------|
| mgr-1 | [2,9] | [2,8] | upper-left alcove |
| mgr-2 | [5,9] | [5,8] | upper-right alcove |
| mgr-3 | [2,11] | [2,10] | lower-left alcove |
| mgr-4 | [5,11] | [5,10] | lower-right alcove |

### Consultant (1, zone: 'consultant')

| ID | tile | charTile |
|----|------|---------|
| con-1 | [9,12] | [9,11] |

### Reception (2, zone: 'reception')

| ID | tile | charTile |
|----|------|---------|
| rec-1 | [12,12] | [12,11] |
| rec-2 | [15,12] | [15,11] |

## Pathfinding Verification

All charTile positions are reachable from row 7 (main walkway):
- **eng charTiles (rows 1,3):** accessible via row 5 walkway → row 6 doors → row 7
- **mgr-1,2 (row 8):** one step south from row 7
- **mgr-3,4 (row 10):** row 7 → [1 or 4, 8] → south through cols 1/4 → row 10
- **con-1, rec-1,2 (row 11):** row 7 → col 8/9 → south to row 11/12

## New Furniture Sprites

### meetingTable (16×16, tileable horizontally)

Design: dark metal/wood conference table segment. Left/right edges have table-side bevels; center tiles show flat surface. When 5 tiles placed side by side (cols 11-15), they visually form one long conference table. Used for rows 8 and 10 of meeting room.

Palette reuses `FURNITURE_PALETTE`: `7`=frame, `8`=dark surface, `b`=cyan accent edge.

### whiteboard (16×16)

Design: wall-mounted display board. Light face (`#e8e8f0`) with dark frame (`#2a2a3a`). Decorative "writing" lines in light blue. Mounted at meeting room position [12,9].

## officeRenderer.js Updates

### FLOOR_COLORS additions
```javascript
10: '#141430',  // meeting room floor (deep indigo-purple)
11: '#e8e8f0',  // whiteboard face
```

### TILE_TO_FURNITURE additions
```javascript
10: 'meetingTable',
11: 'whiteboard',
```

### Zone Labels (in _drawBackground)
Add to existing labels:
```javascript
ctx.fillText('MEETING', 11 * TILE_PX, 10 * TILE_PX)
ctx.fillText('BREAK', 17 * TILE_PX, 11 * TILE_PX)
```

## getZoneTiles() Function

Zone boundaries as rectangle constants:

```javascript
const ZONE_BOUNDS = {
  openOffice:    { colMin: 0,  colMax: 19, rowMin: 0,  rowMax: 5  },
  meeting:       { colMin: 11, colMax: 15, rowMin: 8,  rowMax: 10 },
  breakArea:     { colMin: 17, colMax: 18, rowMin: 8,  rowMax: 12 },
  managerOffice: { colMin: 0,  colMax: 8,  rowMin: 8,  rowMax: 11 },
  reception:     { colMin: 8,  colMax: 19, rowMin: 11, rowMax: 13 },
}
```

Returns `[{col, row}, ...]` for all walkable (non-wall) tile positions within a zone.

## Files to Modify

1. `frontend/src/lib/office/layout.js` — FLOOR_MAP, DESKS, add getZoneTiles()
2. `frontend/src/lib/office/sprites.js` — add meetingTable + whiteboard to FURNITURE_SPRITES
3. `frontend/src/lib/office/officeRenderer.js` — FLOOR_COLORS, TILE_TO_FURNITURE, zone labels, prerender new sprites
