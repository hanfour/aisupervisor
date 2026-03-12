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
  'f': '#a08050',  // dark wood shadow
  'g': '#dcc8a0',  // light wood highlight
  'h': '#3d8b4e',  // dark green (plant shadow)
  'i': '#ffe8a0',  // warm lamp glow
  'j': '#4488bb',  // dark blue (screen shadow)
  'k': '#333333',  // near black (screen frame)
  'l': '#f5f0e0',  // cream highlight
  'm': '#bb6644',  // dark terracotta
  'n': '#66cc88',  // bright green
  'o': '#8866aa',  // purple (book)
  'p': '#cc4444',  // red (book / accent)
  'q': '#e0d0b0',  // light surface shadow
  'r': '#ffcc66',  // gold accent
}

const FURNITURE_SPRITES = {
  desk: [
    '0000000000000000',
    '0000000000000000',
    '0fgggggggggggf00',
    '0f78l8l8l8l87f00',
    '0f78l8l8l8l87f00',
    '0f7888888888qf00',
    '0fgggggggggggf00',
    '0f77777777777f00',
    '0f8q8q8q8q8q7f00',
    '0f77777777777f00',
    '000f0000000f0000',
    '000f00000c0f0000',
    '000f0000000f0000',
    '000f00000c0f0000',
    '000ff000000ff000',
    '0000000000000000',
  ],
  computer: [
    '0000000000000000',
    '000kkkkkkk000000',
    '000kj11bjk000000',
    '000k1b1j1k0r0000',
    '000kj11b1k000000',
    '000kkkkkkk000000',
    '00000kfk00000000',
    '0000kf77fk000000',
    '000kf7777fk00000',
    '000kkkkkkk000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  plant: [
    '00000nh000000000',
    '0000n9hn9h000000',
    '000n999h999n0000',
    '00n9h999h9h9n000',
    '0n999999999h9n00',
    '0nh9h9999h999n00',
    '00n999h99999n000',
    '000nh99999hn0000',
    '0000n9h99n000000',
    '00000nhhn0000000',
    '000000770f000000',
    '00000f770f000000',
    '0000f77777f00000',
    '0000fd8d88f00000',
    '00000fffff000000',
    '0000000000000000',
  ],
  watercooler: [
    '00077777777f0000',
    '000781b1b187f000',
    '000781b1b187f000',
    '00077777777f0000',
    '0007l8888l7f0000',
    '0007l8888l7f0000',
    '00077777777f0000',
    '000788888f7f0000',
    '00078c99c87f0000',
    '000788888f7f0000',
    '00077777777f0000',
    '000f78888f7f0000',
    '000f78888f7f0000',
    '000ff7777fff0000',
    '0000fffffff00000',
    '0000000000000000',
  ],
  bookshelf: [
    'fggggggggggggggg',
    '7pb7obp7bop7ob77',
    '7bp7pob7pbo7po77',
    '7ob7bpo7obp7bp77',
    'fggggggggggggggg',
    '7opb7bp7obp7bo77',
    '7bpo7ob7pob7op77',
    '7pob7po7bop7bp77',
    'fggggggggggggggg',
    '7bop7ob7pbo7op77',
    '7pbo7bp7obp7bo77',
    '7obp7po7bop7pb77',
    'fggggggggggggggg',
    '70c000c000c00077',
    '7f00f00f00f00f77',
    'ffffffffffffffff',
  ],
  meetingTable: [
    '0fgggggggggggf00',
    '0f888l888l888f00',
    '0fb8888888888bf0',
    '0f888l888l888f00',
    '0fgggggggggggf00',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0fgggggggggggf00',
    '0f888l888l888f00',
    '0fb8888888888bf0',
    '0f888l888l888f00',
    '0fgggggggggggf00',
  ],
  whiteboard: [
    'ffffffffffffffff',
    'feeeeeeeeeeeeeef',
    'fe00eee00eeee3ef',
    'febb1bbbbbbbbbef',
    'feeeeeeeeeeeeef',
    'febbbbbbbbbbbef',
    'feeeeeeeeeeeeef',
    'febbbbb3bbeeeef',
    'feeeeeeeeeeeeef',
    'febb3bbbeeeeef',
    'feeeeeeeeeeeef',
    'fe00p00b00ee3ef',
    'ffffffffffffffff',
    'f0000000000000f',
    'ff000000000000ff',
    'ffffffffffffffff',
  ],
  coffeeBar: [
    'fggggggggggggggg',
    'f8l888l888l888gf',
    'f833r33r33r338gf',
    'f8l888l888l888gf',
    'fggggggggggggggg',
    'f888dd8dd8888fgf',
    'f8883r83r8dd8fgf',
    'f88888888ddd8fgf',
    'f888l888l888lfgf',
    'fggggggggggggggg',
    'f80000000000f0gf',
    'f80000000000f0gf',
    'f80000000000f0gf',
    'f80000000000f0gf',
    'ffffffffffffffff',
    '0000000000000000',
  ],
  sofa: [
    '0000000000000000',
    '00fgggggggggf000',
    '00f788888887gf00',
    '00f7dddddddm7f0',
    '00f7dmdddmdd7f0',
    '00f7dddddddm7f0',
    '00f788888887gf00',
    '00fgggggggggf000',
    '00ff0000000ff000',
    '00ff0000000ff000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  largePlant: [
    '00000nhn00000000',
    '000nn9h99nn00000',
    '00n9h999h99n0000',
    '0n999h99999hn000',
    '0n9h99999h999n00',
    '00n99h999999n000',
    '000n9999h9nn0000',
    '0000nn99nn000000',
    '0000007700000000',
    '000000770f000000',
    '00000f7777f00000',
    '0000f777777f0000',
    '000fd88d888df000',
    '000fd88d888df000',
    '0000ffffffff0000',
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
    'ffffffffffffffff',
    'fggggggggggggggg',
    '3f3f3f3f3f3f3f3f',
    'fggggggggggggggg',
    'ffffffffffffffff',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
    '0000000000000000',
  ],
  rugPattern: [
    '0000000000000000',
    '00000000f0000000',
    '0000000f7f000000',
    '000000f777f00000',
    '00000f77f77f0000',
    '0000f77f0f77f000',
    '000f77f000f77f00',
    '0000f70000077f00',
    '0000f70000077f00',
    '000f77f000f77f00',
    '0000f77f0f77f000',
    '00000f77f77f0000',
    '000000f777f00000',
    '0000000f7f000000',
    '00000000f0000000',
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

// ══════════════════════════════════════════════════════════════════════════════
// PROGRAMMATIC FURNITURE DRAWING (48×48 canvas 2D)
// Rich detailed furniture drawn with gradients, shadows, and textures
// ══════════════════════════════════════════════════════════════════════════════

const DRAW_SIZE = 48  // native tile resolution

// Helper: rounded rectangle
function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.lineTo(x + w - r, y)
  ctx.quadraticCurveTo(x + w, y, x + w, y + r)
  ctx.lineTo(x + w, y + h - r)
  ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h)
  ctx.lineTo(x + r, y + h)
  ctx.quadraticCurveTo(x, y + h, x, y + h - r)
  ctx.lineTo(x, y + r)
  ctx.quadraticCurveTo(x, y, x + r, y)
  ctx.closePath()
}

// Helper: wood grain lines
function drawWoodGrain(ctx, x, y, w, h, color, count) {
  ctx.strokeStyle = color
  ctx.lineWidth = 0.5
  for (let i = 0; i < count; i++) {
    const yy = y + (h / (count + 1)) * (i + 1)
    ctx.beginPath()
    ctx.moveTo(x + 2, yy)
    // Slight wave for natural wood grain
    const mid = x + w / 2
    ctx.quadraticCurveTo(mid, yy + (i % 2 ? 1 : -1), x + w - 2, yy)
    ctx.stroke()
  }
}

const FURNITURE_DRAW = {
  desk(ctx, S) {
    // Shadow
    ctx.fillStyle = 'rgba(0,0,0,0.08)'
    roundRect(ctx, 5, 8, 38, 22, 2)
    ctx.fill()

    // Desktop surface — warm wood with gradient
    const topGrad = ctx.createLinearGradient(4, 5, 4, 20)
    topGrad.addColorStop(0, '#dcc8a0')
    topGrad.addColorStop(0.5, '#c8a878')
    topGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = topGrad
    roundRect(ctx, 4, 5, 38, 18, 2)
    ctx.fill()

    // Desktop edge highlight
    ctx.strokeStyle = '#e8dcc0'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(6, 6)
    ctx.lineTo(40, 6)
    ctx.stroke()

    // Wood grain on surface
    drawWoodGrain(ctx, 6, 7, 34, 14, 'rgba(139,110,70,0.3)', 4)

    // Front face (drawers area)
    const faceGrad = ctx.createLinearGradient(4, 23, 4, 40)
    faceGrad.addColorStop(0, '#c8a878')
    faceGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = faceGrad
    ctx.fillRect(4, 23, 38, 17)

    // Drawer dividers
    ctx.strokeStyle = '#8a6d40'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(6, 30); ctx.lineTo(40, 30)
    ctx.moveTo(6, 37); ctx.lineTo(40, 37)
    ctx.stroke()

    // Drawer handles (small gold ovals)
    ctx.fillStyle = '#d4a850'
    for (const yy of [26, 33]) {
      roundRect(ctx, 20, yy, 8, 2, 1)
      ctx.fill()
    }

    // Legs
    ctx.fillStyle = '#8a6d40'
    ctx.fillRect(6, 40, 3, 6)
    ctx.fillRect(37, 40, 3, 6)

    // Subtle border
    ctx.strokeStyle = '#8a6d40'
    ctx.lineWidth = 0.5
    roundRect(ctx, 4, 5, 38, 35, 2)
    ctx.stroke()
  },

  computer(ctx, S) {
    // This tile sits next to a desk tile — draw matching desk surface first
    // so the computer looks like it's ON the desk, not on the floor

    // ── Desk surface (same style as desk sprite) ──
    const topGrad = ctx.createLinearGradient(4, 5, 4, 23)
    topGrad.addColorStop(0, '#d4b880')
    topGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = topGrad
    roundRect(ctx, 4, 5, 38, 18, 2)
    ctx.fill()

    ctx.strokeStyle = '#e8dcc0'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(6, 6)
    ctx.lineTo(40, 6)
    ctx.stroke()

    drawWoodGrain(ctx, 6, 7, 34, 14, 'rgba(139,110,70,0.3)', 4)

    // Front face
    const faceGrad = ctx.createLinearGradient(4, 23, 4, 40)
    faceGrad.addColorStop(0, '#c8a878')
    faceGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = faceGrad
    ctx.fillRect(4, 23, 38, 17)

    // Legs
    ctx.fillStyle = '#8a6d40'
    ctx.fillRect(6, 40, 3, 6)
    ctx.fillRect(37, 40, 3, 6)

    ctx.strokeStyle = '#8a6d40'
    ctx.lineWidth = 0.5
    roundRect(ctx, 4, 5, 38, 35, 2)
    ctx.stroke()

    // ── Computer equipment ON the desk ──

    // Monitor stand base
    ctx.fillStyle = '#555'
    roundRect(ctx, 16, 16, 14, 2.5, 1)
    ctx.fill()

    // Monitor stand neck
    ctx.fillStyle = '#666'
    ctx.fillRect(21, 12, 4, 5)

    // Monitor body
    const monGrad = ctx.createLinearGradient(10, 1, 10, 13)
    monGrad.addColorStop(0, '#2a2a2a')
    monGrad.addColorStop(1, '#1a1a1a')
    ctx.fillStyle = monGrad
    roundRect(ctx, 10, 1, 26, 12, 2)
    ctx.fill()

    // Screen
    const scrGrad = ctx.createLinearGradient(12, 2, 12, 11)
    scrGrad.addColorStop(0, '#7ab8d4')
    scrGrad.addColorStop(0.3, '#5baad5')
    scrGrad.addColorStop(1, '#4488bb')
    ctx.fillStyle = scrGrad
    roundRect(ctx, 12, 2, 22, 9, 1)
    ctx.fill()

    // Screen content — code lines
    ctx.fillStyle = 'rgba(255,255,255,0.6)'
    for (let i = 0; i < 3; i++) {
      const w = 6 + (i * 7 % 11)
      ctx.fillRect(14, 4 + i * 2.5, w, 1.5)
    }

    // Screen reflection
    ctx.fillStyle = 'rgba(255,255,255,0.08)'
    ctx.fillRect(12, 2, 22, 4)

    // Power LED
    ctx.fillStyle = '#00ff41'
    ctx.beginPath()
    ctx.arc(23, 12.5, 1, 0, Math.PI * 2)
    ctx.fill()

    // Keyboard (on desk surface)
    ctx.fillStyle = '#444'
    roundRect(ctx, 14, 19, 18, 3, 1.5)
    ctx.fill()

    // Keyboard keys
    ctx.fillStyle = '#666'
    for (let r = 0; r < 2; r++) {
      for (let c = 0; c < 6; c++) {
        ctx.fillRect(15 + c * 2.8, 19.5 + r * 1.5, 2, 1)
      }
    }

    // Mouse
    ctx.fillStyle = '#555'
    roundRect(ctx, 34, 18, 4, 4, 2)
    ctx.fill()

    // Screen glow
    ctx.fillStyle = 'rgba(91,186,213,0.06)'
    ctx.beginPath()
    ctx.arc(23, 8, 16, 0, Math.PI * 2)
    ctx.fill()
  },

  plant(ctx, S) {
    // Pot
    const potGrad = ctx.createLinearGradient(14, 30, 32, 30)
    potGrad.addColorStop(0, '#c97b5e')
    potGrad.addColorStop(0.5, '#d4926e')
    potGrad.addColorStop(1, '#a06040')
    ctx.fillStyle = potGrad
    ctx.beginPath()
    ctx.moveTo(14, 30)
    ctx.lineTo(16, 44)
    ctx.lineTo(30, 44)
    ctx.lineTo(32, 30)
    ctx.closePath()
    ctx.fill()

    // Pot rim
    ctx.fillStyle = '#b06848'
    roundRect(ctx, 12, 28, 22, 4, 1)
    ctx.fill()

    // Pot highlight
    ctx.strokeStyle = 'rgba(255,255,255,0.15)'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(16, 32); ctx.lineTo(17, 42)
    ctx.stroke()

    // Dirt
    ctx.fillStyle = '#6b4a30'
    ctx.beginPath()
    ctx.ellipse(23, 29, 9, 2, 0, 0, Math.PI * 2)
    ctx.fill()

    // Leaves — multiple layers for depth
    const leaves = [
      { cx: 23, cy: 16, rx: 10, ry: 8, color: '#4a9e5c' },   // back layer
      { cx: 20, cy: 14, rx: 7, ry: 6, color: '#5cb86e' },     // mid-left
      { cx: 27, cy: 14, rx: 7, ry: 6, color: '#5cb86e' },     // mid-right
      { cx: 23, cy: 12, rx: 8, ry: 6, color: '#66cc88' },     // front layer
      { cx: 19, cy: 10, rx: 5, ry: 4, color: '#78dda0' },     // top-left highlight
      { cx: 28, cy: 11, rx: 5, ry: 4, color: '#78dda0' },     // top-right highlight
    ]
    for (const lf of leaves) {
      ctx.fillStyle = lf.color
      ctx.beginPath()
      ctx.ellipse(lf.cx, lf.cy, lf.rx, lf.ry, 0, 0, Math.PI * 2)
      ctx.fill()
    }

    // Leaf veins
    ctx.strokeStyle = 'rgba(0,80,20,0.2)'
    ctx.lineWidth = 0.5
    ctx.beginPath()
    ctx.moveTo(23, 20); ctx.lineTo(23, 8)
    ctx.moveTo(18, 16); ctx.lineTo(14, 10)
    ctx.moveTo(28, 16); ctx.lineTo(32, 10)
    ctx.stroke()

    // Stem
    ctx.strokeStyle = '#3d7040'
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(23, 28); ctx.lineTo(23, 18)
    ctx.stroke()
  },

  watercooler(ctx, S) {
    // Shadow base
    ctx.fillStyle = 'rgba(0,0,0,0.06)'
    ctx.beginPath()
    ctx.ellipse(23, 45, 10, 3, 0, 0, Math.PI * 2)
    ctx.fill()

    // Main body
    const bodyGrad = ctx.createLinearGradient(12, 12, 34, 12)
    bodyGrad.addColorStop(0, '#d8d8e0')
    bodyGrad.addColorStop(0.3, '#f0f0f8')
    bodyGrad.addColorStop(0.7, '#f0f0f8')
    bodyGrad.addColorStop(1, '#b8b8c0')
    ctx.fillStyle = bodyGrad
    roundRect(ctx, 12, 12, 22, 32, 3)
    ctx.fill()

    // Water bottle on top
    const bottleGrad = ctx.createLinearGradient(15, 0, 31, 0)
    bottleGrad.addColorStop(0, 'rgba(135,206,235,0.6)')
    bottleGrad.addColorStop(0.4, 'rgba(180,230,255,0.7)')
    bottleGrad.addColorStop(1, 'rgba(100,180,220,0.5)')
    ctx.fillStyle = bottleGrad
    roundRect(ctx, 15, 0, 16, 14, 3)
    ctx.fill()

    // Water level line
    ctx.strokeStyle = 'rgba(80,160,200,0.4)'
    ctx.lineWidth = 0.5
    ctx.beginPath()
    ctx.moveTo(16, 6); ctx.lineTo(30, 6)
    ctx.stroke()

    // Bottle cap
    ctx.fillStyle = '#aaa'
    roundRect(ctx, 18, 12, 10, 2, 1)
    ctx.fill()

    // Dispenser buttons
    ctx.fillStyle = '#cc4444' // hot
    roundRect(ctx, 14, 22, 6, 4, 1)
    ctx.fill()
    ctx.fillStyle = '#4488cc' // cold
    roundRect(ctx, 26, 22, 6, 4, 1)
    ctx.fill()

    // Drip tray
    ctx.fillStyle = '#999'
    roundRect(ctx, 14, 38, 18, 3, 1)
    ctx.fill()

    // Body edge
    ctx.strokeStyle = '#aaa'
    ctx.lineWidth = 0.5
    roundRect(ctx, 12, 12, 22, 32, 3)
    ctx.stroke()

    // Highlight reflection
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(15, 15); ctx.lineTo(15, 40)
    ctx.stroke()
  },

  bookshelf(ctx, S) {
    // Back panel
    ctx.fillStyle = '#8a6d40'
    ctx.fillRect(1, 1, 46, 46)

    // Shelf frame — dark wood
    const frameGrad = ctx.createLinearGradient(0, 0, 0, 48)
    frameGrad.addColorStop(0, '#a08050')
    frameGrad.addColorStop(1, '#7a5d35')
    ctx.fillStyle = frameGrad

    // Outer frame
    ctx.fillRect(0, 0, 48, 3)   // top
    ctx.fillRect(0, 45, 48, 3)  // bottom
    ctx.fillRect(0, 0, 3, 48)   // left
    ctx.fillRect(45, 0, 3, 48)  // right

    // Shelves (4 rows)
    for (const y of [12, 23, 34]) {
      ctx.fillStyle = '#a08050'
      ctx.fillRect(2, y, 44, 3)
      // Shelf edge highlight
      ctx.fillStyle = '#c0a870'
      ctx.fillRect(2, y, 44, 1)
    }

    // Books on shelves — varied colors and heights
    const bookColors = ['#cc4444', '#4488bb', '#8866aa', '#e8a855', '#5cb86e', '#d4926e', '#6bb87b', '#5bbad5']
    const shelves = [
      { y: 4, h: 8 },
      { y: 15, h: 8 },
      { y: 26, h: 8 },
    ]
    for (const shelf of shelves) {
      let x = 4
      let bookIdx = shelf.y  // vary starting color per shelf
      while (x < 43) {
        const bw = 2 + (bookIdx % 3)  // book width 2-4
        const bh = shelf.h - (bookIdx % 3)  // vary height
        const color = bookColors[bookIdx % bookColors.length]
        ctx.fillStyle = color
        roundRect(ctx, x, shelf.y + (shelf.h - bh), bw, bh, 0.5)
        ctx.fill()
        // Spine line
        ctx.strokeStyle = 'rgba(0,0,0,0.2)'
        ctx.lineWidth = 0.3
        ctx.beginPath()
        ctx.moveTo(x + bw / 2, shelf.y + (shelf.h - bh) + 1)
        ctx.lineTo(x + bw / 2, shelf.y + shelf.h - 1)
        ctx.stroke()
        x += bw + 1
        bookIdx++
      }
    }

    // Bottom shelf: decorative items
    ctx.fillStyle = '#6bb87b'  // small plant
    ctx.beginPath()
    ctx.ellipse(10, 38, 4, 3, 0, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = '#c97b5e'
    ctx.fillRect(8, 40, 4, 4) // tiny pot

    ctx.fillStyle = '#ffcc66'  // trophy/cup
    ctx.fillRect(30, 38, 3, 6)
    ctx.fillRect(28, 37, 7, 2)
  },

  meetingTable(ctx, S) {
    // Shadow
    ctx.fillStyle = 'rgba(0,0,0,0.06)'
    ctx.beginPath()
    ctx.ellipse(24, 26, 20, 12, 0, 0, Math.PI * 2)
    ctx.fill()

    // Table surface — large oval
    const tableGrad = ctx.createLinearGradient(4, 10, 4, 38)
    tableGrad.addColorStop(0, '#dcc8a0')
    tableGrad.addColorStop(0.5, '#c8a878')
    tableGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = tableGrad
    ctx.beginPath()
    ctx.ellipse(24, 24, 20, 11, 0, 0, Math.PI * 2)
    ctx.fill()

    // Surface highlight
    ctx.fillStyle = 'rgba(255,255,255,0.12)'
    ctx.beginPath()
    ctx.ellipse(22, 20, 14, 6, -0.2, 0, Math.PI * 2)
    ctx.fill()

    // Wood grain on surface
    ctx.strokeStyle = 'rgba(139,110,70,0.2)'
    ctx.lineWidth = 0.5
    for (let i = 0; i < 3; i++) {
      ctx.beginPath()
      ctx.ellipse(24, 24 - 3 + i * 3, 16 - i * 2, 7 - i, 0, 0, Math.PI * 2)
      ctx.stroke()
    }

    // Edge rim
    ctx.strokeStyle = '#8a6d40'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.ellipse(24, 24, 20, 11, 0, 0, Math.PI * 2)
    ctx.stroke()

    // Center decoration (papers / laptop)
    ctx.fillStyle = '#e8e8f0'
    ctx.fillRect(18, 20, 6, 4)  // paper
    ctx.fillStyle = '#444'
    ctx.fillRect(26, 19, 6, 5)  // laptop
    ctx.fillStyle = '#5baad5'
    ctx.fillRect(27, 20, 4, 3)  // laptop screen
  },

  whiteboard(ctx, S) {
    // Frame
    ctx.fillStyle = '#888'
    roundRect(ctx, 1, 1, 46, 42, 2)
    ctx.fill()

    // White surface
    ctx.fillStyle = '#f5f5f8'
    roundRect(ctx, 3, 3, 42, 36, 1)
    ctx.fill()

    // Grid lines (faint)
    ctx.strokeStyle = 'rgba(0,0,0,0.04)'
    ctx.lineWidth = 0.5
    for (let x = 8; x < 44; x += 8) {
      ctx.beginPath()
      ctx.moveTo(x, 4); ctx.lineTo(x, 38)
      ctx.stroke()
    }
    for (let y = 8; y < 38; y += 8) {
      ctx.beginPath()
      ctx.moveTo(4, y); ctx.lineTo(44, y)
      ctx.stroke()
    }

    // Written content — colorful diagrams/text
    ctx.strokeStyle = '#cc4444'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.moveTo(8, 10); ctx.lineTo(20, 10)
    ctx.stroke()

    ctx.strokeStyle = '#4488bb'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.rect(8, 14, 12, 8)
    ctx.stroke()
    ctx.beginPath()
    ctx.moveTo(20, 18); ctx.lineTo(28, 18)
    ctx.stroke()
    ctx.beginPath()
    ctx.rect(28, 14, 10, 8)
    ctx.stroke()

    ctx.strokeStyle = '#e8a855'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(8, 26); ctx.lineTo(22, 26)
    ctx.moveTo(8, 30); ctx.lineTo(18, 30)
    ctx.moveTo(8, 34); ctx.lineTo(25, 34)
    ctx.stroke()

    // Marker tray
    ctx.fillStyle = '#aaa'
    roundRect(ctx, 8, 40, 32, 4, 1)
    ctx.fill()

    // Markers
    const markerColors = ['#cc4444', '#4488bb', '#5cb86e', '#333']
    for (let i = 0; i < 4; i++) {
      ctx.fillStyle = markerColors[i]
      roundRect(ctx, 10 + i * 7, 41, 5, 2.5, 0.5)
      ctx.fill()
    }

    // Frame edge
    ctx.strokeStyle = '#777'
    ctx.lineWidth = 1
    roundRect(ctx, 1, 1, 46, 42, 2)
    ctx.stroke()
  },

  coffeeBar(ctx, S) {
    // Counter top
    const counterGrad = ctx.createLinearGradient(0, 2, 0, 20)
    counterGrad.addColorStop(0, '#dcc8a0')
    counterGrad.addColorStop(1, '#a08050')
    ctx.fillStyle = counterGrad
    roundRect(ctx, 1, 2, 46, 16, 2)
    ctx.fill()

    // Counter surface wood grain
    drawWoodGrain(ctx, 3, 3, 42, 10, 'rgba(139,110,70,0.25)', 3)

    // Counter edge
    ctx.fillStyle = '#8a6d40'
    ctx.fillRect(1, 16, 46, 2)

    // Front cabinet
    const cabGrad = ctx.createLinearGradient(0, 18, 0, 44)
    cabGrad.addColorStop(0, '#c8a878')
    cabGrad.addColorStop(1, '#8a6d40')
    ctx.fillStyle = cabGrad
    ctx.fillRect(1, 18, 46, 26)

    // Cabinet doors
    ctx.strokeStyle = '#7a5d35'
    ctx.lineWidth = 0.5
    ctx.strokeRect(3, 20, 20, 22)
    ctx.strokeRect(25, 20, 20, 22)

    // Door handles
    ctx.fillStyle = '#d4a850'
    roundRect(ctx, 21, 28, 2, 6, 1)
    ctx.fill()
    roundRect(ctx, 25, 28, 2, 6, 1)
    ctx.fill()

    // Coffee machine on counter
    ctx.fillStyle = '#444'
    roundRect(ctx, 4, 3, 12, 12, 2)
    ctx.fill()
    ctx.fillStyle = '#666'
    roundRect(ctx, 6, 5, 8, 6, 1)
    ctx.fill()
    // Steam nozzle
    ctx.fillStyle = '#888'
    ctx.fillRect(9, 11, 2, 3)
    // LED
    ctx.fillStyle = '#00ff41'
    ctx.beginPath()
    ctx.arc(14, 6, 1, 0, Math.PI * 2)
    ctx.fill()

    // Cups on counter
    ctx.fillStyle = '#f0e6d3'
    for (let i = 0; i < 3; i++) {
      roundRect(ctx, 20 + i * 8, 6, 5, 7, 1)
      ctx.fill()
      // Coffee inside
      ctx.fillStyle = '#6b4a30'
      ctx.fillRect(21 + i * 8, 7, 3, 2)
      ctx.fillStyle = '#f0e6d3'
    }

    // Gold accent strip
    ctx.fillStyle = '#d4a850'
    ctx.fillRect(1, 1, 46, 1)

    // Counter bottom edge
    ctx.fillStyle = '#7a5d35'
    ctx.fillRect(1, 44, 46, 2)
  },

  sofa(ctx, S) {
    // Shadow
    ctx.fillStyle = 'rgba(0,0,0,0.06)'
    roundRect(ctx, 6, 10, 36, 32, 4)
    ctx.fill()

    // Back rest
    const backGrad = ctx.createLinearGradient(6, 6, 6, 18)
    backGrad.addColorStop(0, '#7a5d35')
    backGrad.addColorStop(1, '#6b4e2a')
    ctx.fillStyle = backGrad
    roundRect(ctx, 6, 6, 36, 14, 4)
    ctx.fill()

    // Seat base
    const seatGrad = ctx.createLinearGradient(6, 18, 6, 38)
    seatGrad.addColorStop(0, '#c97b5e')
    seatGrad.addColorStop(0.5, '#bb6644')
    seatGrad.addColorStop(1, '#a05535')
    ctx.fillStyle = seatGrad
    roundRect(ctx, 6, 18, 36, 20, 3)
    ctx.fill()

    // Cushion divider
    ctx.strokeStyle = '#8a5533'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(24, 19); ctx.lineTo(24, 36)
    ctx.stroke()

    // Cushion highlights
    ctx.fillStyle = 'rgba(255,255,255,0.08)'
    roundRect(ctx, 9, 20, 13, 14, 2)
    ctx.fill()
    roundRect(ctx, 26, 20, 13, 14, 2)
    ctx.fill()

    // Armrests
    ctx.fillStyle = '#7a5d35'
    roundRect(ctx, 3, 10, 5, 26, 2)
    ctx.fill()
    roundRect(ctx, 40, 10, 5, 26, 2)
    ctx.fill()

    // Armrest highlights
    ctx.fillStyle = 'rgba(255,255,255,0.1)'
    ctx.fillRect(4, 12, 2, 20)
    ctx.fillRect(41, 12, 2, 20)

    // Legs
    ctx.fillStyle = '#5a4020'
    ctx.fillRect(8, 38, 3, 6)
    ctx.fillRect(37, 38, 3, 6)

    // Sofa outline
    ctx.strokeStyle = '#5a4020'
    ctx.lineWidth = 0.5
    roundRect(ctx, 6, 6, 36, 32, 4)
    ctx.stroke()

    // Throw pillow
    ctx.fillStyle = '#e8a855'
    roundRect(ctx, 11, 12, 8, 6, 2)
    ctx.fill()
    ctx.strokeStyle = '#c89040'
    ctx.lineWidth = 0.5
    roundRect(ctx, 11, 12, 8, 6, 2)
    ctx.stroke()
  },

  chair(ctx, S) {
    // Chair back
    ctx.fillStyle = '#3a3a3a'
    roundRect(ctx, S * 0.25, S * 0.25, S * 0.5, S * 0.32, 4)
    ctx.fill()
    // Chair seat
    ctx.fillStyle = '#4a4a4a'
    roundRect(ctx, S * 0.2, S * 0.55, S * 0.6, S * 0.15, 3)
    ctx.fill()
    // Chair legs
    ctx.fillStyle = '#2a2a2a'
    ctx.fillRect(S * 0.25, S * 0.7, 3, S * 0.15)
    ctx.fillRect(S * 0.72, S * 0.7, 3, S * 0.15)
    // Wheels
    ctx.fillStyle = '#222'
    ctx.beginPath(); ctx.arc(S * 0.26, S * 0.88, 3, 0, Math.PI * 2); ctx.fill()
    ctx.beginPath(); ctx.arc(S * 0.74, S * 0.88, 3, 0, Math.PI * 2); ctx.fill()
    ctx.beginPath(); ctx.arc(S * 0.5, S * 0.9, 3, 0, Math.PI * 2); ctx.fill()
  },

  largePlant(ctx, S) {
    // Large decorative pot
    const potGrad = ctx.createLinearGradient(10, 30, 36, 30)
    potGrad.addColorStop(0, '#c97b5e')
    potGrad.addColorStop(0.5, '#d4926e')
    potGrad.addColorStop(1, '#a06040')
    ctx.fillStyle = potGrad
    ctx.beginPath()
    ctx.moveTo(12, 30)
    ctx.lineTo(14, 46)
    ctx.lineTo(32, 46)
    ctx.lineTo(34, 30)
    ctx.closePath()
    ctx.fill()

    // Pot rim
    ctx.fillStyle = '#b06848'
    roundRect(ctx, 10, 28, 26, 4, 1)
    ctx.fill()

    // Decorative band on pot
    ctx.strokeStyle = '#d4a850'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(14, 36); ctx.lineTo(32, 36)
    ctx.stroke()

    // Dirt
    ctx.fillStyle = '#5a3d20'
    ctx.beginPath()
    ctx.ellipse(23, 29, 11, 2.5, 0, 0, Math.PI * 2)
    ctx.fill()

    // Trunk / stems
    ctx.strokeStyle = '#5a7040'
    ctx.lineWidth = 3
    ctx.beginPath()
    ctx.moveTo(23, 28); ctx.lineTo(23, 14)
    ctx.stroke()
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(23, 20); ctx.lineTo(15, 10)
    ctx.moveTo(23, 18); ctx.lineTo(32, 8)
    ctx.moveTo(23, 14); ctx.lineTo(18, 4)
    ctx.stroke()

    // Large leaf clusters
    const clusters = [
      { cx: 23, cy: 10, rx: 12, ry: 8, color: '#4a9e5c' },
      { cx: 15, cy: 8, rx: 8, ry: 6, color: '#5cb86e' },
      { cx: 32, cy: 6, rx: 8, ry: 6, color: '#5cb86e' },
      { cx: 18, cy: 3, rx: 7, ry: 5, color: '#66cc88' },
      { cx: 28, cy: 4, rx: 6, ry: 4, color: '#66cc88' },
      { cx: 23, cy: 6, rx: 10, ry: 5, color: '#78dda0' },
    ]
    for (const cl of clusters) {
      ctx.fillStyle = cl.color
      ctx.beginPath()
      ctx.ellipse(cl.cx, cl.cy, cl.rx, cl.ry, 0, 0, Math.PI * 2)
      ctx.fill()
    }

    // Leaf detail veins
    ctx.strokeStyle = 'rgba(0,80,20,0.15)'
    ctx.lineWidth = 0.5
    ctx.beginPath()
    ctx.moveTo(23, 14); ctx.lineTo(23, 4)
    ctx.moveTo(15, 10); ctx.lineTo(10, 4)
    ctx.moveTo(32, 8); ctx.lineTo(36, 2)
    ctx.stroke()
  },
}

// Programmatic env sprite drawing
const ENV_DRAW = {
  baseboard(ctx, S) {
    // Baseboard — decorative wall trim
    const y = Math.floor(S * 0.35)
    const h = Math.floor(S * 0.3)

    // Main board
    const grad = ctx.createLinearGradient(0, y, 0, y + h)
    grad.addColorStop(0, '#c8a878')
    grad.addColorStop(0.3, '#a08050')
    grad.addColorStop(1, '#8a6d40')
    ctx.fillStyle = grad
    ctx.fillRect(0, y, S, h)

    // Top molding
    ctx.fillStyle = '#dcc8a0'
    ctx.fillRect(0, y, S, 2)

    // Bottom edge
    ctx.fillStyle = '#7a5d35'
    ctx.fillRect(0, y + h - 1, S, 1)

    // Groove pattern
    ctx.strokeStyle = 'rgba(139,110,70,0.3)'
    ctx.lineWidth = 0.5
    for (let x = 6; x < S; x += 12) {
      ctx.beginPath()
      ctx.moveTo(x, y + 3)
      ctx.lineTo(x, y + h - 2)
      ctx.stroke()
    }
  },

  rugPattern(ctx, S) {
    const cx = S / 2
    const cy = S / 2

    // Diamond shape
    ctx.beginPath()
    ctx.moveTo(cx, 3)
    ctx.lineTo(S - 3, cy)
    ctx.lineTo(cx, S - 3)
    ctx.lineTo(3, cy)
    ctx.closePath()

    // Fill with warm gradient
    const grad = ctx.createRadialGradient(cx, cy, 2, cx, cy, S / 2)
    grad.addColorStop(0, '#d4a06a')
    grad.addColorStop(0.5, '#c89060')
    grad.addColorStop(1, '#a07040')
    ctx.fillStyle = grad
    ctx.fill()

    // Border
    ctx.strokeStyle = '#8a6040'
    ctx.lineWidth = 1.5
    ctx.stroke()

    // Inner diamond
    ctx.beginPath()
    ctx.moveTo(cx, 8)
    ctx.lineTo(S - 8, cy)
    ctx.lineTo(cx, S - 8)
    ctx.lineTo(8, cy)
    ctx.closePath()
    ctx.strokeStyle = '#c8a060'
    ctx.lineWidth = 0.8
    ctx.stroke()

    // Center motif
    ctx.fillStyle = '#8a6040'
    ctx.beginPath()
    ctx.arc(cx, cy, 3, 0, Math.PI * 2)
    ctx.fill()
  },
}

// ── Desk decoration mini-sprites (drawn at ~10-14px scale on desk surface) ───
const DESK_DECORATIONS = {
  miniPlant(ctx, x, y) {
    // Tiny pot
    ctx.fillStyle = '#c97b5e'
    ctx.fillRect(x, y + 5, 7, 5)
    // Leaves
    ctx.fillStyle = '#5cb86e'
    ctx.beginPath()
    ctx.ellipse(x + 3, y + 3, 5, 4, 0, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = '#78dda0'
    ctx.beginPath()
    ctx.ellipse(x + 4, y + 1, 3, 2.5, 0, 0, Math.PI * 2)
    ctx.fill()
  },
  photoFrame(ctx, x, y) {
    // Frame
    ctx.fillStyle = '#8a6d40'
    ctx.fillRect(x, y, 10, 9)
    // Photo
    ctx.fillStyle = '#b8d8f0'
    ctx.fillRect(x + 1, y + 1, 8, 7)
    // Silhouette
    ctx.fillStyle = '#6a9cc0'
    ctx.beginPath()
    ctx.arc(x + 5, y + 3, 2, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillRect(x + 3, y + 5, 4, 3)
  },
  figurine(ctx, x, y) {
    // Base
    ctx.fillStyle = '#ddd'
    ctx.fillRect(x + 1, y + 7, 6, 3)
    // Body
    ctx.fillStyle = '#e8a855'
    ctx.fillRect(x + 2, y + 3, 4, 5)
    // Head
    ctx.fillStyle = '#f0d0a0'
    ctx.beginPath()
    ctx.arc(x + 4, y + 2, 2.5, 0, Math.PI * 2)
    ctx.fill()
  },
  mug(ctx, x, y) {
    // Cup body
    ctx.fillStyle = '#f0e6d3'
    roundRect(ctx, x, y + 2, 7, 8, 1)
    ctx.fill()
    // Coffee inside
    ctx.fillStyle = '#6b4a30'
    ctx.fillRect(x + 1, y + 3, 5, 2)
    // Handle
    ctx.strokeStyle = '#ddd'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.arc(x + 8, y + 6, 3, -Math.PI * 0.5, Math.PI * 0.5)
    ctx.stroke()
  },
  bookStack(ctx, x, y) {
    const colors = ['#cc4444', '#4488bb', '#8866aa', '#e8a855']
    for (let i = 0; i < 4; i++) {
      ctx.fillStyle = colors[i]
      ctx.fillRect(x, y + i * 2.5, 9, 2)
    }
  },
  deskLamp(ctx, x, y) {
    // Arm
    ctx.strokeStyle = '#666'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.moveTo(x + 4, y + 10)
    ctx.lineTo(x + 3, y + 4)
    ctx.lineTo(x + 7, y + 1)
    ctx.stroke()
    // Shade
    ctx.fillStyle = '#444'
    ctx.beginPath()
    ctx.moveTo(x + 3, y)
    ctx.lineTo(x + 11, y)
    ctx.lineTo(x + 9, y + 3)
    ctx.lineTo(x + 5, y + 3)
    ctx.closePath()
    ctx.fill()
    // Base
    ctx.fillStyle = '#555'
    ctx.fillRect(x + 1, y + 10, 6, 2)
    // Glow
    ctx.fillStyle = 'rgba(255,240,180,0.2)'
    ctx.beginPath()
    ctx.ellipse(x + 7, y + 4, 5, 3, 0, 0, Math.PI * 2)
    ctx.fill()
  },
}

const DECORATION_NAMES = Object.keys(DESK_DECORATIONS)

// Deterministic hash for worker → decoration selection
function hashString(str) {
  let h = 0
  for (let i = 0; i < str.length; i++) {
    h = ((h << 5) - h) + str.charCodeAt(i)
    h |= 0
  }
  return Math.abs(h)
}

export function getWorkerDecorations(workerName, skillProfile) {
  const seed = hashString(workerName || 'default')
  const count = 2 + (seed % 2)  // 2 or 3 decorations
  const decorations = []
  const used = new Set()
  for (let i = 0; i < count; i++) {
    let idx = (seed + i * 7) % DECORATION_NAMES.length
    while (used.has(idx)) idx = (idx + 1) % DECORATION_NAMES.length
    used.add(idx)
    decorations.push(DECORATION_NAMES[idx])
  }
  return decorations
}

export function drawDeskDecoration(ctx, name, x, y) {
  if (DESK_DECORATIONS[name]) DESK_DECORATIONS[name](ctx, x, y)
}

export function prerenderWallVariants() {
  const S = DRAW_SIZE // 48
  const variants = []
  for (let mask = 0; mask < 16; mask++) {
    const c = document.createElement('canvas')
    c.width = S; c.height = S
    const ctx = c.getContext('2d')
    // Base fill
    ctx.fillStyle = '#d4c4a8'
    ctx.fillRect(0, 0, S, S)
    // Panel texture
    ctx.fillStyle = '#dcc8a0'
    ctx.fillRect(2, 4, S - 4, S - 8)
    // Borders on sides without adjacent wall
    const hasN = mask & 1, hasE = mask & 2, hasS = mask & 4, hasW = mask & 8
    ctx.fillStyle = '#b8a882'
    if (!hasN) ctx.fillRect(0, 0, S, 3)        // top border
    if (!hasS) ctx.fillRect(0, S - 3, S, 3)    // bottom border
    if (!hasW) ctx.fillRect(0, 0, 3, S)        // left border
    if (!hasE) ctx.fillRect(S - 3, 0, 3, S)    // right border
    // Top decorative strip (no wall above)
    if (!hasN) {
      ctx.fillStyle = '#c9b896'
      ctx.fillRect(0, 0, S, 2)
    }
    // Bottom baseboard (no wall below)
    if (!hasS) {
      ctx.fillStyle = '#8b7355'
      ctx.fillRect(0, S - 2, S, 2)
    }
    variants.push(c)
  }
  return variants
}

export function prerenderFurniture(name) {
  const S = DRAW_SIZE
  const offscreen = document.createElement('canvas')
  offscreen.width = S
  offscreen.height = S
  const ctx = offscreen.getContext('2d')

  // Use programmatic drawing if available, else fall back to pixel-string
  if (FURNITURE_DRAW[name]) {
    FURNITURE_DRAW[name](ctx, S)
  } else {
    const rows = FURNITURE_SPRITES[name]
    if (!rows) return null
    const scale = 3
    renderSpriteToCanvas(ctx, rows, null, 0, 0, scale)
  }
  return offscreen
}

export function prerenderEnvSprite(name) {
  const S = DRAW_SIZE
  const offscreen = document.createElement('canvas')
  offscreen.width = S
  offscreen.height = S
  const ctx = offscreen.getContext('2d')

  // Use programmatic drawing if available, else fall back to pixel-string
  if (ENV_DRAW[name]) {
    ENV_DRAW[name](ctx, S)
  } else {
    const rows = ENV_SPRITES[name]
    if (!rows) return null
    const scale = 3
    renderSpriteToCanvas(ctx, rows, null, 0, 0, scale)
  }
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
