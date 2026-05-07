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

  // AI sprite cache populated by a separate async load. Kept on the
  // component instance so paint() stays synchronous (no awaits in the
  // reactive render path) — last attempt at this feature (PR #45)
  // awaited inside paint() and Vite's HMR reloaded the whole app on
  // unhandled rejections, throwing users back to the splash screen.
  let customCache = null
  let customCacheForPath = ''

  // Kick off (or refresh) the AI sprite load whenever the worker's
  // sprite path changes. Fire-and-forget — paint() runs once the
  // cache lands. The path comparison guards against re-fetching when
  // unrelated worker fields change.
  $: maybeLoadCustomSheet(worker)

  function maybeLoadCustomSheet(w) {
    const path = w?.appearance?.spriteSheetPath || ''
    if (path === customCacheForPath) return
    customCacheForPath = path
    if (!path) {
      customCache = null
      return
    }
    loadCustomSpriteSheet(w.id).then((c) => {
      // Only commit when the worker's sprite path is still the one we
      // started loading for (worker prop may have swapped during the
      // async fetch).
      if (w?.appearance?.spriteSheetPath === path) {
        customCache = c
      }
    }).catch((err) => {
      console.warn('CharacterPortrait: loadCustomSpriteSheet failed', w?.id, err)
    })
  }

  // Synchronous paint — never awaits. All async work (sprite loading,
  // AI sheet fetch) lives elsewhere; paint just reads whatever caches
  // are currently populated.
  function paint() {
    if (!canvas) return

    if (worker && spritesReady()) {
      // Resolution order:
      //   1. AI sprite sheet (already loaded into customCache).
      //   2. Layered renderer (body+outfit+hair) when those fields exist.
      //   3. Static character-type lookup as the last office-style option.
      let cache = customCache
      if (!cache && worker.appearance) {
        cache = prerenderCharacterFromAppearance(worker.appearance)
      }
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

    // Hand-drawn fallback — used when sprites aren't loaded yet or the
    // worker has no appearance data at all.
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

  // Make sure office sprites are loaded once globally (idempotent).
  // Trailing paint() call grabs whatever cache is current at that point.
  async function ensureLoaded() {
    if (worker && !spritesReady()) {
      await loadAllSprites()
    }
    paint()
  }

  onMount(ensureLoaded)
  afterUpdate(paint)

  // Sync repaint on prop / cache changes. customCache is a reactive
  // dependency so when the async load lands, paint() reruns.
  $: profileId, worker, scale, customCache, paint && paint()
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
