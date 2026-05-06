<script>
  import { onMount, afterUpdate } from 'svelte'
  import { prerenderCharacter, prerenderCharacterFromAppearance, getCharacterType, spritesReady, loadAllSprites, loadCustomSpriteSheet } from '../office/sprites.js'
  import { renderPortrait, getWorkerPortraitProfile } from '../office/characterPortrait.js'

  // Either pass a profileId directly, or pass a worker object
  export let profileId = ''
  export let worker = null
  export let scale = 3       // 3× → 96×96px display canvas
  export let size = null     // override CSS display size (px)

  let canvas

  async function paint() {
    if (!canvas) return

    // Try office sprites first (matches the pixel office characters)
    if (worker && !spritesReady()) {
      await loadAllSprites()
    }

    if (worker && spritesReady()) {
      // 1. AI sprite sheet (when SpriteSheetPath is set) — highest priority.
      //    Slices the same 24-col PNG the office renderer uses; idle[0] is
      //    the south-facing rest pose.
      let cache = null
      if (worker.appearance?.spriteSheetPath) {
        try {
          cache = await loadCustomSpriteSheet(worker.id)
        } catch {
          cache = null
        }
      }

      // 2. Layered renderer fallback (body+outfit+hair from appearance).
      //    Skipped when the layered fields are empty (e.g. AI-only worker
      //    where the AI sheet failed to load) — falls through to (3).
      if (!cache && worker.appearance && (worker.appearance.outfit || worker.appearance.hair)) {
        cache = prerenderCharacterFromAppearance(worker.appearance)
      }

      // 3. Static character-type lookup (Olivia → designer, etc.) — used
      //    when the worker has no custom appearance fields at all.
      if (!cache) {
        cache = prerenderCharacter(getCharacterType(worker))
      }

      if (cache && cache.idle && cache.idle[0]) {
        const frame = cache.idle[0]
        const displaySize = size || (32 * scale)
        canvas.width = displaySize
        canvas.height = displaySize
        const ctx = canvas.getContext('2d')
        ctx.imageSmoothingEnabled = false
        ctx.clearRect(0, 0, displaySize, displaySize)
        ctx.drawImage(frame, 0, 0, displaySize, displaySize)
        return
      }
    }

    // Fallback to hand-drawn portrait
    const resolvedProfile = worker
      ? getWorkerPortraitProfile(worker)
      : (profileId || 'coder')
    const src = renderPortrait(resolvedProfile, scale)
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingEnabled = false
    canvas.width = src.width
    canvas.height = src.height
    ctx.drawImage(src, 0, 0)
  }

  onMount(paint)
  afterUpdate(paint)

  $: profileId, worker, scale, paint && paint()
</script>

<canvas
  bind:this={canvas}
  class="character-portrait"
  style:width={size ? `${size}px` : null}
  style:height={size ? `${size}px` : null}
></canvas>

<style>
  .character-portrait {
    image-rendering: pixelated;
    image-rendering: crisp-edges;
    display: block;
  }
</style>
