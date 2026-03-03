<script>
  import { onMount, onDestroy } from 'svelte'
  import { getWorker, getManager, getSubordinates } from '../stores/workers.js'
  import { loadCharacterProfile, loadWorkerRelationships, generateNarrative } from '../stores/personality.js'
  import { events } from '../stores/events.js'
  import { prerenderCharacter, getCharacterType, loadAllSprites, spritesReady } from '../office/sprites.js'
  import { openChat } from '../stores/workerChat.js'

  export let workerId
  export let onBack = () => {}
  export let onSelectWorker = () => {}

  let worker = null
  let manager = null
  let subordinates = []
  let workerEvents = []
  let profile = null
  let workerRelationships = []
  let portraitCanvas
  let animFrame = 0
  let animTimer

  $: workerEvents = ($events || []).filter(e =>
    e.workerID === workerId || e.workerId === workerId
  ).slice(0, 20)

  $: tasksDone = workerEvents.filter(e =>
    e.type === 'task_completed' || e.type === 'finished'
  ).length

  $: errorCount = workerEvents.filter(e =>
    e.type === 'error' || e.type === 'task_failed'
  ).length

  $: errorRate = workerEvents.length > 0
    ? Math.round((errorCount / workerEvents.length) * 100)
    : 0

  onMount(async () => {
    await loadAllSprites()
    await loadData()
    startPortraitAnim()
  })

  onDestroy(() => {
    if (animTimer) clearInterval(animTimer)
  })

  const traitLabels = {
    sociability: '社交性',
    focus: '專注力',
    creativity: '創造力',
    empathy: '同理心',
    ambition: '野心',
    humor: '幽默感'
  }

  async function loadData() {
    worker = await getWorker(workerId)
    if (!worker) return
    manager = await getManager(workerId)
    subordinates = await getSubordinates(workerId) || []
    profile = await loadCharacterProfile(workerId)
    workerRelationships = await loadWorkerRelationships(workerId)
  }

  async function handleGenerateNarrative() {
    try {
      await generateNarrative(workerId)
      profile = await loadCharacterProfile(workerId)
    } catch (e) {
      console.error(e)
    }
  }

  function startPortraitAnim() {
    if (animTimer) clearInterval(animTimer)
    animTimer = setInterval(() => {
      animFrame = (animFrame + 1) % 2
      drawPortrait()
    }, 500)
    // initial draw deferred until canvas available
    requestAnimationFrame(drawPortrait)
  }

  function drawPortrait() {
    if (!portraitCanvas || !worker) return
    const ctx = portraitCanvas.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.clearRect(0, 0, 128, 128)

    const charType = getCharacterType(worker, 0)
    const cache = prerenderCharacter(charType)
    if (!cache || !cache.idle) return

    const frame = cache.idle[animFrame % cache.idle.length]
    if (frame) {
      ctx.drawImage(frame, 0, 0, 128, 128)
    }
  }

  function tierLabel(tier) {
    const t = (tier || 'engineer').toLowerCase()
    return { consultant: 'Consultant', manager: 'Manager', engineer: 'Engineer' }[t] || t
  }

  function tierIcon(tier) {
    const t = (tier || 'engineer').toLowerCase()
    return { consultant: '\u265B', manager: '\u265A', engineer: '\u265F' }[t] || '\u265F'
  }

  function statusColor(status) {
    const s = (status || 'idle').toLowerCase()
    return {
      idle: 'var(--accent-blue)',
      working: 'var(--accent-green)',
      waiting: '#ffdd57',
      error: 'var(--accent-red)',
      finished: 'var(--accent-green)',
    }[s] || 'var(--text-color)'
  }

  // Skill tree progress (simplified: engineer=1, manager=2, consultant=3)
  function skillLevel(tier) {
    const t = (tier || 'engineer').toLowerCase()
    return { engineer: 1, manager: 2, consultant: 3 }[t] || 1
  }
</script>

<div class="profile-page">
  <div class="profile-header">
    <button class="nes-btn is-primary back-btn" on:click={onBack}>
      ← Back
    </button>
    <h2 class="profile-title">CHARACTER PROFILE</h2>
  </div>

  {#if worker}
    <div class="profile-grid">
      <!-- Portrait Section -->
      <div class="nes-container is-dark portrait-section">
        <canvas bind:this={portraitCanvas} width="128" height="128" class="portrait-canvas"></canvas>
        <div class="char-name">{worker.name}</div>
        <div class="char-class">
          <span class="tier-icon">{tierIcon(worker.tier)}</span>
          {tierLabel(worker.tier)}
        </div>
        <div class="char-status" style="color: {statusColor(worker.status)}">
          ● {worker.status || 'idle'}
        </div>
        <button class="nes-btn is-primary chat-profile-btn" on:click={() => openChat(worker.id, worker.name, worker.avatar)}>
          Chat with {worker.name}
        </button>
      </div>

      <!-- Equipment Section -->
      <div class="nes-container is-dark equip-section">
        <p class="section-title">⚔ EQUIPMENT</p>
        <div class="equip-list">
          <div class="equip-item">
            <span class="equip-label">Weapon</span>
            <span class="equip-value">{worker.cliTool || 'bare hands'}</span>
          </div>
          <div class="equip-item">
            <span class="equip-label">Armor</span>
            <span class="equip-value">{worker.backendID || 'none'}</span>
          </div>
          <div class="equip-item">
            <span class="equip-label">Relic</span>
            <span class="equip-value">{worker.modelVersion || worker.model || 'unknown'}</span>
          </div>
          <div class="equip-item">
            <span class="equip-label">Avatar</span>
            <span class="equip-value">{worker.avatar || 'default'}</span>
          </div>
        </div>
      </div>

      <!-- Skill Tree Section -->
      <div class="nes-container is-dark skill-section">
        <p class="section-title">⚡ SKILL TREE</p>
        <div class="skill-tree">
          <div class="skill-node" class:active={skillLevel(worker.tier) >= 1}>
            <span class="skill-icon">♟</span>
            <span>Engineer</span>
            <progress class="nes-progress is-success" value={skillLevel(worker.tier) >= 1 ? 100 : 0} max="100"></progress>
          </div>
          <div class="skill-arrow">↓</div>
          <div class="skill-node" class:active={skillLevel(worker.tier) >= 2}>
            <span class="skill-icon">♚</span>
            <span>Manager</span>
            <progress class="nes-progress is-primary" value={skillLevel(worker.tier) >= 2 ? 100 : 0} max="100"></progress>
          </div>
          <div class="skill-arrow">↓</div>
          <div class="skill-node" class:active={skillLevel(worker.tier) >= 3}>
            <span class="skill-icon">♛</span>
            <span>Consultant</span>
            <progress class="nes-progress is-warning" value={skillLevel(worker.tier) >= 3 ? 100 : 0} max="100"></progress>
          </div>
        </div>
      </div>

      <!-- Stats Section -->
      <div class="nes-container is-dark stats-section">
        <p class="section-title">📊 STATS</p>
        <div class="stats-grid">
          <div class="stat-item">
            <span class="stat-value">{tasksDone}</span>
            <span class="stat-label">Tasks Done</span>
          </div>
          <div class="stat-item">
            <span class="stat-value" style="color: {errorRate > 30 ? 'var(--accent-red)' : 'var(--text-color)'}">{errorRate}%</span>
            <span class="stat-label">Error Rate</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{workerEvents.length}</span>
            <span class="stat-label">Events</span>
          </div>
          <div class="stat-item">
            <span class="stat-value" style="color: {statusColor(worker.status)}">{worker.status || 'idle'}</span>
            <span class="stat-label">Current</span>
          </div>
        </div>
      </div>

      <!-- Personality Section -->
      {#if profile}
      <div class="nes-container is-dark personality-section">
        <p class="section-title">🎭 PERSONALITY</p>
        <p style="font-size: 9px; color: #aaa; margin-bottom: 8px;">
          {profile.narrative?.description || '尚未生成性格描述'}
        </p>

        {#if profile.narrative?.catchphrases?.length}
        <div style="margin-bottom: 8px;">
          {#each profile.narrative.catchphrases as phrase}
          <span class="nes-badge" style="margin: 2px;"><span class="is-primary">{phrase}</span></span>
          {/each}
        </div>
        {/if}

        <p class="section-title" style="margin-top: 8px;">情緒</p>
        <div style="font-size: 8px;">
          <div class="mood-row">
            <span>心情: {profile.mood?.current || 'neutral'}</span>
          </div>
          <div class="mood-row">
            <span>能量</span>
            <progress class="nes-progress is-primary" value={profile.mood?.energy || 0} max="100" style="height: 10px; flex: 1;"></progress>
            <span>{profile.mood?.energy || 0}%</span>
          </div>
          <div class="mood-row">
            <span>士氣</span>
            <progress class="nes-progress is-success" value={profile.mood?.morale || 0} max="100" style="height: 10px; flex: 1;"></progress>
            <span>{profile.mood?.morale || 0}%</span>
          </div>
        </div>

        <p class="section-title" style="margin-top: 8px;">特質</p>
        <div style="font-size: 8px;">
          {#each Object.entries(profile.traits || {}) as [key, value]}
          <div class="trait-row">
            <span class="trait-label">{traitLabels[key] || key}</span>
            <progress class="nes-progress" value={value} max="100" style="height: 8px; flex: 1;"></progress>
            <span class="trait-value">{value}</span>
          </div>
          {/each}
        </div>

        {#if !profile.narrative?.description}
        <button class="nes-btn is-primary" style="margin-top: 8px; font-size: 8px;" on:click={handleGenerateNarrative}>
          生成性格描述 (Ollama)
        </button>
        {/if}
      </div>
      {/if}

      <!-- Relationships Section -->
      {#if workerRelationships.length > 0}
      <div class="nes-container is-dark relationships-section">
        <p class="section-title">💬 RELATIONSHIPS</p>
        {#each workerRelationships as rel}
        <div style="margin-bottom: 8px; font-size: 8px;">
          <span style="color: var(--accent-green);">
            {rel.workerA === workerId ? rel.workerB : rel.workerA}
          </span>
          <div class="mood-row">
            <span>好感</span>
            <progress class="nes-progress is-warning" value={rel.affinity} max="100" style="height: 8px; flex: 1;"></progress>
            <span>{rel.affinity}</span>
          </div>
          <div class="mood-row">
            <span>信任</span>
            <progress class="nes-progress is-success" value={rel.trust} max="100" style="height: 8px; flex: 1;"></progress>
            <span>{rel.trust}</span>
          </div>
          {#if rel.tags?.length}
          <div>
            {#each rel.tags as tag}
            <span class="nes-badge" style="margin: 1px;"><span class="is-dark">{tag}</span></span>
            {/each}
          </div>
          {/if}
        </div>
        {/each}
      </div>
      {/if}

      <!-- Team Section -->
      <div class="nes-container is-dark team-section">
        <p class="section-title">👥 TEAM</p>
        {#if manager}
          <div class="team-link">
            <span class="team-role">Manager:</span>
            <button class="nes-btn is-primary team-btn" on:click={() => onSelectWorker(manager.id)}>
              {manager.name}
            </button>
          </div>
        {:else}
          <p class="team-empty">No manager (top level)</p>
        {/if}

        {#if subordinates.length > 0}
          <div class="team-subs">
            <span class="team-role">Subordinates:</span>
            {#each subordinates as sub}
              <button class="nes-btn team-btn" on:click={() => onSelectWorker(sub.id)}>
                {sub.name}
              </button>
            {/each}
          </div>
        {:else}
          <p class="team-empty">No subordinates</p>
        {/if}
      </div>

      <!-- Activity Log -->
      <div class="nes-container is-dark log-section">
        <p class="section-title">📜 ACTIVITY LOG</p>
        <div class="log-list">
          {#each workerEvents as evt}
            <div class="log-entry">
              <span class="log-type">[{evt.type}]</span>
              <span class="log-msg">{evt.message || evt.data || ''}</span>
            </div>
          {/each}
          {#if workerEvents.length === 0}
            <p class="team-empty">No recent events</p>
          {/if}
        </div>
      </div>
    </div>
  {:else}
    <div class="nes-container is-dark">
      <p>Loading character data...</p>
    </div>
  {/if}
</div>

<style>
  .profile-page {
    position: absolute;
    inset: 0;
    background: var(--bg-primary);
    overflow-y: auto;
    padding: 16px;
    z-index: 100;
  }

  .profile-header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }

  .back-btn {
    font-size: 10px !important;
    padding: 6px 12px !important;
  }

  .profile-title {
    font-size: 14px;
    color: var(--accent-green);
  }

  .profile-grid {
    display: grid;
    grid-template-columns: 200px 1fr 1fr;
    gap: 12px;
  }

  /* Portrait */
  .portrait-section {
    grid-row: span 2;
    text-align: center;
    padding: 12px !important;
  }

  .portrait-canvas {
    image-rendering: pixelated;
    width: 128px;
    height: 128px;
    border: 4px solid var(--accent-blue);
    margin-bottom: 8px;
    background: #1a1a2e;
  }

  .char-name {
    font-size: 12px;
    color: var(--accent-green);
    margin: 4px 0;
  }

  .char-class {
    font-size: 10px;
    color: var(--accent-blue);
  }

  .tier-icon {
    font-size: 14px;
  }

  .char-status {
    font-size: 9px;
    margin-top: 4px;
  }

  .chat-profile-btn {
    font-size: 8px !important;
    padding: 4px 10px !important;
    margin-top: 8px;
  }

  /* Equipment */
  .section-title {
    font-size: 10px;
    color: var(--accent-blue);
    margin-bottom: 8px;
  }

  .equip-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .equip-item {
    display: flex;
    justify-content: space-between;
    font-size: 8px;
  }

  .equip-label {
    color: #888;
  }

  .equip-value {
    color: var(--accent-green);
    max-width: 150px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Skill Tree */
  .skill-tree {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .skill-node {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 9px;
    opacity: 0.4;
    width: 100%;
  }

  .skill-node.active {
    opacity: 1;
  }

  .skill-icon {
    font-size: 14px;
  }

  .skill-node progress {
    flex: 1;
    height: 12px;
  }

  .skill-arrow {
    color: #555;
    font-size: 12px;
  }

  /* Stats */
  .stats-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .stat-item {
    text-align: center;
    padding: 6px;
    border: 2px solid var(--border-color);
  }

  .stat-value {
    font-size: 14px;
    display: block;
    color: var(--accent-green);
  }

  .stat-label {
    font-size: 7px;
    color: #888;
  }

  /* Team */
  .team-link, .team-subs {
    margin-bottom: 8px;
  }

  .team-role {
    font-size: 8px;
    color: #888;
    display: block;
    margin-bottom: 4px;
  }

  .team-btn {
    font-size: 8px !important;
    padding: 4px 8px !important;
    margin: 2px;
  }

  .team-empty {
    font-size: 8px;
    color: #666;
  }

  /* Activity Log */
  .log-section {
    grid-column: span 3;
  }

  .log-list {
    max-height: 200px;
    overflow-y: auto;
  }

  .log-entry {
    font-size: 8px;
    padding: 3px 0;
    border-bottom: 1px solid rgba(255,255,255,0.05);
  }

  .log-type {
    color: var(--accent-blue);
    margin-right: 6px;
  }

  .log-msg {
    color: var(--text-color);
  }

  /* Personality */
  .personality-section, .relationships-section {
    grid-column: span 2;
  }

  .mood-row {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 2px 0;
  }

  .trait-row {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 2px 0;
  }

  .trait-label {
    width: 50px;
    flex-shrink: 0;
  }

  .trait-value {
    width: 24px;
    text-align: right;
    flex-shrink: 0;
  }

  @media (max-width: 800px) {
    .profile-grid {
      grid-template-columns: 1fr;
    }
    .portrait-section { grid-row: auto; }
    .log-section { grid-column: auto; }
    .personality-section, .relationships-section { grid-column: auto; }
  }
</style>
