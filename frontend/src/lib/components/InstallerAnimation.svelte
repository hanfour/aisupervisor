<script>
  import { onMount, onDestroy } from 'svelte'
  import { prerenderCharacter, loadAllSprites, spritesReady } from '../office/sprites.js'
  import { AnimationState } from '../office/animation.js'

  export let phase = 'idle' // 'idle' | 'installing' | 'done' | 'error'

  let canvas
  let animFrame
  let lastTime = 0
  let ready = false

  // Three workers with different character types for visual variety
  const workers = [
    { type: 'devops',   x: 0,   anim: new AnimationState() },
    { type: 'coder',    x: 56,  anim: new AnimationState() },
    { type: 'hacker',   x: 112, anim: new AnimationState() },
  ]

  let caches = {}

  // Floating tool particles
  let particles = []
  const TOOL_EMOJIS = ['🔧', '📦', '⚙️', '🔨', '💾', '🛠️']

  function spawnParticle() {
    if (particles.length > 8) return
    particles.push({
      x: 20 + Math.random() * 130,
      y: 60 + Math.random() * 10,
      vy: -0.3 - Math.random() * 0.5,
      life: 1.0,
      emoji: TOOL_EMOJIS[Math.floor(Math.random() * TOOL_EMOJIS.length)],
      size: 8 + Math.random() * 6,
    })
  }

  $: {
    const animState = phase === 'installing' ? 'working'
                    : phase === 'done' ? 'finished'
                    : phase === 'error' ? 'error'
                    : 'idle'
    workers.forEach(w => w.anim.setState(animState))
  }

  async function init() {
    if (!spritesReady()) {
      await loadAllSprites()
    }
    for (const w of workers) {
      caches[w.type] = prerenderCharacter(w.type)
    }
    ready = true
    lastTime = performance.now()
    loop()
  }

  function loop() {
    const now = performance.now()
    const delta = now - lastTime
    lastTime = now

    // Update animations
    const mood = phase === 'installing' ? 'excited' : phase === 'done' ? 'happy' : null
    workers.forEach(w => w.anim.update(delta, mood))

    // Spawn particles while installing
    if (phase === 'installing' && Math.random() < 0.03) {
      spawnParticle()
    }

    // Update particles
    particles = particles.filter(p => {
      p.y += p.vy
      p.life -= 0.008
      return p.life > 0
    })

    draw()
    animFrame = requestAnimationFrame(loop)
  }

  function draw() {
    if (!canvas || !ready) return
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // Draw floor line
    ctx.fillStyle = '#333'
    ctx.fillRect(0, 52, canvas.width, 2)

    // Draw workers
    for (const w of workers) {
      const cache = caches[w.type]
      if (!cache) continue
      const state = w.anim.state
      const frames = cache[state] || cache.idle
      if (!frames || frames.length === 0) continue
      const frameIdx = w.anim.getFrame() % frames.length
      const frame = frames[frameIdx]
      if (frame) {
        // Bounce effect when working
        let bounce = 0
        if (phase === 'installing' && state === 'working') {
          bounce = Math.sin(performance.now() * 0.006 + w.x * 0.1) * 2
        }
        ctx.drawImage(frame, w.x, 6 + bounce, 48, 48)
      }
    }

    // Draw particles (tool emojis floating up)
    for (const p of particles) {
      ctx.globalAlpha = Math.max(0, p.life)
      ctx.font = `${p.size}px sans-serif`
      ctx.fillText(p.emoji, p.x, p.y)
    }
    ctx.globalAlpha = 1.0

    // Done sparkle effect
    if (phase === 'done') {
      const t = performance.now() * 0.003
      for (let i = 0; i < 5; i++) {
        const sx = 20 + ((t * 30 + i * 35) % 140)
        const sy = 10 + Math.sin(t + i * 1.3) * 15
        const alpha = 0.4 + Math.sin(t * 2 + i) * 0.3
        ctx.globalAlpha = Math.max(0, alpha)
        ctx.font = '10px sans-serif'
        ctx.fillText('✨', sx, sy)
      }
      ctx.globalAlpha = 1.0
    }
  }

  onMount(init)
  onDestroy(() => {
    if (animFrame) cancelAnimationFrame(animFrame)
  })
</script>

<div class="installer-anim" class:installing={phase === 'installing'} class:done={phase === 'done'}>
  <canvas
    bind:this={canvas}
    width="168"
    height="56"
    class="anim-canvas"
  ></canvas>
</div>

<style>
  .installer-anim {
    display: flex;
    justify-content: center;
    padding: 0.5rem 0;
  }

  .anim-canvas {
    image-rendering: pixelated;
    image-rendering: crisp-edges;
    width: 336px;
    height: 112px;
  }

  .installer-anim.installing .anim-canvas {
    filter: drop-shadow(0 0 4px rgba(146, 204, 65, 0.3));
  }

  .installer-anim.done .anim-canvas {
    filter: drop-shadow(0 0 6px rgba(247, 213, 29, 0.4));
  }
</style>
