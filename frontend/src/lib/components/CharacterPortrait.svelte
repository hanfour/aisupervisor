<script>
  import { onMount, afterUpdate } from 'svelte'
  import { renderPortrait, getWorkerPortraitProfile } from '../office/characterPortrait.js'

  // Either pass a profileId directly, or pass a worker object
  export let profileId = ''
  export let worker = null
  export let scale = 3       // 3× → 96×96px display canvas
  export let size = null     // override CSS display size (px)

  let canvas
  let resolvedProfile = 'coder'

  function resolve() {
    if (worker) {
      resolvedProfile = getWorkerPortraitProfile(worker)
    } else if (profileId) {
      resolvedProfile = profileId
    }
  }

  function paint() {
    if (!canvas) return
    resolve()
    const src = renderPortrait(resolvedProfile, scale)
    const ctx = canvas.getContext('2d')
    ctx.imageSmoothingEnabled = false
    canvas.width  = src.width
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
  title={resolvedProfile}
></canvas>

<style>
  .character-portrait {
    image-rendering: pixelated;
    image-rendering: crisp-edges;
    display: block;
  }
</style>
