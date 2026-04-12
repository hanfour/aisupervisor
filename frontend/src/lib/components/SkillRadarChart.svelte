<script>
  import { onMount } from 'svelte'

  export let branches = {}
  export let width = 280
  export let height = 280

  let canvas

  const BRANCH_ORDER = ['frontend', 'backend', 'security', 'devops', 'architecture', 'research']
  const BRANCH_LABELS = {
    frontend: '前端',
    backend: '後端',
    security: '安全',
    devops: 'DevOps',
    architecture: '架構',
    research: '研究',
  }
  const MAX_LEVEL = 5
  const LEVELS = [1, 2, 3, 4, 5]

  function getComputedColor(varName, fallback) {
    if (typeof getComputedStyle === 'undefined') return fallback
    const root = document.documentElement
    const val = getComputedStyle(root).getPropertyValue(varName).trim()
    return val || fallback
  }

  function draw() {
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    const dpr = window.devicePixelRatio || 1
    canvas.width = width * dpr
    canvas.height = height * dpr
    canvas.style.width = width + 'px'
    canvas.style.height = height + 'px'
    ctx.scale(dpr, dpr)

    const textColor = getComputedColor('--text', '#e0e0e0')
    const accentColor = getComputedColor('--accent-blue', '#5bbad5')
    const bgSecondary = getComputedColor('--bg-secondary', '#1a1a2e')

    const cx = width / 2
    const cy = height / 2
    const radius = Math.min(width, height) / 2 - 40
    const sides = BRANCH_ORDER.length
    const angleStep = (Math.PI * 2) / sides
    const startAngle = -Math.PI / 2 // start from top

    ctx.clearRect(0, 0, width, height)

    // Draw concentric hexagonal grid levels
    for (const lvl of LEVELS) {
      const r = (lvl / MAX_LEVEL) * radius
      ctx.beginPath()
      for (let i = 0; i < sides; i++) {
        const angle = startAngle + i * angleStep
        const x = cx + r * Math.cos(angle)
        const y = cy + r * Math.sin(angle)
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.closePath()
      ctx.strokeStyle = lvl === MAX_LEVEL ? 'rgba(255,255,255,0.2)' : 'rgba(255,255,255,0.08)'
      ctx.lineWidth = lvl === MAX_LEVEL ? 1.5 : 0.5
      ctx.stroke()
    }

    // Draw axis lines
    for (let i = 0; i < sides; i++) {
      const angle = startAngle + i * angleStep
      ctx.beginPath()
      ctx.moveTo(cx, cy)
      ctx.lineTo(cx + radius * Math.cos(angle), cy + radius * Math.sin(angle))
      ctx.strokeStyle = 'rgba(255,255,255,0.1)'
      ctx.lineWidth = 0.5
      ctx.stroke()
    }

    // Draw filled skill area
    const values = BRANCH_ORDER.map(b => {
      const node = branches[b]
      return node ? (node.level || 1) : 1
    })

    ctx.beginPath()
    for (let i = 0; i < sides; i++) {
      const angle = startAngle + i * angleStep
      const r = (values[i] / MAX_LEVEL) * radius
      const x = cx + r * Math.cos(angle)
      const y = cy + r * Math.sin(angle)
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    }
    ctx.closePath()
    ctx.fillStyle = accentColor.replace(')', ',0.25)').replace('rgb(', 'rgba(')
    if (!ctx.fillStyle.startsWith('rgba')) {
      ctx.fillStyle = 'rgba(91,186,213,0.25)'
    }
    ctx.fill()
    ctx.strokeStyle = accentColor
    ctx.lineWidth = 2
    ctx.stroke()

    // Draw data points
    for (let i = 0; i < sides; i++) {
      const angle = startAngle + i * angleStep
      const r = (values[i] / MAX_LEVEL) * radius
      const x = cx + r * Math.cos(angle)
      const y = cy + r * Math.sin(angle)
      ctx.beginPath()
      ctx.arc(x, y, 3, 0, Math.PI * 2)
      ctx.fillStyle = accentColor
      ctx.fill()
    }

    // Draw labels and level numbers
    ctx.font = '9px "Press Start 2P", monospace'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'

    for (let i = 0; i < sides; i++) {
      const angle = startAngle + i * angleStep
      const labelR = radius + 22
      const x = cx + labelR * Math.cos(angle)
      const y = cy + labelR * Math.sin(angle)

      // Branch label
      ctx.fillStyle = textColor
      ctx.fillText(BRANCH_LABELS[BRANCH_ORDER[i]], x, y)

      // Level number (slightly inward)
      const lvlR = (values[i] / MAX_LEVEL) * radius + 10
      const lvlX = cx + lvlR * Math.cos(angle)
      const lvlY = cy + lvlR * Math.sin(angle)
      ctx.font = '7px "Press Start 2P", monospace'
      ctx.fillStyle = accentColor
      ctx.fillText('Lv' + values[i], lvlX, lvlY - 8)
      ctx.font = '9px "Press Start 2P", monospace'
    }
  }

  onMount(() => {
    draw()
  })

  $: if (canvas && branches) draw()
</script>

<canvas bind:this={canvas} style="width: {width}px; height: {height}px;"></canvas>

<style>
  canvas {
    display: block;
    image-rendering: pixelated;
  }
</style>
