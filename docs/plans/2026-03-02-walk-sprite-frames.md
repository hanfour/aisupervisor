# Walking Sprite Frames (4 Directions) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `walkDown`, `walkUp`, `walkLeft`, `walkRight` animation states (3 frames each) to the Pixel Office character sprite system so characters can animate while moving.

**Architecture:** Two files are touched: `animation.js` gets 4 new `ANIM_CONFIG` entries; `sprites.js` gets 4 new states in `CHARACTER_FRAMES` and a `walkDown[0]` entry in each class's `CLASS_FRAME_OVERRIDES`. No other files need changing — `getFramesForClass()` already handles fallback via override merging, and `prerenderCharacter()` auto-caches all states.

**Tech Stack:** Plain JavaScript ES modules; 16×16 pixel-art strings; existing color-map system (`0`=transparent, `1-5`=hair/skin/shirt/pants/shoes, `6`=eye, `7`=outline, `A`=accent, uppercase=shade, lowercase=highlight)

---

## Colour Reference

| Char | Meaning         | Char | Meaning       |
|------|-----------------|------|---------------|
| `0`  | transparent     | `7`  | outline #111  |
| `1`  | hair            | `H`  | hairShade     |
| `h`  | hairHi          | `2`  | skin          |
| `S`  | skinShade       | `s`  | skinHi        |
| `3`  | shirt           | `T`  | shirtShade    |
| `t`  | shirtHi         | `4`  | pants         |
| `P`  | pantsShade      | `5`  | shoes         |
| `6`  | eye             | `e`  | eyeHi         |
| `A`  | accent          |      |               |

## Walk Cycle Logic

Each walk state has 3 frames:
- **Frame 0** — left leg forward (left shoe touches ground, right foot lifted)
- **Frame 1** — neutral stance (identical or near-identical to idle lower body)
- **Frame 2** — right leg forward (mirror of frame 0)

Lower-body changes per step frame (rows 13-15):
```
Frame 0 (left step):
  row 13: '0000074070000000'   right leg lifted (shorter)
  row 14: '0000075070000000'   left shoe, no right shoe
  row 15: '0000750000000000'   left shoe spread, right foot up

Frame 1 (neutral):
  row 13: '0000074470000000'   both legs
  row 14: '0000075570000000'   both shoes
  row 15: '0000750057000000'   feet spread

Frame 2 (right step):
  row 13: '0000070470000000'   left leg lifted
  row 14: '0000070570000000'   right shoe, no left shoe
  row 15: '0000000057000000'   right shoe spread, left foot up
```

---

## Task 1: Add ANIM_CONFIG entries — `animation.js`

**Files:**
- Modify: `frontend/src/lib/office/animation.js:3-9`

**Step 1: Open and read the file**

File: `frontend/src/lib/office/animation.js`

The `ANIM_CONFIG` object currently ends at line 9 with the `finished` entry.

**Step 2: Add the 4 walk entries**

Insert after the `finished` line inside `ANIM_CONFIG`:

```javascript
  walkDown:  { frameCount: 3, interval: 180, loop: true },
  walkUp:    { frameCount: 3, interval: 180, loop: true },
  walkLeft:  { frameCount: 3, interval: 180, loop: true },
  walkRight: { frameCount: 3, interval: 180, loop: true },
```

Final `ANIM_CONFIG` should look like:

```javascript
const ANIM_CONFIG = {
  idle:     { frameCount: 2, interval: 500, loop: true },
  working:  { frameCount: 3, interval: 250, loop: true },
  waiting:  { frameCount: 2, interval: 700, loop: true },
  error:    { frameCount: 1, interval: 400, loop: false, playCount: 2, fallback: 'idle' },
  finished: { frameCount: 3, interval: 300, loop: false, playCount: 2, fallback: 'idle' },
  walkDown:  { frameCount: 3, interval: 180, loop: true },
  walkUp:    { frameCount: 3, interval: 180, loop: true },
  walkLeft:  { frameCount: 3, interval: 180, loop: true },
  walkRight: { frameCount: 3, interval: 180, loop: true },
}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/office/animation.js
git commit -m "feat: add walkDown/Up/Left/Right entries to ANIM_CONFIG"
```

---

## Task 2: Add `walkDown` frames to `CHARACTER_FRAMES` — `sprites.js`

**Files:**
- Modify: `frontend/src/lib/office/sprites.js` — inside `CHARACTER_FRAMES`, after `finished`

**Step 1: Add walkDown**

Insert after the `finished` closing `],` inside `CHARACTER_FRAMES`:

```javascript
  walkDown: [
    // Frame 0 — left leg forward
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ],
    // Frame 1 — neutral
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    // Frame 2 — right leg forward
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000070470000000',
      '0000070570000000',
      '0000000057000000',
    ],
  ],
```

**Step 2: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat: add walkDown frames to CHARACTER_FRAMES"
```

---

## Task 3: Add `walkUp` frames to `CHARACTER_FRAMES`

**Design rationale:** Back-of-head view — no eyes (`6`), hair (`1`/`H`) fills the face rows, shirtShade (`T`) more prominent on torso back.

**Step 1: Add walkUp**

Insert after `walkDown` inside `CHARACTER_FRAMES`:

```javascript
  walkUp: [
    // Frame 0 — left leg forward (back view)
    [
      '0000077770000000',
      '00007H11H7000000',
      '0007H111H7000000',
      '0007H111H7000000',
      '00071S11S7000000',
      '0000711117000000',
      '0000073370000000',
      '000073T337000000',
      '00073T3TT3700000',
      '00073T3TT3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ],
    // Frame 1 — neutral (back view)
    [
      '0000077770000000',
      '00007H11H7000000',
      '0007H111H7000000',
      '0007H111H7000000',
      '00071S11S7000000',
      '0000711117000000',
      '0000073370000000',
      '000073T337000000',
      '00073T3TT3700000',
      '00073T3TT3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    // Frame 2 — right leg forward (back view)
    [
      '0000077770000000',
      '00007H11H7000000',
      '0007H111H7000000',
      '0007H111H7000000',
      '00071S11S7000000',
      '0000711117000000',
      '0000073370000000',
      '000073T337000000',
      '00073T3TT3700000',
      '00073T3TT3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000070470000000',
      '0000070570000000',
      '0000000057000000',
    ],
  ],
```

**Step 2: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat: add walkUp (back view) frames to CHARACTER_FRAMES"
```

---

## Task 4: Add `walkLeft` and `walkRight` frames to `CHARACTER_FRAMES`

**Design rationale:**
- Both views use a 3/4-perspective approach (not a hard profile)
- `walkLeft`: single eye on the left side of face row (`'0007762007700000'`), left arm swings forward in frame 0, right arm swings back in frame 2
- `walkRight`: single eye on the right side (`'0007700267700000'`), right arm swings in frame 0, left arm swings in frame 2
- Arm swing reuses the existing `working` arm patterns from `sprites.js`

**Step 1: Add walkLeft**

Insert after `walkUp` inside `CHARACTER_FRAMES`:

```javascript
  walkLeft: [
    // Frame 0 — left leg + left arm forward (character moves left)
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762007700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000273t337000000',
      '0027t33T37000000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ],
    // Frame 1 — neutral (facing left)
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762007700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    // Frame 2 — right leg + right arm back
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762007700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337200000',
      '000073t33T720000',
      '000073t33T270000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000070470000000',
      '0000070570000000',
      '0000000057000000',
    ],
  ],
  walkRight: [
    // Frame 0 — right leg + right arm forward (character moves right)
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007700267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337200000',
      '000073t33T720000',
      '000073t33T270000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000070470000000',
      '0000070570000000',
      '0000000057000000',
    ],
    // Frame 1 — neutral (facing right)
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007700267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    // Frame 2 — left leg + left arm back
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007700267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000273t337000000',
      '0027t33T37000000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ],
  ],
```

**Step 2: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat: add walkLeft and walkRight frames to CHARACTER_FRAMES"
```

---

## Task 5: Add `walkDown[0]` to `CLASS_FRAME_OVERRIDES`

**Context:** `getFramesForClass()` checks `overrides[state]` and splices the override frames at index 0. Adding `walkDown: [[...]]` is sufficient — frames 1 and 2 automatically fall back to `CHARACTER_FRAMES.walkDown`.

Each override below is the left-step frame (frame 0) with the class's distinctive accessories preserved, plus the left-step lower body (`rows 13-15` same as base walkDown frame 0).

**Step 1: Add to `coder`**

In `CLASS_FRAME_OVERRIDES.coder`, add after the `idle` property:

```javascript
    walkDown: [[
      '000A077770A00000',
      '000A7h11H7A00000',
      '0007h111HH700000',
      '0007762e67700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '0007t3tt3T700000',
      '0007t3tt3T700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 2: Add to `hacker`**

In `CLASS_FRAME_OVERRIDES.hacker`, add after the `idle` property:

```javascript
    walkDown: [[
      '0000777777000000',
      '0007H1111H700000',
      '0071H1111H170000',
      '0077762e67770000',
      '0007722227700000',
      '0000777777000000',
      '0000073370000000',
      '000073t337000000',
      '0007t3tt3T700000',
      '000A73337A000000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 3: Add to `designer`**

In `CLASS_FRAME_OVERRIDES.designer`, add after the `idle` property:

```javascript
    walkDown: [[
      '00000A7700000000',
      '0000A77770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762e67700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 4: Add to `analyst`**

In `CLASS_FRAME_OVERRIDES.analyst`, add after the `idle` property:

```javascript
    walkDown: [[
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '000A76e6e7A00000',
      '00072s22S2700000',
      '0000722227000000',
      '000007A370000000',
      '000073A337000000',
      '00073tA3T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 5: Add to `architect`**

In `CLASS_FRAME_OVERRIDES.architect`, add after the `idle` property:

```javascript
    walkDown: [[
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762e67700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '00A073t337A00000',
      '0A073t33T370A000',
      '0A073t33T370A000',
      '0A007333370A0000',
      '0A00074470A00000',
      '00A074P4470A0000',
      '000A074070A00000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 6: Add to `devops`**

In `CLASS_FRAME_OVERRIDES.devops`, add after the `idle` property:

```javascript
    walkDown: [[
      '000AAAAAAA000000',
      '000A7A7A7A000000',
      '00007h11H7000000',
      '000A76e6e7A00000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074070000000',
      '0000075070000000',
      '0000750000000000',
    ]],
```

**Step 7: Commit**

```bash
git add frontend/src/lib/office/sprites.js
git commit -m "feat: add walkDown class overrides for all 6 character types"
```

---

## Task 6: Visual verification

**Step 1: Start the dev server**

```bash
cd frontend && npm run dev
```

Expected: server starts at `http://localhost:5173` (or similar port shown in terminal)

**Step 2: Open Pixel Office page**

Navigate to the Pixel Office page in the app. You should see characters rendered on the canvas.

**Step 3: Trigger walk animation (if characters don't auto-walk)**

Open browser devtools console and run:
```javascript
// If there's an office store/API, set a worker's animation state
// Check for the AnimationState class usage in layout.js or the office canvas component
```

Or find where `AnimationState.setState()` is called in the codebase and add a temporary `setState('walkDown')` call for testing.

**Step 4: Verify each direction**

Check that:
- `walkDown` shows front-view character with alternating feet
- `walkUp` shows back-of-head with no eyes, alternating feet
- `walkLeft` shows single left eye, arm swing to left
- `walkRight` shows single right eye, arm swing to right
- Class variants (coder, hacker, designer, analyst, architect, devops) show their accessories in `walkDown`

**Step 5: Check for console errors**

Ensure no `undefined` frame errors or canvas rendering errors appear in the browser console.

**Step 6: Final commit (if any fixes were needed)**

```bash
git add frontend/src/lib/office/sprites.js frontend/src/lib/office/animation.js
git commit -m "fix: correct walk sprite frame issues found during visual review"
```

---

## Checklist

- [ ] Task 1: `ANIM_CONFIG` entries added — 4 walk states with `interval: 180, loop: true`
- [ ] Task 2: `walkDown` 3 frames in `CHARACTER_FRAMES`
- [ ] Task 3: `walkUp` 3 frames in `CHARACTER_FRAMES`
- [ ] Task 4: `walkLeft` + `walkRight` 3 frames each in `CHARACTER_FRAMES`
- [ ] Task 5: `walkDown[0]` added to all 6 classes in `CLASS_FRAME_OVERRIDES`
- [ ] Task 6: Visual verification complete, no console errors
