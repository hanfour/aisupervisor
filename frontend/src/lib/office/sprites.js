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
// outfit1=casual, outfit2=hoodie/hacker, outfit3=creative/fashionable,
// outfit4=smart-casual/analytical, outfit5=executive/architect, outfit6=technical/devops
const CHARACTER_CONFIGS = {
  coder:      { bodyRow: 0, outfit: 'outfit1', hair: 'hair1' },  // casual tee
  hacker:     { bodyRow: 1, outfit: 'outfit2', hair: 'hair2' },  // hoodie
  designer:   { bodyRow: 2, outfit: 'outfit3', hair: 'hair3' },  // fashionable
  analyst:    { bodyRow: 3, outfit: 'outfit4', hair: 'hair4' },  // smart-casual
  architect:  { bodyRow: 4, outfit: 'outfit5', hair: 'hair5' },  // executive
  devops:     { bodyRow: 5, outfit: 'outfit6', hair: 'hair6' },  // technical
  researcher: { bodyRow: 0, outfit: 'outfit1', hair: 'hair7' },  // academic casual
  reviewer:   { bodyRow: 2, outfit: 'outfit3', hair: 'hair5' },  // professional
}

// ── Per-worker unique appearance ─────────────────────────────────────────────
// Each worker gets a unique body/outfit/hair combination reflecting their role and personality.
// bodyRow = skin tone (0=lightest … 5=darkest), outfit = style, hair = hairstyle
const WORKER_CONFIGS = {
  // ── Top executive (architect/consultant) ──────────────────────────────────
  'Hanfour':           { bodyRow: 4, outfit: 'outfit5', hair: 'hair5' }, // distinguished exec, dark skin

  // ── Managers (business casual to formal) ─────────────────────────────────
  'Sundar Pichai':     { bodyRow: 3, outfit: 'outfit4', hair: 'hair2' }, // South Asian, polished
  'Joe':               { bodyRow: 1, outfit: 'outfit4', hair: 'hair3' }, // business casual, lighter
  'Steve':             { bodyRow: 0, outfit: 'outfit3', hair: 'hair1' }, // creative-manager, fair
  'Ken Norton':        { bodyRow: 2, outfit: 'outfit5', hair: 'hair4' }, // formal manager, medium

  // ── Coders (casual tech wear) ─────────────────────────────────────────────
  'Niko':              { bodyRow: 5, outfit: 'outfit1', hair: 'hair6' }, // casual coder, dark
  'Ryan':              { bodyRow: 1, outfit: 'outfit2', hair: 'hair3' }, // hoodie coder
  'Edwina':            { bodyRow: 0, outfit: 'outfit1', hair: 'hair7' }, // female coder, fair, long hair
  'Shan':              { bodyRow: 3, outfit: 'outfit1', hair: 'hair4' }, // casual, medium-dark
  'Rocco':             { bodyRow: 2, outfit: 'outfit2', hair: 'hair6' }, // hoodie, medium
  'Jamie':             { bodyRow: 4, outfit: 'outfit2', hair: 'hair2' }, // gender-neutral, hoodie

  // ── DevOps (technical, rugged) ────────────────────────────────────────────
  'Kirt':              { bodyRow: 0, outfit: 'outfit6', hair: 'hair2' }, // technical, fair
  'Mario':             { bodyRow: 5, outfit: 'outfit6', hair: 'hair3' }, // technical, dark skin

  // ── Designers (creative, fashionable) ────────────────────────────────────
  'Bruce Tognazinni':  { bodyRow: 2, outfit: 'outfit3', hair: 'hair5' }, // seasoned designer, medium
  'Elain':             { bodyRow: 1, outfit: 'outfit3', hair: 'hair7' }, // female designer, long hair
  'Luke Wroblewski':   { bodyRow: 3, outfit: 'outfit3', hair: 'hair1' }, // UX designer, medium-dark

  // ── Analyst ───────────────────────────────────────────────────────────────
  'Jakob Nielsen':     { bodyRow: 4, outfit: 'outfit4', hair: 'hair4' }, // analytical, smart-casual

  // ── Hacker (hoodie, distinctive) ─────────────────────────────────────────
  'Lastor':            { bodyRow: 5, outfit: 'outfit2', hair: 'hair1' }, // hacker, dark, cropped

  // ── Researcher (academic, thoughtful) ────────────────────────────────────
  'Alice':             { bodyRow: 0, outfit: 'outfit1', hair: 'hair5' }, // researcher, fair, medium hair
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

// Render character frames from an appearance config {bodyRow, outfit, hair}
function _renderFromAppearance(appearance) {
  const bodyImg = imageCache.body
  const outfitImg = imageCache[appearance.outfit]
  const hairImg = imageCache[appearance.hair]
  if (!bodyImg) return null

  const cache = {}

  for (const [state, frameIndices] of Object.entries(ANIM_FRAME_MAP)) {
    cache[state] = frameIndices.map(fi => {
      const col = DIR.down * FRAMES_PER_DIR + fi
      return compositeFrame(bodyImg, appearance.bodyRow, outfitImg, hairImg, col)
    })
  }

  const WALK_FRAMES = [0, 1, 2]
  for (const [dirName, dirIdx] of Object.entries(DIR)) {
    const animName = 'walk' + dirName[0].toUpperCase() + dirName.slice(1)
    cache[animName] = WALK_FRAMES.map(fi => {
      const col = dirIdx * FRAMES_PER_DIR + fi
      return compositeFrame(bodyImg, appearance.bodyRow, outfitImg, hairImg, col)
    })
  }

  return cache
}

export function prerenderCharacter(charType) {
  if (!imagesLoaded) return null
  const config = WORKER_CONFIGS[charType] || CHARACTER_CONFIGS[charType]
  if (!config) return null
  return _renderFromAppearance(config)
}

// Prerender from a worker's custom appearance (backend WorkerAppearance)
export function prerenderCharacterFromAppearance(appearance) {
  if (!imagesLoaded || !appearance) return null
  return _renderFromAppearance(appearance)
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
  // Custom appearance from backend takes highest priority
  if (worker.appearance) return `custom_${worker.id}`

  // Per-worker unique appearance takes priority
  if (worker.name && WORKER_CONFIGS[worker.name]) return worker.name

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
  '7': '#c8a878',  // light wood frame
  '8': '#f0e6d3',  // light panel surface
  '9': '#4a9e5c',  // plant green
  'b': '#5bbad5',  // soft blue
  '1': '#87ceeb',  // sky blue screen
  '3': '#e8a855',  // warm orange
  'c': '#6bb87b',  // warm green
  'd': '#c97b5e',  // terracotta
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
  coffeeBar: [
    '7777777777777777',
    '7888888888888877',
    '7833333333333877',
    '7888888888888877',
    '7777777777777777',
    '7888dd8dd8888877',
    '78883388338dd877',
    '78888888888dd877',
    '7888888888888877',
    '7777777777777777',
    '7800000000008077',
    '7800000000008077',
    '7800000000008077',
    '7800000000008077',
    '7777777777777777',
    '0000000000000000',
  ],
  sofa: [
    '0000000000000000',
    '0077777777777700',
    '0078888888887700',
    '007ddddddddd7700',
    '007ddddddddd7700',
    '007ddddddddd7700',
    '0078888888887700',
    '0077777777777700',
    '0077000000007700',
    '0077000000007700',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  largePlant: [
    '0000009990000000',
    '0000999999000000',
    '0009999999900000',
    '0099999999990000',
    '0099999999990000',
    '0009999999900000',
    '0000999999000000',
    '0000099990000000',
    '0000007700000000',
    '0000007700000000',
    '0000077770000000',
    '0007777777700000',
    '0078888888870000',
    '0078888888870000',
    '0077777777770000',
    '0000000000000000',
  ],
}

const ENV_SPRITES = {
  baseboard: [
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '7777777777777777',
    '3333333333333333',
    '7777777777777777',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  rugPattern: [
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
        color = '#b89860'
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
  coder:      '#e07050',
  hacker:     '#6bb87b',
  designer:   '#d4a0d4',
  analyst:    '#5bbad5',
  architect:  '#b088d0',
  devops:     '#e8a855',
  researcher: '#60c0d0',
  reviewer:   '#80c888',
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

export { FURNITURE_SPRITES, ENV_SPRITES, CHARACTER_NAMES, CHARACTER_CONFIGS, WORKER_CONFIGS }
