<script>
  import { onMount } from 'svelte'
  import { getWorker, getManager, getSubordinates } from '../stores/workers.js'
  import WorkerLogPanel from './WorkerLogPanel.svelte'

  export let workerId = ''
  export let onClose = () => {}
  export let onSelectWorker = () => {}

  let worker = null
  let manager = null
  let subordinates = []
  let showLogs = false
  let loading = true

  const avatarMap = {
    robot: '🤖', cat: '🐱', kirby: '⭐', mario: '🍄',
    ash: '⚡', bulbasaur: '🌿', charmander: '🔥', squirtle: '💧', pokeball: '⚪',
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
    }
    loading = false
  }

  $: if (workerId) loadData(workerId)
</script>

{#key workerId}
<div class="drawer-overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
  <div class="drawer" on:click|stopPropagation role="presentation">
    <div class="drawer-header">
      <span class="drawer-title">Worker Detail</span>
      <button class="nes-btn btn-close" on:click={onClose}>&times;</button>
    </div>

    {#if loading}
      <div class="loading">Loading...</div>
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
          <span class="label">Status</span>
          <span class="nes-badge"><span class="is-primary">{worker.status}</span></span>
        </div>

        <!-- IDs & Config -->
        <div class="detail-row">
          <span class="label">ID</span>
          <span class="value mono">{worker.id}</span>
        </div>
        {#if worker.backendId}
          <div class="detail-row">
            <span class="label">Backend</span>
            <span class="value">{worker.backendId}</span>
          </div>
        {/if}
        {#if worker.cliTool}
          <div class="detail-row">
            <span class="label">CLI Tool</span>
            <span class="value">{worker.cliTool}</span>
          </div>
        {/if}
        {#if worker.modelVersion}
          <div class="detail-row">
            <span class="label">Model</span>
            <span class="value">{worker.modelVersion}</span>
          </div>
        {/if}
        {#if worker.createdAt}
          <div class="detail-row">
            <span class="label">Created</span>
            <span class="value">{new Date(worker.createdAt).toLocaleString()}</span>
          </div>
        {/if}

        <!-- Current Task -->
        {#if worker.currentTaskId}
          <div class="detail-row">
            <span class="label">Task</span>
            <span class="value mono">{worker.currentTaskId}</span>
          </div>
        {/if}

        <!-- Manager -->
        <div class="section-title">Manager</div>
        {#if manager}
          <button class="nes-btn is-primary link-btn" on:click={() => onSelectWorker(manager.id)}>
            {avatarMap[manager.avatar] || '🤖'} {manager.name} [{manager.tier}]
          </button>
        {:else}
          <span class="empty-text">None</span>
        {/if}

        <!-- Subordinates -->
        <div class="section-title">Subordinates ({subordinates.length})</div>
        {#if subordinates.length > 0}
          <div class="sub-list">
            {#each subordinates as sub}
              <button class="nes-btn link-btn" on:click={() => onSelectWorker(sub.id)}>
                {avatarMap[sub.avatar] || '🤖'} {sub.name} [{sub.tier}]
              </button>
            {/each}
          </div>
        {:else}
          <span class="empty-text">No subordinates</span>
        {/if}

        <!-- View Logs -->
        {#if worker.tmuxSession}
          <button class="nes-btn is-warning logs-btn" on:click={() => showLogs = true}>
            View Logs
          </button>
        {/if}
      </div>
    {:else}
      <div class="loading">Worker not found</div>
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
</style>
