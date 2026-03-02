// Pixel Office — JRPG-style 16×16 sprite data for characters & hacker-base furniture
// Each sprite row is a 16-char string mapped to an expanded palette.

// ── Expanded Character Palettes (11+ colors per class) ──────────────────────
const PALETTES = {
  coder: {
    skin: '#fdd', skinShade: '#dbb', skinHi: '#fee',
    hair: '#543', hairShade: '#321', hairHi: '#876',
    shirt: '#48f', shirtShade: '#26a', shirtHi: '#6af',
    pants: '#336', pantsShade: '#224',
    shoes: '#222', eye: '#111', eyeHi: '#fff', accent: '#ff4444',
  },
  hacker: {
    skin: '#fed', skinShade: '#dca', skinHi: '#ffe',
    hair: '#111', hairShade: '#000', hairHi: '#333',
    shirt: '#1a1', shirtShade: '#070', shirtHi: '#3d3',
    pants: '#222', pantsShade: '#111',
    shoes: '#111', eye: '#00ff41', eyeHi: '#aaffaa', accent: '#00ff41',
  },
  designer: {
    skin: '#fec', skinShade: '#dca', skinHi: '#ffe',
    hair: '#f80', hairShade: '#c50', hairHi: '#fb3',
    shirt: '#f4a', shirtShade: '#c27', shirtHi: '#f8d',
    pants: '#537', pantsShade: '#325',
    shoes: '#433', eye: '#111', eyeHi: '#fff', accent: '#ff44ff',
  },
  analyst: {
    skin: '#edb', skinShade: '#cb9', skinHi: '#fed',
    hair: '#654', hairShade: '#432', hairHi: '#987',
    shirt: '#fff', shirtShade: '#ccc', shirtHi: '#fff',
    pants: '#447', pantsShade: '#225',
    shoes: '#333', eye: '#111', eyeHi: '#fff', accent: '#88ccff',
  },
  architect: {
    skin: '#fdd', skinShade: '#dbb', skinHi: '#fee',
    hair: '#888', hairShade: '#555', hairHi: '#bbb',
    shirt: '#669', shirtShade: '#447', shirtHi: '#88b',
    pants: '#334', pantsShade: '#112',
    shoes: '#222', eye: '#111', eyeHi: '#fff', accent: '#cc88ff',
  },
  devops: {
    skin: '#dc9', skinShade: '#ba7', skinHi: '#fed',
    hair: '#320', hairShade: '#100', hairHi: '#653',
    shirt: '#f62', shirtShade: '#c30', shirtHi: '#f94',
    pants: '#333', pantsShade: '#111',
    shoes: '#222', eye: '#111', eyeHi: '#fff', accent: '#ffaa00',
  },
  researcher: {
    skin: '#fdd', skinShade: '#dbb', skinHi: '#fee',
    hair: '#654', hairShade: '#432', hairHi: '#987',
    shirt: '#28d', shirtShade: '#06a', shirtHi: '#4af',
    pants: '#446', pantsShade: '#224',
    shoes: '#333', eye: '#111', eyeHi: '#fff', accent: '#44ddff',
  },
  reviewer: {
    skin: '#edb', skinShade: '#cb9', skinHi: '#fed',
    hair: '#543', hairShade: '#321', hairHi: '#876',
    shirt: '#2a6', shirtShade: '#084', shirtHi: '#4c8',
    pants: '#345', pantsShade: '#123',
    shoes: '#222', eye: '#111', eyeHi: '#fff', accent: '#44ff88',
  },
}

// ── Color Map: char → palette key ───────────────────────────────────────────
// Uppercase = shade, lowercase non-digit = highlight
const COLOR_MAP = {
  '1': 'hair',     'H': 'hairShade',   'h': 'hairHi',
  '2': 'skin',     'S': 'skinShade',   's': 'skinHi',
  '3': 'shirt',    'T': 'shirtShade',  't': 'shirtHi',
  '4': 'pants',    'P': 'pantsShade',
  '5': 'shoes',
  '6': 'eye',      'e': 'eyeHi',
  '7': 'outline',  // resolved to #111 in renderSpriteToCanvas
  'A': 'accent',
}

// ── Base Character Frames (generic humanoid, used as fallback) ──────────────
// JRPG style: 7=outline wrapping all body parts, shade/highlight baked in
const CHARACTER_FRAMES = {
  idle: [
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
    [
      '0000000000000000',
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
      '0000075570000000',
      '0000750057000000',
    ],
  ],
  working: [
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000273t337000000',
      '0027t33T37000000',
      '0072t33T37000000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337200000',
      '000073t33T720000',
      '000073t33T270000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000273t337200000',
      '00273t33T3720000',
      '0007333337000000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
  ],
  waiting: [
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
      '0000075557000000',
    ],
  ],
  error: [
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0270073370072000',
      '0270733337072000',
      '00073t33T3700000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
  ],
  finished: [
    [
      '0000077770000000',
      '00007h11H7000000',
      '0007h111HH700000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370072000',
      '000073t337072000',
      '000073t33T270000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    [
      '0027077770027000',
      '0027h11H77027000',
      '00271111HH270000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '000073t337000000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ],
    [
      '0000000000000000',
      '0027077770027000',
      '0027h11H77027000',
      '00271111HH270000',
      '0007762267700000',
      '00072s22S2700000',
      '0000722227000000',
      '0000073370000000',
      '000073t337000000',
      '000073t337000000',
      '00073t33T3700000',
      '0000733337000000',
      '0000074470000000',
      '000074P447000000',
      '0000000000000000',
      '0000750057000000',
    ],
  ],
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
}

// ── Class-Specific Frame Overrides ──────────────────────────────────────────
// Each class overrides idle[0] with a unique silhouette; other frames derive
// from the base CHARACTER_FRAMES above.
const CLASS_FRAME_OVERRIDES = {
  coder: {
    // Hoodie + headphones (A=accent headphone color)
    idle: [[
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
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ]],
  },
  hacker: {
    // Hood + face mask, matrix-green eyes
    idle: [[
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
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ]],
  },
  designer: {
    // Beret + relaxed wide stance
    idle: [[
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
      '0000074470000000',
      '0000755570000000',
      '0007500057000000',
    ]],
  },
  analyst: {
    // Glasses + tie
    idle: [[
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
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ]],
  },
  architect: {
    // Long coat / cape
    idle: [[
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
      '000A074470A00000',
      '0000075570000000',
      '0000750057000000',
    ]],
  },
  devops: {
    // Hard hat + goggles
    idle: [[
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
      '0000074470000000',
      '0000075570000000',
      '0000750057000000',
    ]],
  },
}

// ── Hacker-Base Furniture Sprites ───────────────────────────────────────────
const FURNITURE_SPRITES = {
  desk: [  // hackerStation — triple monitor workstation
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
  computer: [  // holoDisplay — floating holographic screen
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
  plant: [  // serverRack — server rack with blinking LEDs
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
  watercooler: [  // vendingMachine — energy drink vending machine
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
  bookshelf: [  // wallOfScreens — multi-screen display wall
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
}

// Extra environment sprites for tile rendering
const ENV_SPRITES = {
  glowStrip: [  // neon glow strip along walls
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
  cableFloor: [  // floor cables
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

// ── Furniture Palette (dark metal hacker-base) ──────────────────────────────
const FURNITURE_PALETTE = {
  '7': '#2a2a3a',  // metal frame
  '8': '#1a1a2e',  // dark surface
  '9': '#00ff41',  // LED green
  'b': '#00ddff',  // cyan glow
  '1': '#44eeff',  // screen
  '3': '#ff4444',  // LED red
  'c': '#00ff41',  // neon green
  'd': '#ff00ff',  // neon pink
}

const AVATAR_TO_CHAR = {
  'coder': 'coder',
  'hacker': 'hacker',
  'designer': 'designer',
  'analyst': 'analyst',
  'architect': 'architect',
  'devops': 'devops',
}
const CHARACTER_NAMES = Object.keys(PALETTES)

export function getCharacterType(worker) {
  if (worker.avatar && AVATAR_TO_CHAR[worker.avatar]) return AVATAR_TO_CHAR[worker.avatar]
  let hash = 0
  const id = worker.id || worker.name || ''
  for (let i = 0; i < id.length; i++) {
    hash = ((hash << 5) - hash) + id.charCodeAt(i)
    hash |= 0
  }
  return CHARACTER_NAMES[Math.abs(hash) % CHARACTER_NAMES.length]
}

// Build merged frames: base + class overrides
function getFramesForClass(charType) {
  const overrides = CLASS_FRAME_OVERRIDES[charType]
  if (!overrides) return CHARACTER_FRAMES
  const merged = {}
  for (const [state, frames] of Object.entries(CHARACTER_FRAMES)) {
    if (overrides[state]) {
      // Replace first N frames from override, keep remaining from base
      const oFrames = overrides[state]
      merged[state] = oFrames.concat(frames.slice(oFrames.length))
    } else {
      merged[state] = frames
    }
  }
  return merged
}

export function renderSpriteToCanvas(ctx, rows, palette, x, y, scale) {
  for (let row = 0; row < rows.length; row++) {
    const line = rows[row]
    for (let col = 0; col < line.length; col++) {
      const ch = line[col]
      if (ch === '0') continue
      let color
      if (ch === '7') {
        color = '#111'
      } else if (palette && COLOR_MAP[ch]) {
        color = palette[COLOR_MAP[ch]]
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

export function prerenderCharacter(charType) {
  const palette = PALETTES[charType]
  if (!palette) return null
  const frames = getFramesForClass(charType)
  const cache = {}
  const scale = 1
  const size = 16 * scale
  for (const [state, stateFrames] of Object.entries(frames)) {
    cache[state] = stateFrames.map(rows => {
      const offscreen = document.createElement('canvas')
      offscreen.width = size
      offscreen.height = size
      const ctx = offscreen.getContext('2d')
      renderSpriteToCanvas(ctx, rows, palette, 0, 0, scale)
      return offscreen
    })
  }
  return cache
}

export function prerenderFurniture(name) {
  const rows = FURNITURE_SPRITES[name]
  if (!rows) return null
  const scale = 1
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
  const offscreen = document.createElement('canvas')
  offscreen.width = 16
  offscreen.height = 16
  const ctx = offscreen.getContext('2d')
  renderSpriteToCanvas(ctx, rows, null, 0, 0, 1)
  return offscreen
}

// Skill profile → accent color and icon for office rendering
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

export { PALETTES, CHARACTER_FRAMES, FURNITURE_SPRITES, ENV_SPRITES, CHARACTER_NAMES, CLASS_FRAME_OVERRIDES }
