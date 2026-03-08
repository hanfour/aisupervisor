// characterPortrait.js
// Generates 32×32 pixel-art character portraits for each skill profile.
// Uses Canvas 2D API — no external assets required.

const CANVAS_SIZE = 32

// ── Drawing helpers ────────────────────────────────────────────────────────────

function fill(ctx, x, y, w, h, color) {
  ctx.fillStyle = color
  ctx.fillRect(x, y, w, h)
}

function px(ctx, x, y, color) {
  ctx.fillStyle = color
  ctx.fillRect(x, y, 1, 1)
}

// ── Skin tones ─────────────────────────────────────────────────────────────────

const SKINS = {
  light:  { base: '#f5c5a3', shadow: '#d4956e', dark: '#b07050' },
  medium: { base: '#c8843a', shadow: '#9a6020', dark: '#7a4010' },
  warm:   { base: '#e8a87a', shadow: '#c07850', dark: '#a05030' },
  tan:    { base: '#d4a070', shadow: '#b07848', dark: '#906030' },
}

// ── Per-profile style definitions ──────────────────────────────────────────────

const PROFILE_STYLES = {
  coder: {
    skin: SKINS.light,
    hair: '#1e1e2e',         // near-black blue-tinted
    hairLight: '#3a3a5e',
    hairStyle: 'short',
    outfit: '#3a3a4a',       // dark charcoal hoodie
    outfitLight: '#505068',
    accent: '#00c853',       // terminal green
    eye: '#5ba3ff',          // blue
    hasGlasses: true,
  },
  hacker: {
    skin: SKINS.medium,
    hair: '#1a1a1a',
    hairLight: '#2a2a2a',
    hairStyle: 'hood',
    outfit: '#0d0d0d',       // black hoodie (hood up)
    outfitLight: '#1e1e1e',
    accent: '#00ff41',       // matrix green
    eye: '#00dd33',
    hasSunglasses: true,
  },
  designer: {
    skin: SKINS.light,
    hair: '#8b1a00',         // deep auburn
    hairLight: '#c03020',
    hairStyle: 'long',
    outfit: '#9b4dca',       // purple
    outfitLight: '#b568e8',
    accent: '#ff79c6',       // pink accent
    eye: '#a060d0',          // violet
  },
  analyst: {
    skin: SKINS.tan,
    hair: '#4a3010',         // dark brown
    hairLight: '#6a5020',
    hairStyle: 'medium',
    outfit: '#2c5282',       // navy blue
    outfitLight: '#3a6aa0',
    accent: '#63b3ed',       // light blue
    eye: '#3182ce',
    hasGlasses: true,
  },
  architect: {
    skin: SKINS.light,
    hair: '#888899',         // silver-gray
    hairLight: '#aaaabc',
    hairStyle: 'slicked',
    outfit: '#1a1a2e',       // dark navy suit
    outfitLight: '#2a2a4e',
    accent: '#c9a84c',       // gold
    eye: '#607080',
    hasTie: true,
  },
  devops: {
    skin: SKINS.warm,
    hair: '#cc4400',         // orange-red
    hairLight: '#ff6622',
    hairStyle: 'messy',
    outfit: '#4a5a2a',       // olive green vest
    outfitLight: '#6a7a4a',
    accent: '#e8a855',       // amber
    eye: '#5a8a50',
    hasHeadset: true,
  },
  researcher: {
    skin: SKINS.light,
    hair: '#2a2a2a',
    hairLight: '#4a4a4a',
    hairStyle: 'bun',
    outfit: '#e8e8f0',       // white lab coat
    outfitLight: '#f8f8ff',
    accent: '#5bc0de',       // teal
    eye: '#5b6a7a',
  },
  reviewer: {
    skin: SKINS.warm,
    hair: '#3a2a1a',
    hairLight: '#5a4a2a',
    hairStyle: 'neat',
    outfit: '#2a3a2a',       // dark green vest
    outfitLight: '#3a5a3a',
    accent: '#5ac85a',       // approval green
    eye: '#4a6040',
  },
}

// ── Head / face ────────────────────────────────────────────────────────────────

function drawHead(ctx, skin) {
  const { base, shadow, dark } = skin
  // Oval head approximated with horizontal spans:
  // y=2: narrow top
  fill(ctx, 11, 2, 10, 1, base)
  // y=3: wider
  fill(ctx,  9, 3, 14, 1, base)
  // y=4..11: full width (16px)
  for (let y = 4; y <= 11; y++) {
    fill(ctx, 8, y, 16, 1, base)
  }
  // y=12: taper
  fill(ctx,  9, 12, 14, 1, base)
  // y=13: chin tip
  fill(ctx, 11, 13, 10, 1, base)

  // Right-side shadow (depth illusion)
  for (let y = 4; y <= 11; y++) {
    px(ctx, 23, y, shadow)
  }
  px(ctx, 22, 12, shadow)

  // Cheek blush (subtle)
  px(ctx,  9, 10, shadow)
  px(ctx, 22, 10, shadow)
}

// ── Eyebrows ───────────────────────────────────────────────────────────────────

function drawEyebrows(ctx, hairColor, expression) {
  // Left brow
  fill(ctx, 10, 5, 4, 1, hairColor)
  // Right brow
  fill(ctx, 18, 5, 4, 1, hairColor)

  if (expression === 'focused') {
    // Slight inner lift (determined look)
    px(ctx, 13, 4, hairColor)
    px(ctx, 18, 4, hairColor)
  } else if (expression === 'intense') {
    // Furrowed brows
    fill(ctx, 10, 5, 4, 1, hairColor)
    fill(ctx, 18, 5, 4, 1, hairColor)
    px(ctx, 13, 4, hairColor)
    px(ctx, 18, 4, hairColor)
  } else if (expression === 'creative') {
    // One raised brow
    fill(ctx, 10, 4, 4, 1, hairColor)
    fill(ctx, 18, 5, 4, 1, hairColor)
  }
}

// ── Eyes ───────────────────────────────────────────────────────────────────────

function drawEyes(ctx, eyeColor) {
  const white = '#ffffff'
  const pupil = '#1a1a1a'

  // Left eye (x=10-12, y=7-8)
  fill(ctx, 10, 7, 3, 2, white)
  px(ctx, 11, 7, eyeColor)   // iris
  px(ctx, 11, 8, eyeColor)
  px(ctx, 11, 7, pupil)      // pupil dot
  px(ctx, 12, 7, '#ffffff')  // highlight

  // Right eye (x=19-21, y=7-8)
  fill(ctx, 19, 7, 3, 2, white)
  px(ctx, 20, 7, eyeColor)
  px(ctx, 20, 8, eyeColor)
  px(ctx, 20, 7, pupil)
  px(ctx, 21, 7, '#ffffff')

  // Eyelid outline (top)
  fill(ctx, 10, 6, 3, 1, '#503020')
  fill(ctx, 19, 6, 3, 1, '#503020')
}

// ── Glasses ────────────────────────────────────────────────────────────────────

function drawGlasses(ctx) {
  const frame = '#304050'
  // Left lens frame
  fill(ctx,  9, 6, 5, 1, frame)  // top edge
  fill(ctx,  9, 9, 5, 1, frame)  // bottom edge
  fill(ctx,  9, 6, 1, 4, frame)  // left edge
  fill(ctx, 13, 6, 1, 4, frame)  // right edge
  // Right lens frame
  fill(ctx, 18, 6, 5, 1, frame)
  fill(ctx, 18, 9, 5, 1, frame)
  fill(ctx, 18, 6, 1, 4, frame)
  fill(ctx, 22, 6, 1, 4, frame)
  // Bridge
  fill(ctx, 14, 8, 4, 1, frame)
  // Temples
  fill(ctx,  8, 8, 1, 1, frame)
  fill(ctx, 23, 8, 1, 1, frame)
  // Lens tint (slight)
  fill(ctx, 10, 7, 3, 2, '#d0e8ff44')
  fill(ctx, 19, 7, 3, 2, '#d0e8ff44')
}

function drawSunglasses(ctx) {
  const lens = '#0a0a14'
  const shine = '#1a2a3a'
  const frame = '#222222'
  // Left lens
  fill(ctx,  9, 6, 6, 4, lens)
  px(ctx, 10, 7, shine)
  px(ctx, 11, 7, shine)
  // Right lens
  fill(ctx, 18, 6, 6, 4, lens)
  px(ctx, 19, 7, shine)
  px(ctx, 20, 7, shine)
  // Frame outline
  fill(ctx,  9, 6, 6, 1, frame)
  fill(ctx,  9, 9, 6, 1, frame)
  fill(ctx, 18, 6, 6, 1, frame)
  fill(ctx, 18, 9, 6, 1, frame)
  // Bridge
  fill(ctx, 15, 7, 3, 2, frame)
  // Temples
  fill(ctx,  8, 8, 1, 1, frame)
  fill(ctx, 24, 8, 1, 1, frame)
}

// ── Nose ───────────────────────────────────────────────────────────────────────

function drawNose(ctx, skin) {
  px(ctx, 15, 10, skin.dark)
  px(ctx, 16, 10, skin.dark)
  px(ctx, 14, 11, skin.shadow)
  px(ctx, 17, 11, skin.shadow)
}

// ── Mouth ──────────────────────────────────────────────────────────────────────

function drawMouth(ctx, expression) {
  const lip = '#c07060'
  const lipDark = '#984040'
  const teeth = '#f0f0f0'

  if (expression === 'smile') {
    // Friendly smile
    fill(ctx, 13, 12, 6, 1, lip)
    px(ctx, 12, 12, lipDark)
    px(ctx, 19, 12, lipDark)
    fill(ctx, 13, 13, 6, 1, teeth)
    px(ctx, 12, 13, lip)
    px(ctx, 19, 13, lip)
  } else if (expression === 'smirk') {
    // Confident smirk
    fill(ctx, 14, 12, 6, 1, lip)
    px(ctx, 13, 12, lipDark)
    px(ctx, 13, 13, lip)
    px(ctx, 14, 13, teeth)
    px(ctx, 15, 13, teeth)
  } else {
    // Neutral
    fill(ctx, 13, 12, 6, 1, lip)
    px(ctx, 12, 12, lipDark)
    px(ctx, 19, 12, lipDark)
  }
}

// ── Neck ───────────────────────────────────────────────────────────────────────

function drawNeck(ctx, skin) {
  fill(ctx, 13, 14, 6, 3, skin.base)
  px(ctx, 18, 14, skin.shadow)
  px(ctx, 18, 15, skin.shadow)
}

// ── Hair styles ────────────────────────────────────────────────────────────────

function drawHairShort(ctx, hair, hairLight) {
  // Short, tidy — covers forehead and sides
  // Top
  fill(ctx, 10, 0, 12, 1, hair)
  fill(ctx,  9, 1, 14, 2, hair)
  fill(ctx,  8, 3, 16, 1, hair)   // hairline
  // Side tufts
  fill(ctx,  8, 4, 1, 5, hair)    // left
  fill(ctx, 23, 4, 1, 5, hair)    // right
  // Highlight streak
  fill(ctx, 12, 1, 4, 1, hairLight)
}

function drawHairHood(ctx, hair, outfitColor) {
  // Hood pulled up — covers most of head, only face visible
  fill(ctx,  6, 0, 20, 6, outfitColor)   // top of hood
  fill(ctx,  5, 4, 3, 10, outfitColor)   // left side of hood
  fill(ctx, 24, 4, 3, 10, outfitColor)   // right side of hood
  // Inner hood opening (dark)
  fill(ctx,  8, 4, 3, 2, '#0a0a0a')
  fill(ctx, 21, 4, 3, 2, '#0a0a0a')
  // Tiny hair tuft visible at forehead
  fill(ctx, 11, 3, 4, 1, hair)
}

function drawHairLong(ctx, hair, hairLight) {
  // Long flowing hair — top and long sides
  fill(ctx, 10, 0, 12, 1, hair)
  fill(ctx,  9, 1, 14, 2, hair)
  fill(ctx,  8, 3, 16, 1, hair)
  // Long sides extending past face
  fill(ctx,  7, 4, 2, 14, hair)    // left side, long
  fill(ctx, 23, 4, 2, 14, hair)    // right side, long
  // Volume layers
  fill(ctx,  6, 7, 2, 8, hair)     // extra left volume
  // Highlight
  fill(ctx, 12, 1, 5, 1, hairLight)
  fill(ctx,  7, 6, 1, 4, hairLight)
}

function drawHairMedium(ctx, hair, hairLight) {
  // Medium length, professional
  fill(ctx, 10, 0, 12, 1, hair)
  fill(ctx,  9, 1, 14, 2, hair)
  fill(ctx,  8, 3, 16, 1, hair)
  // Medium sides (to jaw)
  fill(ctx,  8, 4, 1, 9, hair)
  fill(ctx, 23, 4, 1, 9, hair)
  // Highlight
  fill(ctx, 12, 1, 5, 1, hairLight)
}

function drawHairMessy(ctx, hair, hairLight) {
  // Messy/spiky — irregular spikes
  fill(ctx, 10, 0, 12, 2, hair)
  // Spikes going up
  fill(ctx,  9, 0, 1, 2, hair)
  fill(ctx, 12, 0, 2, 1, hairLight)  // highlight spike
  fill(ctx, 16, 0, 2, 1, hair)
  fill(ctx, 20, 0, 2, 1, hair)
  px(ctx, 22, 0, hair)
  fill(ctx, 8, 1, 16, 2, hair)
  fill(ctx, 8, 3, 16, 1, hair)
  // Messy sides
  fill(ctx, 7,  4, 2, 7, hair)
  fill(ctx, 23, 4, 2, 7, hair)
  px(ctx, 7, 6, hairLight)
  px(ctx, 24, 5, hairLight)
}

function drawHairBun(ctx, hair, hairLight) {
  // Bun on top — tight pulled-back look
  // Bun blob
  fill(ctx, 12, 0, 8, 3, hair)
  fill(ctx, 11, 1, 10, 1, hair)
  // Sleek sides (hair pulled back)
  fill(ctx,  8, 3, 16, 1, hair)
  fill(ctx,  8, 4, 1, 8, hair)
  fill(ctx, 23, 4, 1, 8, hair)
  // Bun highlight
  fill(ctx, 14, 0, 3, 1, hairLight)
  px(ctx, 13, 1, hairLight)
}

function drawHairSlicked(ctx, hair, hairLight) {
  // Slicked back / side-parted
  fill(ctx,  9, 0, 14, 1, hair)
  fill(ctx,  8, 1, 16, 2, hair)
  fill(ctx,  8, 3, 16, 1, hair)
  fill(ctx,  8, 4, 1, 4, hair)     // left side
  fill(ctx, 23, 4, 1, 4, hair)     // right side
  // Side part highlight (left side of part)
  fill(ctx,  9, 0, 3, 2, hairLight)
  fill(ctx,  9, 2, 2, 1, hairLight)
  // Part line
  px(ctx, 12, 0, '#00000033')
}

function drawHairNeat(ctx, hair, hairLight) {
  // Neat professional short cut
  fill(ctx, 11, 0, 10, 2, hair)
  fill(ctx,  9, 2, 14, 1, hair)
  fill(ctx,  8, 3, 16, 1, hair)
  fill(ctx,  8, 4, 1, 5, hair)
  fill(ctx, 23, 4, 1, 5, hair)
  // Clean highlight
  fill(ctx, 13, 0, 4, 1, hairLight)
}

// ── Accessories ────────────────────────────────────────────────────────────────

function drawHeadset(ctx, color) {
  // Headphone cups over ears + top band
  fill(ctx,  6, 5, 3, 4, color)   // left cup
  fill(ctx, 23, 5, 3, 4, color)   // right cup
  fill(ctx,  7, 0, 18, 2, color)  // headband (over hair)
  // Mic arm
  fill(ctx,  5, 8, 2, 1, color)
  fill(ctx,  4, 8, 1, 3, color)
  px(ctx, 4, 11, '#e8a855')        // mic tip
}

function drawTie(ctx, tieColor) {
  // Tie in collar region
  fill(ctx, 15, 17, 2, 2, tieColor)  // knot
  fill(ctx, 15, 19, 2, 5, tieColor)  // body
  fill(ctx, 14, 23, 4, 2, tieColor)  // wide end
}

// ── Outfit styles ──────────────────────────────────────────────────────────────

function drawOutfitHoodie(ctx, outfit, outfitLight, accent) {
  // Dark hoodie body
  fill(ctx,  7, 17, 18, 15, outfit)
  // Hood shadow at neckline
  fill(ctx, 10, 17, 12, 2, outfitLight)
  // Kangaroo pocket
  fill(ctx, 11, 25,  10, 5, outfitLight)
  px(ctx, 15, 25, outfit)
  px(ctx, 16, 25, outfit)
  // Drawstrings
  fill(ctx, 14, 17, 1, 5, outfitLight)
  fill(ctx, 17, 17, 1, 5, outfitLight)
  // Accent glow on chest (terminal symbol)
  fill(ctx, 13, 19, 6, 1, accent)
  fill(ctx, 12, 20, 8, 1, accent)
  fill(ctx, 13, 21, 6, 1, '#00000033')
}

function drawOutfitHoodedBlack(ctx, outfit, outfitLight, accent) {
  // Black hoodie — hood is drawn in hair, body continues
  fill(ctx,  6, 14, 20, 18, outfit)
  // Subtle seam
  fill(ctx, 14, 17, 4, 1, outfitLight)
  // Matrix code hint on sleeve
  fill(ctx,  8, 19, 2, 1, accent)
  fill(ctx,  8, 21, 2, 1, accent)
  fill(ctx, 22, 20, 2, 1, accent)
  fill(ctx, 22, 22, 2, 1, accent)
  // Glowing emblem
  fill(ctx, 13, 22, 6, 2, accent)
}

function drawOutfitCreative(ctx, outfit, outfitLight, accent) {
  // Colorful blouse with artistic details
  fill(ctx,  8, 17, 16, 15, outfit)
  // Draped collar
  fill(ctx, 11, 17, 10, 2, outfitLight)
  // Decorative pattern on chest
  fill(ctx, 12, 20, 2, 2, accent)
  fill(ctx, 14, 19, 2, 2, outfitLight)
  fill(ctx, 16, 21, 2, 2, accent)
  fill(ctx, 18, 19, 2, 2, outfitLight)
  // Side ruffle
  fill(ctx,  8, 20, 2, 8, outfitLight)
  fill(ctx, 22, 21, 2, 7, outfitLight)
  // Accent stripe at bottom
  fill(ctx,  8, 29, 16, 3, accent)
}

function drawOutfitBusinessCasual(ctx, outfit, outfitLight, accent) {
  // Button-up shirt, professional
  fill(ctx,  8, 17, 16, 15, outfit)
  // Collar
  fill(ctx, 12, 17, 8, 2, '#e8eef8')
  fill(ctx, 14, 17, 4, 1, '#f0f4ff')
  // Buttons
  fill(ctx, 15, 20, 2, 1, '#ffffff')
  fill(ctx, 15, 23, 2, 1, '#ffffff')
  fill(ctx, 15, 26, 2, 1, '#ffffff')
  // Accent on pocket
  fill(ctx,  9, 21, 4, 4, outfitLight)
  fill(ctx, 10, 21, 2, 1, accent)
}

function drawOutfitSuit(ctx, outfit, outfitLight, accent) {
  // Formal dark suit
  fill(ctx,  7, 16, 18, 16, outfit)
  // White shirt center
  fill(ctx, 14, 16, 4, 16, '#f0f0f8')
  // Left lapel
  fill(ctx, 12, 17, 3, 8, '#e8ecf4')
  // Right lapel
  fill(ctx, 17, 17, 3, 8, '#e8ecf4')
  // Pocket square
  fill(ctx,  9, 20, 3, 2, outfitLight)
  px(ctx, 10, 20, accent)
  // Gold cufflinks
  fill(ctx,  8, 26, 2, 1, accent)
  fill(ctx, 22, 26, 2, 1, accent)
}

function drawOutfitVest(ctx, outfit, outfitLight, accent) {
  // Cargo/work vest over shirt
  // Shirt base
  fill(ctx,  8, 17, 16, 15, '#c8d4c8')
  // Vest sides
  fill(ctx,  8, 17,  5, 15, outfit)
  fill(ctx, 19, 17,  5, 15, outfit)
  // Vest top strap
  fill(ctx,  8, 17, 16, 2, outfit)
  // Pockets
  fill(ctx,  9, 21, 4, 4, outfitLight)
  fill(ctx, 19, 21, 4, 4, outfitLight)
  // Pocket details
  fill(ctx, 10, 21, 2, 1, outfit)
  fill(ctx, 20, 21, 2, 1, outfit)
  // Accent badge
  fill(ctx, 20, 19, 3, 2, accent)
}

function drawOutfitLabCoat(ctx, outfit, outfitLight, accent) {
  // White lab coat
  fill(ctx,  7, 16, 18, 16, outfit)
  // Inner shirt (blue/colored)
  fill(ctx, 13, 16, 6, 8, accent)
  fill(ctx, 14, 16, 4, 2, '#5bc0de')
  // Coat lapels
  fill(ctx, 11, 17, 3, 10, '#d0d8e8')
  fill(ctx, 18, 17, 3, 10, '#d0d8e8')
  // Left breast pocket
  fill(ctx,  9, 19, 4, 4, outfitLight)
  // Pen in pocket
  fill(ctx, 11, 19, 1, 4, accent)
  // Right breast pocket
  fill(ctx, 19, 19, 4, 4, outfitLight)
  // Coat buttons
  fill(ctx, 15, 24, 2, 1, '#b0b8c8')
  fill(ctx, 15, 27, 2, 1, '#b0b8c8')
  fill(ctx, 15, 30, 2, 1, '#b0b8c8')
}

function drawOutfitFormalVest(ctx, outfit, outfitLight, accent) {
  // Formal vest with collared shirt — reviewer
  // Shirt
  fill(ctx,  8, 17, 16, 15, '#e0ece0')
  // Vest body
  fill(ctx,  9, 17, 14, 15, outfit)
  // V-neck opening
  fill(ctx, 13, 17, 6, 8, '#e0ece0')
  fill(ctx, 14, 17, 4, 2, '#f0f4f0')
  // Collar
  fill(ctx, 12, 17, 3, 3, '#f0f4f0')
  fill(ctx, 17, 17, 3, 3, '#f0f4f0')
  // Checkmark badge on chest
  fill(ctx, 10, 21, 4, 4, accent)
  px(ctx, 11, 23, '#ffffff')
  px(ctx, 12, 24, '#ffffff')
  px(ctx, 13, 23, '#ffffff')
  px(ctx, 14, 22, '#ffffff')
  // Vest buttons
  fill(ctx, 15, 24, 2, 1, outfitLight)
  fill(ctx, 15, 27, 2, 1, outfitLight)
}

// ── Main portrait renderer ─────────────────────────────────────────────────────

const expressionMap = {
  coder:      'focused',
  hacker:     'intense',
  designer:   'smile',
  analyst:    'thoughtful',
  architect:  'smirk',
  devops:     'focused',
  researcher: 'smile',
  reviewer:   'thoughtful',
}

function drawPortrait(ctx, profileId) {
  const style = PROFILE_STYLES[profileId] || PROFILE_STYLES.coder
  const expr = expressionMap[profileId] || 'focused'

  ctx.clearRect(0, 0, CANVAS_SIZE, CANVAS_SIZE)

  // 1. Hair (behind head layer)
  switch (style.hairStyle) {
    case 'short':   drawHairShort(ctx, style.hair, style.hairLight); break
    case 'hood':    drawHairHood(ctx, style.hair, style.outfit); break
    case 'long':    drawHairLong(ctx, style.hair, style.hairLight); break
    case 'medium':  drawHairMedium(ctx, style.hair, style.hairLight); break
    case 'messy':   drawHairMessy(ctx, style.hair, style.hairLight); break
    case 'bun':     drawHairBun(ctx, style.hair, style.hairLight); break
    case 'slicked': drawHairSlicked(ctx, style.hair, style.hairLight); break
    case 'neat':    drawHairNeat(ctx, style.hair, style.hairLight); break
  }

  // 2. Head
  drawHead(ctx, style.skin)

  // 3. Neck
  drawNeck(ctx, style.skin)

  // 4. Outfit (body, drawn before accessories on head)
  switch (profileId) {
    case 'coder':      drawOutfitHoodie(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'hacker':     drawOutfitHoodedBlack(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'designer':   drawOutfitCreative(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'analyst':    drawOutfitBusinessCasual(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'architect':  drawOutfitSuit(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'devops':     drawOutfitVest(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'researcher': drawOutfitLabCoat(ctx, style.outfit, style.outfitLight, style.accent); break
    case 'reviewer':   drawOutfitFormalVest(ctx, style.outfit, style.outfitLight, style.accent); break
    default:           drawOutfitHoodie(ctx, style.outfit, style.outfitLight, style.accent); break
  }

  // 5. Face features (drawn on top of head)
  drawEyebrows(ctx, style.hair, expr)
  drawEyes(ctx, style.eye)
  drawNose(ctx, style.skin)
  drawMouth(ctx, expr)

  // 6. Accessories (on top of everything)
  if (style.hasGlasses)   drawGlasses(ctx)
  if (style.hasSunglasses) drawSunglasses(ctx)
  if (style.hasHeadset)   drawHeadset(ctx, style.outfit)
  if (style.hasTie)       drawTie(ctx, style.accent)
}

// ── Public API ─────────────────────────────────────────────────────────────────

/**
 * Render a character portrait to a canvas element.
 * @param {string} profileId  - skill profile ID (coder, hacker, designer, ...)
 * @param {number} [scale=1]  - pixel scale factor (1 = 32px, 2 = 64px, 4 = 128px)
 * @returns {HTMLCanvasElement}
 */
export function renderPortrait(profileId, scale = 1) {
  const canvas = document.createElement('canvas')
  canvas.width  = CANVAS_SIZE * scale
  canvas.height = CANVAS_SIZE * scale

  const ctx = canvas.getContext('2d')
  ctx.imageSmoothingEnabled = false

  if (scale === 1) {
    drawPortrait(ctx, profileId)
  } else {
    // Draw at 1x then scale up via drawImage
    const tmp = document.createElement('canvas')
    tmp.width  = CANVAS_SIZE
    tmp.height = CANVAS_SIZE
    const tmpCtx = tmp.getContext('2d')
    tmpCtx.imageSmoothingEnabled = false
    drawPortrait(tmpCtx, profileId)

    ctx.drawImage(tmp, 0, 0, CANVAS_SIZE * scale, CANVAS_SIZE * scale)
  }

  return canvas
}

/**
 * Get a portrait as a PNG data URL (for use in <img> src).
 * @param {string} profileId
 * @param {number} [scale=2]
 * @returns {string}  data:image/png;base64,...
 */
export function getPortraitDataUrl(profileId, scale = 2) {
  return renderPortrait(profileId, scale).toDataURL('image/png')
}

/**
 * Resolve which portrait profile to use for a worker object.
 * Falls back to skill profile, then avatar name, then 'coder'.
 */
export function getWorkerPortraitProfile(worker) {
  const validProfiles = new Set(Object.keys(PROFILE_STYLES))
  if (worker.skillProfile && validProfiles.has(worker.skillProfile)) return worker.skillProfile
  if (worker.avatar && validProfiles.has(worker.avatar)) return worker.avatar
  return 'coder'
}

export { PROFILE_STYLES, CANVAS_SIZE }
