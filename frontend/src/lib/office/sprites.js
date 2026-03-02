// Pixel Office — MetroCity spritesheet characters + hacker-base furniture
// Characters use MetroCity Free Top Down Character Pack (CC0) by JIK-A-4
// Furniture/env sprites remain as pixel-string data.

import bodyUrl from './assets/body.png'
import outfit1Url from './assets/outfit1.png'
import outfit2Url from './assets/outfit2.png'
import outfit3Url from './assets/outfit3.png'
import outfit4Url from './assets/outfit4.png'
import outfit5Url from './assets/outfit5.png'
import outfit6Url from './assets/outfit6.png'
import hair1Url from './assets/hair1.png'
import hair2Url from './assets/hair2.png'
import hair3Url from './assets/hair3.png'
import hair4Url from './assets/hair4.png'
import hair5Url from './assets/hair5.png'
import hair6Url from './assets/hair6.png'
import hair7Url from './assets/hair7.png'
import shadowUrl from './assets/shadow.png'

// ── Spritesheet constants ────────────────────────────────────────────────────
const FRAME_SIZE = 32    // each frame is 32×32 px in the spritesheet
const SHEET_COLS = 24    // 24 frames per row
const FRAMES_PER_DIR = 6 // 6 walk frames per direction

// Direction offsets (column index = dir * FRAMES_PER_DIR)
const DIR = { down: 0, left: 1, right: 2, up: 3 }

// body.png has 6 rows = 6 body/skin types
const BODY_ROWS = 6

// ── Character type → spritesheet combination ─────────────────────────────────
// Maps each character type to a body row, outfit file, and hair file
const CHARACTER_CONFIGS = {
  coder:      { bodyRow: 0, outfit: 'outfit1', hair: 'hair1' },
  hacker:     { bodyRow: 1, outfit: 'outfit2', hair: 'hair2' },
  designer:   { bodyRow: 2, outfit: 'outfit3', hair: 'hair3' },
  analyst:    { bodyRow: 3, outfit: 'outfit4', hair: 'hair4' },
  architect:  { bodyRow: 4, outfit: 'outfit5', hair: 'hair5' },
  devops:     { bodyRow: 5, outfit: 'outfit6', hair: 'hair6' },
  researcher: { bodyRow: 0, outfit: 'outfit1', hair: 'hair7' },
  reviewer:   { bodyRow: 2, outfit: 'outfit3', hair: 'hair5' },
}

const CHARACTER_NAMES = Object.keys(CHARACTER_CONFIGS)

// ── Animation state → frame indices (from down-facing direction) ─────────────
// MetroCity walk cycle: 6 frames per direction, we pick subsets for each state
const ANIM_FRAME_MAP = {
  idle:     [0, 1],        // standing still, slight sway
  working:  [0, 2, 4],     // active typing/working
  waiting:  [0, 3],        // slow pace
  error:    [0],            // frozen
  finished: [0, 1, 2],     // celebratory
}

// ── Image loading ────────────────────────────────────────────────────────────
const imageCache = {}
let imagesLoaded = false
let loadPromise = null

const IMAGE_URLS = {
  body: bodyUrl,
  outfit1: outfit1Url, outfit2: outfit2Url, outfit3: outfit3Url,
  outfit4: outfit4Url, outfit5: outfit5Url, outfit6: outfit6Url,
  hair1: hair1Url, hair2: hair2Url, hair3: hair3Url, hair4: hair4Url,
  hair5: hair5Url, hair6: hair6Url, hair7: hair7Url,
  shadow: shadowUrl,
}

function loadImage(url) {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      // Use decode() to ensure image is fully decoded before resolving
      // This is critical for Wails WebView where onload fires before decode completes
      if (img.decode) {
        img.decode().then(() => resolve(img)).catch(() => resolve(img))
      } else {
        resolve(img)
      }
    }
    img.onerror = () => reject(new Error(`Failed to load: ${url}`))
    img.src = url
  })
}

export function loadAllSprites() {
  if (loadPromise) return loadPromise
  loadPromise = Promise.all(
    Object.entries(IMAGE_URLS).map(async ([name, url]) => {
      imageCache[name] = await loadImage(url)
    })
  ).then(() => { imagesLoaded = true })
  return loadPromise
}

export function spritesReady() { return imagesLoaded }

// ── Character prerendering (composites body + outfit + hair) ─────────────────

function extractFrame(img, col, row) {
  // Extract a single 32×32 frame from a spritesheet
  const c = document.createElement('canvas')
  c.width = FRAME_SIZE
  c.height = FRAME_SIZE
  const ctx = c.getContext('2d')
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(img,
    col * FRAME_SIZE, row * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE,
    0, 0, FRAME_SIZE, FRAME_SIZE
  )
  return c
}

function compositeFrame(bodyImg, bodyRow, outfitImg, hairImg, col) {
  const c = document.createElement('canvas')
  c.width = FRAME_SIZE
  c.height = FRAME_SIZE
  const ctx = c.getContext('2d')
  ctx.imageSmoothingEnabled = false

  // Layer 1: body
  ctx.drawImage(bodyImg,
    col * FRAME_SIZE, bodyRow * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE,
    0, 0, FRAME_SIZE, FRAME_SIZE
  )

  // Layer 2: outfit (single row, same column)
  if (outfitImg) {
    ctx.drawImage(outfitImg,
      col * FRAME_SIZE, 0, FRAME_SIZE, FRAME_SIZE,
      0, 0, FRAME_SIZE, FRAME_SIZE
    )
  }

  // Layer 3: hair (32×32 single image, always overlaid at same position)
  if (hairImg) {
    ctx.drawImage(hairImg, 0, 0, FRAME_SIZE, FRAME_SIZE)
  }

  return c
}

export function prerenderCharacter(charType) {
  if (!imagesLoaded) return null
  const config = CHARACTER_CONFIGS[charType]
  if (!config) return null

  const bodyImg = imageCache.body
  const outfitImg = imageCache[config.outfit]
  const hairImg = imageCache[config.hair]
  if (!bodyImg) return null

  const cache = {}

  // For each animation state, prerender the frames (all use down-facing direction)
  for (const [state, frameIndices] of Object.entries(ANIM_FRAME_MAP)) {
    cache[state] = frameIndices.map(fi => {
      const col = DIR.down * FRAMES_PER_DIR + fi
      return compositeFrame(bodyImg, config.bodyRow, outfitImg, hairImg, col)
    })
  }

  // Four-direction walk frames (3 frames per direction, matching animation.js walkDown/Up/Left/Right)
  const WALK_FRAMES = [0, 1, 2]
  for (const [dirName, dirIdx] of Object.entries(DIR)) {
    const animName = 'walk' + dirName[0].toUpperCase() + dirName.slice(1)
    cache[animName] = WALK_FRAMES.map(fi => {
      const col = dirIdx * FRAMES_PER_DIR + fi
      return compositeFrame(bodyImg, config.bodyRow, outfitImg, hairImg, col)
    })
  }

  return cache
}

// ── Character type resolution ────────────────────────────────────────────────
const AVATAR_TO_CHAR = {
  'coder': 'coder',
  'hacker': 'hacker',
  'designer': 'designer',
  'analyst': 'analyst',
  'architect': 'architect',
  'devops': 'devops',
  'researcher': 'researcher',
  'reviewer': 'reviewer',
}

export function getCharacterType(worker) {
  if (worker.avatar && AVATAR_TO_CHAR[worker.avatar]) return AVATAR_TO_CHAR[worker.avatar]
  if (worker.skillProfile && AVATAR_TO_CHAR[worker.skillProfile]) return AVATAR_TO_CHAR[worker.skillProfile]
  let hash = 0
  const id = worker.id || worker.name || ''
  for (let i = 0; i < id.length; i++) {
    hash = ((hash << 5) - hash) + id.charCodeAt(i)
    hash |= 0
  }
  return CHARACTER_NAMES[Math.abs(hash) % CHARACTER_NAMES.length]
}

// ══════════════════════════════════════════════════════════════════════════════
// FURNITURE & ENV SPRITES — Unchanged pixel-string rendering
// ══════════════════════════════════════════════════════════════════════════════

const FURNITURE_PALETTE = {
  '7': '#2a2a3a',  // metal frame
  '8': '#1a1a2e',  // dark surface
  '9': '#00ff41',  // LED green
  'b': '#00ddff',  // cyan glow
  '1': '#44eeff',  // screen
  '3': '#ff4444',  // LED red
  'c': '#00ff41',  // neon green
  'd': '#ff00ff',  // neon pink
  'e': '#e8e8f0',  // whiteboard face (off-white)
}

const FURNITURE_SPRITES = {
  desk: [
    '0000000000000000',
    '0000000000000000',
    '0771107711077110',
    '07b1107b1107b110',
    '07b1107b1107b110',
    '0771107711077110',
    '0007700770077000',
    '0777777777777700',
    '0788888888888700',
    '0777777777777700',
    '0007000000070000',
    '00070c00c0070000',
    '0007000000070000',
    '0007000000070000',
    '00070c00c0070000',
    '0000000000000000',
  ],
  computer: [
    '0000000000000000',
    '000077777700b000',
    '00007b11b700b000',
    '00007b1b1700b000',
    '00007b11b7000000',
    '000077777700b000',
    '000000bb00000000',
    '00000b77b0000000',
    '0000077770000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  plant: [
    '0777777777777700',
    '07888888888c8700',
    '0789999938888700',
    '07888888888c8700',
    '0777777777777700',
    '07888888888c8700',
    '0789999938888700',
    '07888888888c8700',
    '0777777777777700',
    '078888888883c700',
    '0789999938888700',
    '078888888883c700',
    '0777777777777700',
    '0007000000070000',
    '0007000000070000',
    '0077700000777000',
  ],
  watercooler: [
    '0077777777770000',
    '0078888888870000',
    '007c3c3c3c870000',
    '007c3c3c3c870000',
    '0078888888870000',
    '0077777777770000',
    '00780c000c870000',
    '0078888888870000',
    '0078888888870000',
    '007899c998870000',
    '0078888888870000',
    '0077777777770000',
    '0078888888870000',
    '0078888888870000',
    '0077777777770000',
    '0000000000000000',
  ],
  bookshelf: [
    '7777777777777777',
    '7b1b71b171b1b177',
    '71b171b1b1b17177',
    '7b1b71b171b1b177',
    '7777777777777777',
    '71b1b7b1b171b177',
    '7b1b171b17b1b177',
    '71b1b7b1b171b177',
    '7777777777777777',
    '7b17b1b171b1b177',
    '71b1b171b17b1b77',
    '7b17b1b171b1b177',
    '7777777777777777',
    '70c000c000c00077',
    '7000000000000077',
    '7777777777777777',
  ],
  meetingTable: [
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
  whiteboard: [
    '7777777777777777',
    '7eeeeeeeeeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbbbbe7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbbbee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbbbeeee7',
    '7eeeeeeeeeeeeee7',
    '7ebbbbbbbeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7eeeeeeeeeeeeee7',
    '7777777777777777',
    '7000000000000007',
    '7700000000000077',
    '7777777777777777',
  ],
}

const ENV_SPRITES = {
  glowStrip: [
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    'cccccccccccccccc',
    'bbbbbbbbbbbbbbbb',
    'cccccccccccccccc',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  cableFloor: [
    '0000000000000000',
    '0000000000000000',
    '0000000070000000',
    '0000000770000000',
    '0000007700000000',
    '0000077000000000',
    '0000770000000000',
    '0000700000000000',
    '0000700000000000',
    '0000770000000000',
    '0000077000000000',
    '0000007700000000',
    '0000000770000000',
    '0000000070000000',
    '0000000000000000',
    '0000000000000000',
  ],
}

function renderSpriteToCanvas(ctx, rows, palette, x, y, scale) {
  for (let row = 0; row < rows.length; row++) {
    const line = rows[row]
    for (let col = 0; col < line.length; col++) {
      const ch = line[col]
      if (ch === '0') continue
      let color
      if (palette && ch === '7') {
        color = '#3a3a4e'
      } else if (FURNITURE_PALETTE[ch]) {
        color = FURNITURE_PALETTE[ch]
      } else {
        continue
      }
      ctx.fillStyle = color
      ctx.fillRect(x + col * scale, y + row * scale, scale, scale)
    }
  }
}

export function prerenderFurniture(name) {
  const rows = FURNITURE_SPRITES[name]
  if (!rows) return null
  const scale = 3
  const size = 16 * scale
  const offscreen = document.createElement('canvas')
  offscreen.width = size
  offscreen.height = size
  const ctx = offscreen.getContext('2d')
  renderSpriteToCanvas(ctx, rows, null, 0, 0, scale)
  return offscreen
}

export function prerenderEnvSprite(name) {
  const rows = ENV_SPRITES[name]
  if (!rows) return null
  const scale = 3
  const offscreen = document.createElement('canvas')
  offscreen.width = 16 * scale
  offscreen.height = 16 * scale
  const ctx = offscreen.getContext('2d')
  renderSpriteToCanvas(ctx, rows, null, 0, 0, scale)
  return offscreen
}

// ── Skill profile rendering data ─────────────────────────────────────────────
export const SKILL_PROFILE_COLORS = {
  coder:      '#ff4444',
  hacker:     '#00ff41',
  designer:   '#ff44ff',
  analyst:    '#88ccff',
  architect:  '#cc88ff',
  devops:     '#ffaa00',
  researcher: '#44ddff',
  reviewer:   '#44ff88',
}

export const SKILL_PROFILE_ICONS = {
  coder:      '\u{1F4BB}',
  hacker:     '\u{1F575}',
  designer:   '\u{1F3A8}',
  analyst:    '\u{1F50D}',
  architect:  '\u{1F3DB}',
  devops:     '\u{1F680}',
  researcher: '\u{1F4DA}',
  reviewer:   '\u2705',
}

export { FURNITURE_SPRITES, ENV_SPRITES, CHARACTER_NAMES, CHARACTER_CONFIGS }
