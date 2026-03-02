<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { workers as workersStore, loadWorkers } from '../stores/workers.js'
  import { events } from '../stores/events.js'
  import { assignDesksToWorkers } from '../office/layout.js'
  import { OfficeRenderer } from '../office/officeRenderer.js'
  import { SimulationEngine } from '../office/simulation.js'
  import { loadAllSprites } from '../office/sprites.js'
  import { playFinished, playError, playAssign, setSoundEnabled, isSoundEnabled } from '../office/sounds.js'
  import CharacterProfilePage from '../components/CharacterProfilePage.svelte'

  let canvasEl
  let renderer
  let simulation
  let selectedWorkerId = null
  let soundOn = isSoundEnabled()
  let pollTimer
  let prevStatuses = {}

  // Workers not assigned to any desk (overflow)
  let overflowWorkers = []

  $: allWorkers = $workersStore || []

  onMount(async () => {
    // Clear stale desk assignments from previous sessions
    localStorage.removeItem('pixelOffice_deskAssignments')

    await loadAllSprites()  // load MetroCity PNG spritesheets
    await loadWorkers()
    await tick()  // ensure canvas element is bound
    initRenderer()

    // Retry renderer init after a delay in case sprites weren't fully decoded
    setTimeout(() => updateRenderer(), 500)
    setTimeout(() => updateRenderer(), 1500)

    pollTimer = setInterval(async () => {
      await loadWorkers()
      updateRenderer()
    }, 3000)
  })

  onDestroy(() => {
    if (renderer) renderer.destroy()
    if (pollTimer) clearInterval(pollTimer)
  })

  function initRenderer() {
    if (!canvasEl) return
    renderer = new OfficeRenderer(canvasEl)
    simulation = new SimulationEngine(renderer)
    renderer.setSimulation(simulation)
    updateRenderer()
    renderer.start()
  }

  function updateRenderer() {
    if (!renderer) {
      initRenderer()  // retry init if renderer wasn't created yet
      if (!renderer) return
    }
    const workers = $workersStore || []
    const assignments = assignDesksToWorkers(workers)
    renderer.setWorkers(workers, assignments)

    // Detect status changes for sounds
    for (const w of workers) {
      const prev = prevStatuses[w.id]
      if (prev && prev !== w.status) {
        if (w.status === 'finished' || w.status === 'done' || w.status === 'completed') playFinished()
        else if (w.status === 'error' || w.status === 'failed') playError()
        else if (w.status === 'working' || w.status === 'busy') playAssign()
      }
      prevStatuses[w.id] = w.status
    }

    // Find overflow workers (no desk assigned)
    const assignedIds = new Set(Object.values(assignments))
    overflowWorkers = workers.filter(w => !assignedIds.has(w.id))
  }

  // React to event store changes
  $: if ($events) {
    updateRenderer()
    const latest = $events[$events.length - 1]
    if (latest && simulation) simulation.handleEvent(latest)
  }

  function handleCanvasClick(e) {
    if (!renderer) return
    const rect = canvasEl.getBoundingClientRect()
    const scaleX = canvasEl.width / rect.width
    const scaleY = canvasEl.height / rect.height
    const x = (e.clientX - rect.left) * scaleX
    const y = (e.clientY - rect.top) * scaleY
    const w = renderer.getWorkerAtPixel(x, y)
    if (w) {
      selectedWorkerId = w.id
    }
  }

  function handleCanvasMove(e) {
    if (!renderer) return
    const rect = canvasEl.getBoundingClientRect()
    const scaleX = canvasEl.width / rect.width
    const scaleY = canvasEl.height / rect.height
    const x = (e.clientX - rect.left) * scaleX
    const y = (e.clientY - rect.top) * scaleY
    const w = renderer.getWorkerAtPixel(x, y)
    renderer.setHoveredWorker(w ? w.id : null)
    canvasEl.style.cursor = w ? 'pointer' : 'default'
  }

  function toggleSound() {
    soundOn = !soundOn
    setSoundEnabled(soundOn)
  }

  function handleSelectWorker(id) {
    selectedWorkerId = id
  }
</script>

<div class="office-wrapper">
  <!-- Office canvas always in DOM and always rendering -->
  <div class="office-page">
    <div class="office-header">
      <h2 class="office-title">&#9635; PIXEL OFFICE</h2>
      <div class="header-controls">
        <span class="worker-count">{allWorkers.length} workers</span>
        <button class="nes-btn sound-btn" class:is-primary={soundOn} on:click={toggleSound}>
          {soundOn ? '&#9835; ON' : '&#9835; OFF'}
        </button>
      </div>
    </div>

    <div class="canvas-container">
      <canvas
        bind:this={canvasEl}
        on:click={handleCanvasClick}
        on:mousemove={handleCanvasMove}
        class="office-canvas"
      ></canvas>
    </div>

    {#if overflowWorkers.length > 0}
      <div class="nes-container is-dark overflow-list">
        <p class="overflow-title">Workers without desks ({overflowWorkers.length})</p>
        <div class="overflow-grid">
          {#each overflowWorkers as w}
            <button class="nes-btn overflow-btn" on:click={() => selectedWorkerId = w.id}>
              {w.name} ({w.status || 'idle'})
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- Character profile overlays on top -->
  {#if selectedWorkerId}
    <div class="profile-overlay">
      <CharacterProfilePage
        workerId={selectedWorkerId}
        onBack={() => selectedWorkerId = null}
        onSelectWorker={handleSelectWorker}
      />
    </div>
  {/if}
</div>

<style>
  .office-wrapper {
    position: relative;
    height: 100%;
    overflow: hidden;
  }

  .office-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 8px;
  }

  .profile-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 10;
    background: var(--bg-color, #0d1117);
    overflow: auto;
  }

  .office-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 2px solid var(--border-color);
  }

  .office-title {
    font-size: 14px;
    color: var(--accent-green);
    margin: 0;
  }

  .header-controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .worker-count {
    font-size: 9px;
    color: #888;
  }

  .sound-btn {
    font-size: 8px !important;
    padding: 4px 8px !important;
  }

  .canvas-container {
    flex: 1;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    overflow: auto;
    padding: 8px;
  }

  .office-canvas {
    image-rendering: pixelated;
    max-width: 100%;
    height: auto;
    border: 4px solid var(--border-color);
    background: #1a1a2e;
  }

  .overflow-list {
    padding: 8px !important;
  }

  .overflow-title {
    font-size: 9px;
    color: var(--accent-blue);
    margin-bottom: 6px;
  }

  .overflow-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .overflow-btn {
    font-size: 7px !important;
    padding: 3px 6px !important;
  }
</style>
