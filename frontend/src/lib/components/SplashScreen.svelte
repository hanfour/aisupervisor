<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { loadAllSprites, spritesReady, prerenderCharacter } from '../office/sprites.js'

  export let onComplete = () => {}
  export let onSettings = () => {}

  let canvas
  let phase = 'fadein'    // fadein → title → chars → ready → fadeout
  let opacity = 0
  let titleY = -40
  let showSubtitle = false
  let showButtons = false
  let charSlots = []
  let animFrame = 0
  let elapsed = 0
  let fadeOutOpacity = 1
  let starParticles = []
  let raf
  let fadeCallback = null

  const CANVAS_W = 800
  const CANVAS_H = 500
  const TITLE_TEXT = 'AI SUPERVISOR'
  const SUBTITLE_TEXT = 'PIXEL OFFICE'

  // Character types to show walking in
  const SPLASH_CHARS = ['coder', 'hacker', 'designer', 'analyst', 'architect', 'devops']

  onMount(async () => {
    await loadAllSprites()

    // Prerender splash characters
    charSlots = SPLASH_CHARS.map((type, i) => ({
      type,
      cache: prerenderCharacter(type),
      x: -60 - i * 80,  // start off-screen left
      targetX: 180 + i * 80,
      y: 320,
      arrived: false,
    }))

    // Init star particles
    for (let i = 0; i < 30; i++) {
      starParticles.push({
        x: Math.random() * CANVAS_W,
        y: Math.random() * CANVAS_H * 0.6,
        size: 1 + Math.random() * 2,
        speed: 0.2 + Math.random() * 0.5,
        twinkle: Math.random() * Math.PI * 2,
      })
    }

    loop()
  })

  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf)
  })

  function startFadeOut(callback) {
    fadeCallback = callback
    phase = 'fadeout'
    fadeOutOpacity = 1
  }

  function handleStart() {
    showButtons = false
    startFadeOut(onComplete)
  }

  function handleSettings() {
    showButtons = false
    startFadeOut(onSettings)
  }

  let lastTime = 0
  function loop() {
    const now = performance.now()
    const delta = lastTime ? now - lastTime : 16
    lastTime = now
    elapsed += delta

    update(delta)
    draw()

    raf = requestAnimationFrame(loop)
  }

  function update(delta) {
    animFrame = Math.floor(elapsed / 200) % 3

    // Phase transitions based on elapsed time
    if (elapsed < 500) {
      // Fade in
      phase = 'fadein'
      opacity = Math.min(1, elapsed / 500)
    } else if (elapsed < 1200) {
      // Title drops in
      phase = 'title'
      opacity = 1
      const t = (elapsed - 500) / 700
      titleY = -40 + t * 190  // drops to y=150
      if (t > 0.7) showSubtitle = true
    } else if (elapsed < 2500) {
      // Characters walk in
      phase = 'chars'
      const t = (elapsed - 1200) / 1300
      for (const slot of charSlots) {
        const progress = Math.min(1, t * 1.5)
        slot.x = slot.x + (slot.targetX - slot.x) * 0.08
        if (Math.abs(slot.x - slot.targetX) < 2) {
          slot.x = slot.targetX
          slot.arrived = true
        }
      }
    } else if (phase !== 'fadeout') {
      // Ready — show buttons, wait for user action
      phase = 'ready'
      showButtons = true
      // All chars should be at target
      for (const slot of charSlots) {
        slot.x = slot.targetX
        slot.arrived = true
      }
    } else {
      // Fade out and complete
      fadeOutOpacity = Math.max(0, fadeOutOpacity - delta / 600)
      if (fadeOutOpacity <= 0) {
        cancelAnimationFrame(raf)
        if (fadeCallback) fadeCallback()
      }
    }

    // Update star twinkle
    for (const s of starParticles) {
      s.twinkle += delta * 0.003
    }
  }

  function draw() {
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingEnabled = false

    // Global opacity for fade in/out
    const globalAlpha = phase === 'fadeout' ? fadeOutOpacity : opacity
    ctx.globalAlpha = globalAlpha

    // Background: dark gradient
    const grad = ctx.createLinearGradient(0, 0, 0, CANVAS_H)
    grad.addColorStop(0, '#0a0a1a')
    grad.addColorStop(0.6, '#1a1a2e')
    grad.addColorStop(1, '#16213e')
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, CANVAS_W, CANVAS_H)

    // Stars
    for (const s of starParticles) {
      const alpha = 0.3 + 0.7 * Math.abs(Math.sin(s.twinkle))
      ctx.globalAlpha = globalAlpha * alpha
      ctx.fillStyle = '#ffe8a0'
      ctx.beginPath()
      ctx.arc(s.x, s.y, s.size, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.globalAlpha = globalAlpha

    // Floor line (warm wood)
    ctx.fillStyle = '#2a1f14'
    ctx.fillRect(0, 370, CANVAS_W, 130)
    ctx.fillStyle = '#3d2e1e'
    ctx.fillRect(0, 370, CANVAS_W, 3)

    // Grid pattern on floor
    ctx.strokeStyle = 'rgba(80, 60, 30, 0.3)'
    ctx.lineWidth = 1
    for (let x = 0; x < CANVAS_W; x += 48) {
      ctx.beginPath()
      ctx.moveTo(x, 370)
      ctx.lineTo(x, CANVAS_H)
      ctx.stroke()
    }

    // Title
    if (phase !== 'fadein') {
      // Title shadow
      ctx.font = 'bold 36px "Press Start 2P", monospace'
      ctx.textAlign = 'center'
      ctx.fillStyle = 'rgba(0, 255, 65, 0.15)'
      ctx.fillText(TITLE_TEXT, CANVAS_W / 2 + 3, titleY + 3)

      // Title glow
      ctx.shadowColor = '#00ff41'
      ctx.shadowBlur = 20
      ctx.fillStyle = '#00ff41'
      ctx.fillText(TITLE_TEXT, CANVAS_W / 2, titleY)
      ctx.shadowBlur = 0

      // Subtitle
      if (showSubtitle) {
        ctx.font = '14px "Press Start 2P", monospace'
        ctx.fillStyle = '#5bbad5'
        ctx.shadowColor = '#5bbad5'
        ctx.shadowBlur = 10
        ctx.fillText(SUBTITLE_TEXT, CANVAS_W / 2, titleY + 45)
        ctx.shadowBlur = 0
      }

      // Version badge
      ctx.font = '8px "Press Start 2P", monospace'
      ctx.fillStyle = 'rgba(200, 200, 200, 0.3)'
      ctx.fillText('v2.0', CANVAS_W / 2, titleY + 70)
    }

    // Characters
    for (const slot of charSlots) {
      if (!slot.cache) continue
      const frames = slot.arrived
        ? slot.cache.idle
        : slot.cache.walkRight
      if (!frames) continue
      const frame = frames[animFrame % frames.length]
      if (!frame) continue

      // Shadow
      ctx.globalAlpha = globalAlpha * 0.3
      ctx.fillStyle = '#000'
      ctx.beginPath()
      ctx.ellipse(slot.x + 24, slot.y + 50, 16, 4, 0, 0, Math.PI * 2)
      ctx.fill()
      ctx.globalAlpha = globalAlpha

      // Sprite (scaled 3x)
      ctx.drawImage(frame, slot.x, slot.y, 48, 48)
    }

    // "READY" text when buttons are shown
    if (showButtons) {
      ctx.globalAlpha = globalAlpha * 0.4
      ctx.font = '8px "Press Start 2P", monospace'
      ctx.textAlign = 'center'
      ctx.fillStyle = '#ffdd57'
      ctx.fillText('READY', CANVAS_W / 2, 420)
      ctx.globalAlpha = globalAlpha
    }

    // Decorative scan line effect
    ctx.fillStyle = 'rgba(0, 0, 0, 0.03)'
    for (let y = 0; y < CANVAS_H; y += 3) {
      ctx.fillRect(0, y, CANVAS_W, 1)
    }
  }
</script>

<div class="splash-container">
  <canvas
    bind:this={canvas}
    width={CANVAS_W}
    height={CANVAS_H}
    class="splash-canvas"
  ></canvas>
  {#if showButtons && phase === 'ready'}
    <div class="splash-buttons">
      <button class="splash-btn start-btn" on:click={handleStart}>START</button>
      <button class="splash-btn settings-btn" on:click={handleSettings}>SETTINGS</button>
    </div>
  {/if}
</div>

<style>
  .splash-container {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: #0a0a1a;
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 9999;
  }

  .splash-canvas {
    image-rendering: pixelated;
    max-width: 100%;
    max-height: 100%;
  }

  .splash-buttons {
    position: absolute;
    bottom: 18%;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    gap: 24px;
    z-index: 10;
  }

  .splash-btn {
    font-family: 'Press Start 2P', monospace;
    font-size: 12px;
    padding: 10px 28px;
    border: 3px solid;
    cursor: pointer;
    background: transparent;
    transition: transform 0.1s, box-shadow 0.1s;
    box-shadow: 3px 3px 0 rgba(0, 0, 0, 0.5);
  }

  .splash-btn:hover {
    transform: translate(1px, 1px);
    box-shadow: 2px 2px 0 rgba(0, 0, 0, 0.5);
  }

  .splash-btn:active {
    transform: translate(3px, 3px);
    box-shadow: none;
  }

  .start-btn {
    color: #00ff41;
    border-color: #00ff41;
    text-shadow: 0 0 8px rgba(0, 255, 65, 0.5);
  }

  .start-btn:hover {
    background: rgba(0, 255, 65, 0.1);
  }

  .settings-btn {
    color: #5bbad5;
    border-color: #5bbad5;
    text-shadow: 0 0 8px rgba(91, 186, 213, 0.5);
  }

  .settings-btn:hover {
    background: rgba(91, 186, 213, 0.1);
  }
</style>
