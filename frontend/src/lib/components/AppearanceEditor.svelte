<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { loadAllSprites, spritesReady } from '../office/sprites.js'
  import { t } from '../stores/i18n.js'

  export let workerId
  export let currentAppearance = null  // {bodyRow, outfit, hair} or null
  export let onSave = () => {}
  export let onCancel = () => {}

  const BODY_ROWS = 6
  const OUTFITS = ['outfit1', 'outfit2', 'outfit3', 'outfit4', 'outfit5', 'outfit6']
  const HAIRS = ['hair1', 'hair2', 'hair3', 'hair4', 'hair5', 'hair6', 'hair7']

  const FRAME_SIZE = 32
  const DIR_DOWN = 0
  const FRAMES_PER_DIR = 6

  let bodyRow = currentAppearance?.bodyRow ?? 0
  let outfit = currentAppearance?.outfit ?? 'outfit1'
  let hair = currentAppearance?.hair ?? 'hair1'

  let previewCanvas
  let imageCache = {}
  let ready = false

  // Skin tone representative colors (sampled from body.png rows)
  const SKIN_COLORS = ['#f5d5b8', '#e8c49a', '#d4a878', '#c08860', '#8b6040', '#5a3828']

  onMount(async () => {
    await loadAllSprites()
    // Load images directly for preview
    const urls = {}
    // Dynamic imports for sprite assets
    const modules = import.meta.glob('../office/assets/*.png', { eager: true })
    for (const [path, mod] of Object.entries(modules)) {
      const name = path.split('/').pop().replace('.png', '')
      imageCache[name] = await loadImg(mod.default)
    }
    ready = true
    drawPreview()
  })

  function loadImg(url) {
    return new Promise((resolve, reject) => {
      const img = new Image()
      img.onload = () => {
        if (img.decode) {
          img.decode().then(() => resolve(img)).catch(() => resolve(img))
        } else {
          resolve(img)
        }
      }
      img.onerror = reject
      img.src = url
    })
  }

  $: if (ready) drawPreview()
  $: bodyRow, outfit, hair, ready && drawPreview()

  function drawPreview() {
    if (!previewCanvas || !ready) return
    const ctx = previewCanvas.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.clearRect(0, 0, previewCanvas.width, previewCanvas.height)

    const bodyImg = imageCache['body']
    const outfitImg = imageCache[outfit]
    const hairImg = imageCache[hair]

    if (!bodyImg) return

    const scale = 3
    const size = FRAME_SIZE * scale
    previewCanvas.width = size
    previewCanvas.height = size

    const col = DIR_DOWN * FRAMES_PER_DIR  // first frame, down direction

    // Body
    ctx.drawImage(bodyImg,
      col * FRAME_SIZE, bodyRow * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE,
      0, 0, size, size
    )

    // Outfit
    if (outfitImg) {
      ctx.drawImage(outfitImg,
        col * FRAME_SIZE, 0, FRAME_SIZE, FRAME_SIZE,
        0, 0, size, size
      )
    }

    // Hair
    if (hairImg) {
      ctx.drawImage(hairImg, 0, 0, FRAME_SIZE, FRAME_SIZE, 0, 0, size, size)
    }
  }

  async function handleSave() {
    try {
      await window.go.gui.CompanyApp.UpdateWorkerAppearance(workerId, bodyRow, outfit, hair)
      onSave({ bodyRow, outfit, hair })
    } catch (e) {
      console.error('Failed to save appearance:', e)
    }
  }

  function handleReset() {
    bodyRow = 0
    outfit = 'outfit1'
    hair = 'hair1'
  }
</script>

<div class="appearance-editor">
  <h3 class="editor-title">{$t('appearance.title')}</h3>

  <div class="preview-section">
    <canvas bind:this={previewCanvas} class="preview-canvas"></canvas>
  </div>

  <div class="option-section">
    <span class="option-label">{$t('appearance.skin')}</span>
    <div class="option-row">
      {#each SKIN_COLORS as color, i}
        <button
          class="skin-btn"
          class:selected={bodyRow === i}
          style="background-color: {color}"
          on:click={() => bodyRow = i}
          title="Skin {i}"
        ></button>
      {/each}
    </div>
  </div>

  <div class="option-section">
    <span class="option-label">{$t('appearance.outfit')}</span>
    <div class="option-row">
      {#each OUTFITS as o, i}
        <button
          class="outfit-btn"
          class:selected={outfit === o}
          on:click={() => outfit = o}
        >
          {i + 1}
        </button>
      {/each}
    </div>
  </div>

  <div class="option-section">
    <span class="option-label">{$t('appearance.hair')}</span>
    <div class="option-row">
      {#each HAIRS as h, i}
        <button
          class="hair-btn"
          class:selected={hair === h}
          on:click={() => hair = h}
        >
          {i + 1}
        </button>
      {/each}
    </div>
  </div>

  <div class="button-row">
    <button class="nes-btn is-warning" on:click={handleReset}>{$t('appearance.reset')}</button>
    <button class="nes-btn" on:click={onCancel}>{$t('common.cancel')}</button>
    <button class="nes-btn is-primary" on:click={handleSave}>{$t('appearance.save')}</button>
  </div>
</div>

<style>
  .appearance-editor {
    padding: 12px;
    background: var(--card-bg, #161b22);
    border: 2px solid var(--border-color, #30363d);
    max-width: 320px;
  }

  .editor-title {
    font-size: 12px;
    color: var(--accent-green, #00ff41);
    margin: 0 0 12px 0;
    font-family: 'Press Start 2P', monospace;
  }

  .preview-section {
    display: flex;
    justify-content: center;
    margin-bottom: 12px;
  }

  .preview-canvas {
    image-rendering: pixelated;
    border: 2px solid var(--border-color, #30363d);
    background: #0d1117;
  }

  .option-section {
    margin-bottom: 10px;
  }

  .option-label {
    display: block;
    font-size: 9px;
    color: #888;
    margin-bottom: 4px;
    font-family: 'Press Start 2P', monospace;
  }

  .option-row {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  .skin-btn {
    width: 28px;
    height: 28px;
    border: 2px solid #555;
    cursor: pointer;
    padding: 0;
  }

  .skin-btn.selected {
    border-color: var(--accent-green, #00ff41);
    box-shadow: 0 0 4px var(--accent-green, #00ff41);
  }

  .outfit-btn, .hair-btn {
    width: 32px;
    height: 28px;
    font-size: 10px;
    font-family: 'Press Start 2P', monospace;
    background: #21262d;
    color: #c9d1d9;
    border: 2px solid #555;
    cursor: pointer;
    padding: 0;
  }

  .outfit-btn.selected, .hair-btn.selected {
    border-color: var(--accent-green, #00ff41);
    background: #1a3a1a;
    color: var(--accent-green, #00ff41);
  }

  .button-row {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
    margin-top: 12px;
  }

  .button-row .nes-btn {
    font-size: 8px !important;
    padding: 4px 10px !important;
  }
</style>
