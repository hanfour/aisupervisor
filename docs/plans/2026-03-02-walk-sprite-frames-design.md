# Walking Sprite Frames (4 Directions) — Design

**Date:** 2026-03-02
**Branch:** `ai/p1772437859967-1/t1772437912347-1-walking-sprite-frames-4-direct`

## Overview

Add 4-direction walk animations to the Pixel Office character sprite system. Characters need `walkDown`, `walkUp`, `walkLeft`, `walkRight` states so they can animate while moving around the office canvas.

## Scope

- **`sprites.js`**: Add `CHARACTER_FRAMES` walk states + `CLASS_FRAME_OVERRIDES` for walk
- **`animation.js`**: Add `ANIM_CONFIG` entries for the 4 walk states

## Pixel Art Design

### Coordinate System

Each frame is a 16×16 grid of 16-char strings (row-major, col 0 = left). Characters are centered around cols 3–12, occupying roughly 8px wide.

**Color reference:**
| Char | Meaning | Char | Meaning |
|------|---------|------|---------|
| `0` | transparent | `7` | outline (#111) |
| `1` | hair | `H` | hairShade |
| `h` | hairHi | `2` | skin |
| `S` | skinShade | `s` | skinHi |
| `3` | shirt | `T` | shirtShade |
| `t` | shirtHi | `4` | pants |
| `P` | pantsShade | `5` | shoes |
| `6` | eye | `e` | eyeHi |
| `A` | accent | | |

### walkDown (front view, 3 frames)

Upper body (rows 0–10) identical to `idle[0]`. Lower body alternates:

- **Frame 0**: left foot forward — left shoe column shifts left, right shoe lifted
- **Frame 1**: neutral standing — identical to idle lower body
- **Frame 2**: right foot forward — right shoe column shifts right, left shoe lifted

### walkUp (back view, 3 frames)

- Head area: hair only, no face details (no `6` eye, no `s` skinHi, no `2` in face rows)
- Torso: shirtShade (`T`) more prominent (back of shirt)
- Legs: same 3-frame alternation pattern as walkDown

### walkLeft (side profile facing left, 3 frames)

- Narrower head silhouette (side profile)
- Single eye visible on right side of face sprite (character faces left)
- Body: one arm visible, slight arm swing between frames
- Legs: side-view alternation (one leg forward, one back in each step frame)

### walkRight (side profile facing right, 3 frames)

- Independent pixel data (cannot programmatically flip string rows)
- Mirror design of walkLeft — eye on left side of face sprite
- Same arm/leg rhythm

## Animation Config

```javascript
walkDown:  { frameCount: 3, interval: 180, loop: true },
walkUp:    { frameCount: 3, interval: 180, loop: true },
walkLeft:  { frameCount: 3, interval: 180, loop: true },
walkRight: { frameCount: 3, interval: 180, loop: true },
```

Interval 180ms gives a fluid 5.5-step-per-second rhythm matching typical JRPG walk speed.

## Class Frame Overrides

Only `walkDown[0]` is overridden per class. Other walk directions and frames inherit from base `CHARACTER_FRAMES` via the existing `getFramesForClass()` fallback mechanism (no code changes required).

| Class | walkDown[0] feature preserved |
|-------|-------------------------------|
| coder | `A` accent headphones on both sides of head |
| hacker | Full hood outline (`1`/`H`) over enlarged head |
| designer | `A` accent beret atop head |
| analyst | `A` accent glasses + tie down torso centre |
| architect | `A` accent long coat/cape extending past torso sides |
| devops | `A` accent hard hat rows above normal head |

## Technical Notes

- `getFramesForClass(charType)` already handles: override replaces base frames from index 0, remainder falls back to base. No changes needed.
- `prerenderCharacter()` iterates all states/frames automatically — new walk states are cached the same way.
- `walkRight` is NOT a programmatic mirror of `walkLeft` — separate pixel rows required.
- Frame 1 of all walk states can reuse the neutral idle pose (no duplication issue since each state has its own array).

## Files to Modify

1. `frontend/src/lib/office/sprites.js`
   - Add `walkDown`, `walkUp`, `walkLeft`, `walkRight` to `CHARACTER_FRAMES`
   - Add `walkDown` entry to each class in `CLASS_FRAME_OVERRIDES`

2. `frontend/src/lib/office/animation.js`
   - Add 4 entries to `ANIM_CONFIG`
