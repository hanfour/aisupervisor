<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { workers as workersStore, loadWorkers } from '../stores/workers.js'
  import { events } from '../stores/events.js'
  import { assignDesksToWorkers, clearDeskAssignments, OFFICE_LAYOUTS, getCurrentLayoutId, setCurrentLayoutId } from '../office/layout.js'
  import { rebuildGrid } from '../office/pathfinding.js'
  import { OfficeRenderer } from '../office/officeRenderer.js'
  import { SimulationEngine } from '../office/simulation.js'
  import { loadAllSprites } from '../office/sprites.js'
  import { playFinished, playError, playAssign, setSoundEnabled, isSoundEnabled } from '../office/sounds.js'
  import CharacterProfilePage from '../components/CharacterProfilePage.svelte'
  import { initPersonalityEvents, loadCharacterProfile, loadAllRelationships } from '../stores/personality.js'
  import { t } from '../stores/i18n.js'
  import { gameTimeString, currentPhase, gameClockSpeed } from '../stores/simulation.js'

  let canvasEl
  let renderer
  let simulation
  let selectedWorkerId = null
  let soundOn = isSoundEnabled()
  let clockSpeed = 1
  let pollTimer
  let prevStatuses = {}
  let layoutId = getCurrentLayoutId()

  // Workers not assigned to any desk (overflow)
  let overflowWorkers = []
  let cachedRelationships = null

  $: allWorkers = $workersStore || []

  async function loadAllProfiles(workers) {
    const profiles = new Map()
    for (const w of workers) {
      const p = await loadCharacterProfile(w.id)
      if (p) profiles.set(w.id, p)
    }
    if (renderer) renderer.setProfiles(profiles)
    if (simulation) simulation.setProfiles(profiles)
  }

  onMount(async () => {
    // Clear stale desk assignments from previous sessions
    localStorage.removeItem('pixelOffice_deskAssignments')

    initPersonalityEvents()

    await loadAllSprites()  // load MetroCity PNG spritesheets
    await loadWorkers()
    await tick()  // ensure canvas element is bound
    initRenderer()

    // Load personality profiles for all workers
    const workers = $workersStore || []
    if (workers.length > 0) {
      loadAllProfiles(workers)
      // Load relationships for social graph and desk assignment
      loadAllRelationships(workers.map(w => w.id)).then(rels => {
        cachedRelationships = rels
        if (simulation) simulation.setRelationships(rels)
        // Re-assign desks with relationship awareness
        updateRenderer()
      })
    }

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
    const assignments = assignDesksToWorkers(workers, cachedRelationships)
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

  function handleClockSpeed(e) {
    const speeds = [1, 2, 4, 8]
    const idx = parseInt(e.target.value)
    clockSpeed = speeds[idx] || 1
    if (simulation) simulation.setGameClockSpeed(clockSpeed)
  }

  function switchLayout(e) {
    const newLayoutId = e.target.value
    if (newLayoutId === layoutId) return
    layoutId = newLayoutId
    setCurrentLayoutId(layoutId)
    clearDeskAssignments()
    rebuildGrid()
    if (renderer) {
      renderer.switchLayout(layoutId)
      updateRenderer()
    }
  }

  const PHASE_LABELS = {
    morning_arrival: '上班',
    work_morning: '上午',
    lunch: '午餐',
    work_afternoon: '下午',
    tea_break: '下午茶',
    work_late: '下午',
    overtime: '加班',
    night: '深夜',
  }
</script>

<div class="office-wrapper">
  <!-- Office canvas always in DOM and always rendering -->
  <div class="office-page">
    <div class="office-header">
      <h2 class="office-title">&#9635; {$t('office.title')}</h2>
      <div class="header-controls">
        <div class="clock-hud">
          <span class="clock-time">{$gameTimeString}</span>
          <span class="clock-phase">{PHASE_LABELS[$currentPhase] || $currentPhase}</span>
          <div class="speed-control">
            <span class="speed-label">{clockSpeed}x</span>
            <input
              type="range"
              min="0"
              max="3"
              step="1"
              value={[1,2,4,8].indexOf(clockSpeed)}
              on:input={handleClockSpeed}
              class="speed-slider"
            />
          </div>
        </div>
        <select class="layout-select" bind:value={layoutId} on:change={switchLayout}>
          {#each Object.entries(OFFICE_LAYOUTS) as [id, layout]}
            <option value={id}>{$t(layout.nameKey)}</option>
          {/each}
        </select>
        <span class="worker-count">{allWorkers.length} {$t('office.workers')}</span>
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
        <p class="overflow-title">{$t('office.overflow')} ({overflowWorkers.length})</p>
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

  .layout-select {
    font-family: 'Press Start 2P', monospace;
    font-size: 7px;
    background: #21262d;
    color: #c9d1d9;
    border: 1px solid #30363d;
    padding: 3px 6px;
    cursor: pointer;
  }

  .layout-select:focus {
    border-color: var(--accent-green, #00ff41);
    outline: none;
  }

  .worker-count {
    font-size: 9px;
    color: #888;
  }

  .sound-btn {
    font-size: 8px !important;
    padding: 4px 8px !important;
  }

  .clock-hud {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: 'Press Start 2P', monospace;
  }

  .clock-time {
    font-size: 11px;
    color: #00ff41;
    text-shadow: 0 0 6px rgba(0,255,65,0.4);
  }

  .clock-phase {
    font-size: 7px;
    color: #888;
    background: rgba(0,255,65,0.1);
    padding: 2px 6px;
    border: 1px solid rgba(0,255,65,0.2);
  }

  .speed-control {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .speed-label {
    font-size: 7px;
    color: #00ddff;
    min-width: 20px;
  }

  .speed-slider {
    width: 50px;
    height: 4px;
    -webkit-appearance: none;
    appearance: none;
    background: #333;
    outline: none;
    cursor: pointer;
  }

  .speed-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 10px;
    height: 10px;
    background: #00ddff;
    cursor: pointer;
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
