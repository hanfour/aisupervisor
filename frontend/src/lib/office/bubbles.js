// Speech bubble system for Pixel Office
// Handles speech, thought, discussion, and meeting bubbles that follow characters

import { TILE_SIZE, SCALE } from './layout.js'

const TILE_PX = TILE_SIZE * SCALE  // 48
const FONT = '7px "Press Start 2P", monospace'
const BUBBLE_MAX_WIDTH = 110
const LINE_HEIGHT = 10
const PADDING = 5

function wrapText(ctx, text, maxWidth) {
  const words = text.split(' ')
  const lines = []
  let current = ''
  for (const word of words) {
    const test = current ? current + ' ' + word : word
    if (ctx.measureText(test).width > maxWidth && current) {
      lines.push(current)
      current = word
    } else {
      current = test
    }
  }
  if (current) lines.push(current)
  return lines.length ? lines : ['']
}

class Bubble {
  constructor(id, type, workerIds, text, duration) {
    this.id = id
    this.type = type
    this.workerIds = workerIds
    this.text = text
    this.duration = duration
    this.elapsed = 0
    this.fadeIn = 200
    this.fadeOut = 300
  }

  get opacity() {
    if (this.elapsed < this.fadeIn) return this.elapsed / this.fadeIn
    const remaining = this.duration - this.elapsed
    if (remaining < this.fadeOut) return Math.max(0, remaining / this.fadeOut)
    return 1
  }
}

export class BubbleManager {
  constructor() {
    this.bubbles = new Map()  // bubbleId → Bubble
    this.nextId = 1
  }

  // Show a speech bubble with text above a worker
  showSpeech(workerId, text, duration = 3000) {
    const id = this.nextId++
    this.bubbles.set(id, new Bubble(id, 'speech', [workerId], text, duration))
    return id
  }

  // Show a thought bubble (cloud-shaped)
  showThought(workerId, text, duration = 4000) {
    const id = this.nextId++
    this.bubbles.set(id, new Bubble(id, 'thought', [workerId], text, duration))
    return id
  }

  // Show discussion indicator between two workers
  showDiscussion(worker1Id, worker2Id, topic, duration = 5000) {
    const id = this.nextId++
    this.bubbles.set(id, new Bubble(id, 'discussion', [worker1Id, worker2Id], topic, duration))
    return id
  }

  // Show meeting indicator for multiple workers
  showMeeting(workerIds, topic, duration = 8000) {
    const id = this.nextId++
    this.bubbles.set(id, new Bubble(id, 'meeting', [...workerIds], topic, duration))
    return id
  }

  // Remove a specific bubble
  clear(bubbleId) {
    this.bubbles.delete(bubbleId)
  }

  // Remove all bubbles for a worker
  clearWorker(workerId) {
    for (const [id, bubble] of this.bubbles) {
      if (bubble.workerIds.includes(workerId)) this.bubbles.delete(id)
    }
  }

  // Remove all bubbles
  clearAll() {
    this.bubbles.clear()
  }

  // Update bubble animations (call each frame)
  update(deltaMs) {
    for (const [id, bubble] of this.bubbles) {
      bubble.elapsed += deltaMs
      if (bubble.elapsed >= bubble.duration) this.bubbles.delete(id)
    }
  }

  // Draw all active bubbles
  // positionMap: workerId → { pixelX, pixelY }
  draw(ctx, positionMap) {
    if (!this.bubbles.size) return
    ctx.save()
    ctx.imageSmoothingEnabled = false

    for (const bubble of this.bubbles.values()) {
      const alpha = bubble.opacity
      if (alpha <= 0) continue
      ctx.globalAlpha = alpha
      ctx.font = FONT

      switch (bubble.type) {
        case 'speech':     this._drawSpeech(ctx, bubble, positionMap);     break
        case 'thought':    this._drawThought(ctx, bubble, positionMap);    break
        case 'discussion': this._drawDiscussion(ctx, bubble, positionMap); break
        case 'meeting':    this._drawMeeting(ctx, bubble, positionMap);    break
      }
    }

    ctx.globalAlpha = 1
    ctx.restore()
  }

  // ── Private helpers ─────────────────────────────────────────────────────────

  _pos(workerId, positionMap) {
    return positionMap[workerId] ?? null
  }

  _drawSpeech(ctx, bubble, positionMap) {
    const pos = this._pos(bubble.workerIds[0], positionMap)
    if (!pos) return

    const lines = wrapText(ctx, bubble.text, BUBBLE_MAX_WIDTH)
    const textW = Math.max(...lines.map(l => ctx.measureText(l).width))
    const bw = Math.min(BUBBLE_MAX_WIDTH, textW) + PADDING * 2
    const bh = lines.length * LINE_HEIGHT + PADDING * 2

    const cx = pos.pixelX + TILE_PX / 2
    const by = pos.pixelY - bh - 10
    const bx = cx - bw / 2

    // Box fill + border
    ctx.fillStyle = '#fffef0'
    ctx.fillRect(bx, by, bw, bh)
    ctx.strokeStyle = '#111'
    ctx.lineWidth = 2
    ctx.strokeRect(bx, by, bw, bh)

    // Triangle pointer (bottom center)
    ctx.fillStyle = '#fffef0'
    ctx.beginPath()
    ctx.moveTo(cx - 4, by + bh)
    ctx.lineTo(cx + 4, by + bh)
    ctx.lineTo(cx, by + bh + 6)
    ctx.closePath()
    ctx.fill()
    ctx.strokeStyle = '#111'
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(cx - 4, by + bh - 1)
    ctx.lineTo(cx, by + bh + 6)
    ctx.lineTo(cx + 4, by + bh - 1)
    ctx.stroke()

    // Text
    ctx.fillStyle = '#111'
    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], bx + PADDING, by + PADDING + (i + 1) * LINE_HEIGHT - 2)
    }
  }

  _drawThought(ctx, bubble, positionMap) {
    const pos = this._pos(bubble.workerIds[0], positionMap)
    if (!pos) return

    const lines = wrapText(ctx, bubble.text, BUBBLE_MAX_WIDTH)
    const textW = Math.max(...lines.map(l => ctx.measureText(l).width))
    const contentW = Math.min(BUBBLE_MAX_WIDTH, textW) + PADDING * 2
    const contentH = lines.length * LINE_HEIGHT + PADDING * 2

    const cloudCx = pos.pixelX + TILE_PX / 2
    const cloudCy = pos.pixelY - contentH / 2 - 20

    // Cloud blobs (fill first, then stroke to avoid overlap artifacts)
    const blobs = [
      { dx:  0,              dy: 0,                r: contentW * 0.36 },
      { dx:  contentW * 0.26, dy: -contentH * 0.08, r: contentW * 0.28 },
      { dx: -contentW * 0.26, dy: -contentH * 0.08, r: contentW * 0.28 },
      { dx:  contentW * 0.20, dy:  contentH * 0.14,  r: contentW * 0.22 },
      { dx: -contentW * 0.20, dy:  contentH * 0.14,  r: contentW * 0.22 },
    ]

    ctx.fillStyle = '#fff8e0'
    for (const b of blobs) {
      ctx.beginPath()
      ctx.arc(cloudCx + b.dx, cloudCy + b.dy, b.r, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.strokeStyle = '#c9a868'
    ctx.lineWidth = 1.5
    for (const b of blobs) {
      ctx.beginPath()
      ctx.arc(cloudCx + b.dx, cloudCy + b.dy, b.r, 0, Math.PI * 2)
      ctx.stroke()
    }

    // Chain of circles from character head up to cloud
    const startX = pos.pixelX + TILE_PX / 2
    const startY = pos.pixelY - 3
    const endY   = cloudCy + contentH / 2 + 8
    const chain = [
      { x: startX, y: startY, r: 2 },
      { x: startX + (cloudCx - startX) * 0.45, y: startY + (endY - startY) * 0.45, r: 3 },
      { x: startX + (cloudCx - startX) * 0.75, y: startY + (endY - startY) * 0.75, r: 4 },
    ]

    ctx.fillStyle = '#fff8e0'
    ctx.strokeStyle = '#c9a868'
    ctx.lineWidth = 1
    for (const c of chain) {
      ctx.beginPath()
      ctx.arc(c.x, c.y, c.r, 0, Math.PI * 2)
      ctx.fill()
      ctx.stroke()
    }

    // Text inside cloud
    ctx.fillStyle = '#555'
    const textStartX = cloudCx - contentW / 2 + PADDING
    const textStartY = cloudCy - contentH / 2 + PADDING
    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], textStartX, textStartY + (i + 1) * LINE_HEIGHT - 2)
    }
  }

  _drawDiscussion(ctx, bubble, positionMap) {
    const pos1 = this._pos(bubble.workerIds[0], positionMap)
    const pos2 = this._pos(bubble.workerIds[1], positionMap)
    if (!pos1 || !pos2) return

    const lines = wrapText(ctx, bubble.text, 80)
    const textW = Math.max(...lines.map(l => ctx.measureText(l).width))
    const bw = Math.min(80, textW) + PADDING * 2
    const bh = lines.length * LINE_HEIGHT + PADDING * 2

    // Alternate which bubble is bold every 800ms
    const altPhase = Math.floor(bubble.elapsed / 800) % 2
    this._drawDiscussionBubble(ctx, pos1, lines, bw, bh, altPhase === 0)
    this._drawDiscussionBubble(ctx, pos2, lines, bw, bh, altPhase === 1)

    // Dotted connecting line between bubble centers
    const b1cx = pos1.pixelX + TILE_PX / 2
    const b1cy = pos1.pixelY - bh - 10 + bh / 2
    const b2cx = pos2.pixelX + TILE_PX / 2
    const b2cy = pos2.pixelY - bh - 10 + bh / 2

    ctx.strokeStyle = '#88aa88'
    ctx.lineWidth = 1
    ctx.setLineDash([3, 4])
    ctx.beginPath()
    ctx.moveTo(b1cx, b1cy)
    ctx.lineTo(b2cx, b2cy)
    ctx.stroke()
    ctx.setLineDash([])
  }

  _drawDiscussionBubble(ctx, pos, lines, bw, bh, bold) {
    const cx = pos.pixelX + TILE_PX / 2
    const by = pos.pixelY - bh - 10
    const bx = cx - bw / 2

    ctx.fillStyle = '#f0ffe8'
    ctx.fillRect(bx, by, bw, bh)

    ctx.strokeStyle = '#446622'
    ctx.lineWidth = bold ? 2.5 : 1.5
    ctx.strokeRect(bx, by, bw, bh)

    // Triangle pointer
    ctx.fillStyle = '#f0ffe8'
    ctx.beginPath()
    ctx.moveTo(cx - 4, by + bh)
    ctx.lineTo(cx + 4, by + bh)
    ctx.lineTo(cx, by + bh + 6)
    ctx.closePath()
    ctx.fill()

    ctx.strokeStyle = '#446622'
    ctx.lineWidth = bold ? 2.5 : 1.5
    ctx.beginPath()
    ctx.moveTo(cx - 4, by + bh - 1)
    ctx.lineTo(cx, by + bh + 6)
    ctx.lineTo(cx + 4, by + bh - 1)
    ctx.stroke()

    ctx.fillStyle = '#333'
    for (let i = 0; i < lines.length; i++) {
      ctx.fillText(lines[i], bx + PADDING, by + PADDING + (i + 1) * LINE_HEIGHT - 2)
    }
  }

  _drawMeeting(ctx, bubble, positionMap) {
    const positions = bubble.workerIds
      .map(id => this._pos(id, positionMap))
      .filter(Boolean)
    if (!positions.length) return

    const avgX = positions.reduce((s, p) => s + p.pixelX + TILE_PX / 2, 0) / positions.length
    const minY = Math.min(...positions.map(p => p.pixelY))

    const topicLines = wrapText(ctx, bubble.text, BUBBLE_MAX_WIDTH)
    const countLine  = `[${bubble.workerIds.length} workers]`
    const allLines   = [...topicLines, countLine]

    const textW = Math.max(...allLines.map(l => ctx.measureText(l).width))
    const bw = Math.min(BUBBLE_MAX_WIDTH, textW) + PADDING * 2 + 8
    const bh = allLines.length * LINE_HEIGHT + PADDING * 2

    const bx = avgX - bw / 2
    const by = minY - bh - 16

    // Background + gold border
    ctx.fillStyle = '#fff8e0'
    ctx.fillRect(bx, by, bw, bh)

    ctx.strokeStyle = '#c8a000'
    ctx.lineWidth = 2
    ctx.strokeRect(bx, by, bw, bh)

    // Inner accent border
    ctx.strokeStyle = '#e8c840'
    ctx.lineWidth = 1
    ctx.strokeRect(bx + 3, by + 3, bw - 6, bh - 6)

    // Triangle pointer (bottom center)
    ctx.fillStyle = '#fff8e0'
    ctx.beginPath()
    ctx.moveTo(avgX - 5, by + bh)
    ctx.lineTo(avgX + 5, by + bh)
    ctx.lineTo(avgX, by + bh + 8)
    ctx.closePath()
    ctx.fill()

    ctx.strokeStyle = '#c8a000'
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(avgX - 5, by + bh - 1)
    ctx.lineTo(avgX, by + bh + 8)
    ctx.lineTo(avgX + 5, by + bh - 1)
    ctx.stroke()

    // Diamond corner decorations
    ctx.fillStyle = '#c8a000'
    this._drawDiamond(ctx, bx + 7, by + 7, 3)
    this._drawDiamond(ctx, bx + bw - 7, by + 7, 3)

    // Topic text
    ctx.fillStyle = '#333'
    for (let i = 0; i < topicLines.length; i++) {
      ctx.fillText(topicLines[i], bx + PADDING + 4, by + PADDING + (i + 1) * LINE_HEIGHT - 2)
    }

    // Worker count (dimmer)
    ctx.fillStyle = '#999'
    ctx.fillText(countLine, bx + PADDING + 4, by + PADDING + topicLines.length * LINE_HEIGHT + LINE_HEIGHT - 2)
  }

  _drawDiamond(ctx, cx, cy, r) {
    ctx.beginPath()
    ctx.moveTo(cx, cy - r)
    ctx.lineTo(cx + r, cy)
    ctx.lineTo(cx, cy + r)
    ctx.lineTo(cx - r, cy)
    ctx.closePath()
    ctx.fill()
  }
}
