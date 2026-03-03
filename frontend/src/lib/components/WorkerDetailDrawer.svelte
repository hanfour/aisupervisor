<script>
  import { onMount } from 'svelte'
  import { getWorker, getManager, getSubordinates, skillProfiles, loadSkillProfiles, loadWorkers, loadHierarchy } from '../stores/workers.js'
  import { loadCharacterProfile, loadWorkerRelationships, generateNarrative } from '../stores/personality.js'
  import WorkerLogPanel from './WorkerLogPanel.svelte'
  import { t } from '../stores/i18n.js'

  export let workerId = ''
  export let onClose = () => {}
  export let onSelectWorker = () => {}

  let worker = null
  let manager = null
  let subordinates = []
  let showLogs = false
  let loading = false
  let editingSkill = false
  let selectedSkill = ''
  let profile = null
  let workerRelationships = []

  const avatarMap = {
    robot: '🤖', cat: '🐱', kirby: '⭐', mario: '🍄',
    ash: '⚡', bulbasaur: '🌿', charmander: '🔥', squirtle: '💧', pokeball: '⚪',
  }

  const traitKeys = {
    sociability: 'trait.sociability',
    focus: 'trait.focus',
    creativity: 'trait.creativity',
    empathy: 'trait.empathy',
    ambition: 'trait.ambition',
    humor: 'trait.humor'
  }

  const tierColors = {
    consultant: 'var(--accent-yellow)',
    manager: 'var(--accent-blue)',
    engineer: 'var(--accent-green)',
  }

  async function loadData(id) {
    loading = true
    worker = await getWorker(id)
    if (worker) {
      manager = await getManager(id)
      subordinates = await getSubordinates(id)
      selectedSkill = worker.skillProfile || ''
      await loadSkillProfiles()
      profile = await loadCharacterProfile(id)
      workerRelationships = await loadWorkerRelationships(id)
    }
    loading = false
  }

  async function handleSkillChange() {
    if (!worker) return
    try {
      const val = selectedSkill || '-'
      await window.go.gui.CompanyApp.UpdateWorkerFields(worker.id, '', '', '', val)
      editingSkill = false
      await loadData(worker.id)
      await loadWorkers()
      await loadHierarchy()
    } catch (e) {
      // ignore
    }
  }

  async function handleGenerateNarrative() {
    try {
      await generateNarrative(workerId)
      profile = await loadCharacterProfile(workerId)
    } catch (e) {
      console.error(e)
    }
  }

  $: if (workerId) loadData(workerId)
</script>

{#key workerId}
<div class="drawer-overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
  <div class="drawer" on:click|stopPropagation role="presentation">
    <div class="drawer-header">
      <span class="drawer-title">{$t('workerDetail.title')}</span>
      <button class="nes-btn btn-close" on:click={onClose}>&times;</button>
    </div>

    {#if loading}
      <div class="loading">{$t('common.loading')}</div>
    {:else if worker}
      <div class="drawer-body">
        <!-- Identity -->
        <div class="identity-section">
          <span class="avatar-large">{avatarMap[worker.avatar] || '🤖'}</span>
          <div class="identity-info">
            <span class="worker-name">{worker.name}</span>
            <span class="tier-badge" style="color: {tierColors[worker.tier] || 'var(--text-primary)'}">
              [{worker.tier}]
            </span>
          </div>
        </div>

        <!-- Status -->
        <div class="detail-row">
          <span class="label">{$t('workerDetail.status')}</span>
          <span class="nes-badge"><span class="is-primary">{worker.status}</span></span>
        </div>

        <!-- IDs & Config -->
        <div class="detail-row">
          <span class="label">{$t('workerDetail.id')}</span>
          <span class="value mono">{worker.id}</span>
        </div>
        {#if worker.backendId}
          <div class="detail-row">
            <span class="label">{$t('workerDetail.backend')}</span>
            <span class="value">{worker.backendId}</span>
          </div>
        {/if}
        {#if worker.cliTool}
          <div class="detail-row">
            <span class="label">{$t('workerDetail.cliTool')}</span>
            <span class="value">{worker.cliTool}</span>
          </div>
        {/if}
        {#if worker.modelVersion}
          <div class="detail-row">
            <span class="label">{$t('workerDetail.model')}</span>
            <span class="value">{worker.modelVersion}</span>
          </div>
        {/if}

        <!-- Skill Profile -->
        <div class="detail-row">
          <span class="label">{$t('workerDetail.skillProfile')}</span>
          {#if editingSkill}
            <div class="skill-edit">
              <select class="skill-select" bind:value={selectedSkill}>
                <option value="">None</option>
                {#each $skillProfiles as sp}
                  <option value={sp.id}>{sp.icon} {sp.name}</option>
                {/each}
              </select>
              <button class="nes-btn is-success btn-sm" on:click={handleSkillChange}>OK</button>
              <button class="nes-btn btn-sm" on:click={() => { editingSkill = false; selectedSkill = worker.skillProfile || '' }}>X</button>
            </div>
          {:else}
            <span class="value skill-value" on:click={() => editingSkill = true} on:keydown={(e) => e.key === 'Enter' && (editingSkill = true)} role="button" tabindex="0">
              {#if worker.skillProfile}
                {@const sp = $skillProfiles.find(p => p.id === worker.skillProfile)}
                {#if sp}
                  {sp.icon} {sp.name}
                {:else}
                  {worker.skillProfile}
                {/if}
              {:else}
                <span class="empty-text">{$t('workerDetail.noneClickToSet')}</span>
              {/if}
            </span>
          {/if}
        </div>
        {#if worker.skillProfile && !editingSkill}
          {@const sp = $skillProfiles.find(p => p.id === worker.skillProfile)}
          {#if sp}
            <div class="skill-description">{sp.description}</div>
          {/if}
        {/if}

        {#if worker.createdAt}
          <div class="detail-row">
            <span class="label">{$t('workerDetail.created')}</span>
            <span class="value">{new Date(worker.createdAt).toLocaleString()}</span>
          </div>
        {/if}

        <!-- Current Task -->
        {#if worker.currentTaskId}
          <div class="detail-row">
            <span class="label">{$t('workerDetail.task')}</span>
            <span class="value mono">{worker.currentTaskId}</span>
          </div>
        {/if}

        <!-- Manager -->
        <div class="section-title">{$t('workerDetail.manager')}</div>
        {#if manager}
          <button class="nes-btn is-primary link-btn" on:click={() => onSelectWorker(manager.id)}>
            {avatarMap[manager.avatar] || '🤖'} {manager.name} [{manager.tier}]
          </button>
        {:else}
          <span class="empty-text">{$t('workerDetail.noNone')}</span>
        {/if}

        <!-- Subordinates -->
        <div class="section-title">{$t('workerDetail.subordinates')} ({subordinates.length})</div>
        {#if subordinates.length > 0}
          <div class="sub-list">
            {#each subordinates as sub}
              <button class="nes-btn link-btn" on:click={() => onSelectWorker(sub.id)}>
                {avatarMap[sub.avatar] || '🤖'} {sub.name} [{sub.tier}]
              </button>
            {/each}
          </div>
        {:else}
          <span class="empty-text">{$t('workerDetail.noSubs')}</span>
        {/if}

        <!-- View Logs -->
        {#if worker.tmuxSession}
          <button class="nes-btn is-warning logs-btn" on:click={() => showLogs = true}>
            {$t('workerDetail.viewLogs')}
          </button>
        {/if}

        {#if profile}
        <section class="nes-container is-dark" style="margin-top: 12px;">
          <h3 class="section-title">{$t('workerDetail.personalitySection')}</h3>
          <p style="font-size: 11px; color: #aaa; margin-bottom: 8px;">
            {profile.narrative?.description || $t('workerDetail.noNarrative')}
          </p>

          {#if profile.narrative?.catchphrases?.length}
          <div style="margin-bottom: 8px;">
            {#each profile.narrative.catchphrases as phrase}
            <span class="nes-badge" style="margin: 2px;"><span class="is-primary">{phrase}</span></span>
            {/each}
          </div>
          {/if}

          <h4 style="font-size: 11px; margin-top: 8px;">{$t('workerDetail.mood')}</h4>
          <div style="font-size: 10px;">
            <label>{$t('workerDetail.moodCurrent')} {profile.mood?.current || 'neutral'}</label>
            <progress class="nes-progress is-primary" value={profile.mood?.energy || 0} max="100" style="height: 12px;"></progress>
            <label>{$t('workerDetail.energy')} {profile.mood?.energy || 0}%</label>
            <progress class="nes-progress is-success" value={profile.mood?.morale || 0} max="100" style="height: 12px;"></progress>
            <label>{$t('workerDetail.morale')} {profile.mood?.morale || 0}%</label>
          </div>

          <h4 style="font-size: 11px; margin-top: 8px;">{$t('workerDetail.traits')}</h4>
          <div style="font-size: 10px;">
            {#each Object.entries(profile.traits || {}) as [key, value]}
            <div style="display: flex; align-items: center; gap: 4px; margin: 2px 0;">
              <span style="width: 50px;">{$t(traitKeys[key] || key)}</span>
              <progress class="nes-progress" value={value} max="100" style="height: 8px; flex: 1;"></progress>
              <span style="width: 24px; text-align: right;">{value}</span>
            </div>
            {/each}
          </div>
        </section>

        {#if !profile.narrative?.description}
        <button class="nes-btn is-primary" style="margin-top: 8px; font-size: 10px;" on:click={handleGenerateNarrative}>
          {$t('workerDetail.generateNarrative')}
        </button>
        {/if}
        {/if}

        {#if workerRelationships.length > 0}
        <section class="nes-container is-dark" style="margin-top: 12px;">
          <h3 class="section-title">{$t('workerDetail.relationshipsSection')}</h3>
          {#each workerRelationships as rel}
          <div style="margin-bottom: 8px; font-size: 10px;">
            <span style="color: #00ff41;">
              {rel.workerA === workerId ? rel.workerB : rel.workerA}
            </span>
            <div style="display: flex; gap: 4px; align-items: center;">
              <span>{$t('workerDetail.affinity')}</span>
              <progress class="nes-progress is-warning" value={rel.affinity} max="100" style="height: 8px; flex: 1;"></progress>
              <span>{rel.affinity}</span>
            </div>
            <div style="display: flex; gap: 4px; align-items: center;">
              <span>{$t('workerDetail.trust')}</span>
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
        </section>
        {/if}
      </div>
    {:else}
      <div class="loading">{$t('workerDetail.notFound')}</div>
    {/if}
  </div>
</div>
{/key}

{#if showLogs && worker}
  <WorkerLogPanel
    workerId={worker.id}
    workerName={worker.name}
    onClose={() => showLogs = false}
  />
{/if}

<style>
  .drawer-overlay {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.6);
    display: flex;
    justify-content: flex-end;
    z-index: 150;
  }

  .drawer {
    width: 380px;
    max-width: 90vw;
    height: 100vh;
    background: var(--bg-secondary);
    border-left: 4px solid var(--border-color);
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-bottom: 2px solid var(--border-color);
  }

  .drawer-title {
    font-size: 11px;
    color: var(--accent-blue);
  }

  .btn-close {
    font-size: 14px !important;
    padding: 2px 8px !important;
    line-height: 1;
  }

  .drawer-body {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .identity-section {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-bottom: 10px;
    border-bottom: 2px solid var(--border-color);
  }

  .avatar-large {
    font-size: 32px;
  }

  .identity-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .worker-name {
    font-size: 12px;
    color: var(--accent-green);
  }

  .tier-badge {
    font-size: 10px;
    font-weight: bold;
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 9px;
  }

  .label {
    color: var(--text-secondary);
  }

  .value {
    color: var(--text-primary);
  }

  .mono {
    font-family: monospace;
    font-size: 8px;
  }

  .section-title {
    font-size: 10px;
    color: var(--accent-blue);
    margin-top: 8px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 4px;
  }

  .link-btn {
    font-size: 9px !important;
    padding: 4px 8px !important;
    text-align: left;
    width: 100%;
  }

  .sub-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .empty-text {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .logs-btn {
    font-size: 9px !important;
    margin-top: 12px;
  }

  .loading {
    padding: 24px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 10px;
  }

  .skill-value {
    cursor: pointer;
    padding: 2px 4px;
    border: 1px dashed transparent;
    transition: border-color 0.2s;
  }

  .skill-value:hover {
    border-color: var(--accent-green);
  }

  .skill-edit {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .skill-select {
    font-size: 9px;
    padding: 2px 4px;
    background: var(--bg-primary);
    color: var(--text-primary);
    border: 2px solid var(--border-color);
    max-width: 140px;
  }

  .btn-sm {
    font-size: 7px !important;
    padding: 1px 6px !important;
  }

  .skill-description {
    font-size: 8px;
    color: var(--text-secondary);
    padding: 4px 8px;
    background: rgba(0,255,65,0.05);
    border-left: 2px solid var(--accent-green);
  }
</style>
