<script>
  import { openChat } from '../stores/workerChat.js'
  import { resetWorker } from '../stores/company.js'
  import { t } from '../stores/i18n.js'
  import { addError } from '../stores/errors.js'
  import CharacterPortrait from './CharacterPortrait.svelte'

  export let worker = {}
  export let onClick = null

  async function handleReset() {
    if (!confirm($t('workers.resetConfirm').replace('{name}', worker.name))) return
    try {
      await resetWorker(worker.id)
    } catch (e) {
      addError('Reset failed: ' + (e.message || e))
    }
  }

  function statusClass(status) {
    switch (status) {
      case 'idle': return 'is-success'
      case 'working': return 'is-primary'
      case 'waiting': return 'is-warning'
      case 'error': return 'is-error'
      case 'finished': return 'is-success'
      default: return ''
    }
  }
</script>

<div class="nes-container is-rounded worker-card" class:clickable={!!onClick} on:click={() => onClick && onClick(worker)} on:keydown={(e) => e.key === 'Enter' && onClick && onClick(worker)} role="button" tabindex="0">
  <div class="card-avatar">
    <CharacterPortrait {worker} scale={3} size={64} />
  </div>
  <div class="card-info">
    <span class="worker-name">{worker.name}</span>
    <span class="nes-badge"><span class={statusClass(worker.status)}>{worker.status}</span></span>
  </div>
  {#if worker.currentTaskId}
    <div class="card-task">
      <span class="label">Task:</span> {worker.currentTaskId}
    </div>
  {/if}
  <div class="card-btns">
    <button class="nes-btn is-primary chat-btn" on:click|stopPropagation={() => openChat(worker.id, worker.name, worker.avatar)}>
      Chat
    </button>
    {#if worker.status !== 'idle'}
      <button class="nes-btn is-error chat-btn" on:click|stopPropagation={handleReset}>
        Reset
      </button>
    {/if}
  </div>
</div>

<style>
  .worker-card {
    padding: 12px !important;
    margin: 0 !important;
    min-width: 200px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .worker-card.clickable {
    cursor: pointer;
  }

  .worker-card.clickable:hover {
    border-color: var(--accent-green);
  }

  .card-avatar {
    margin: 4px 0;
    line-height: 0;
    border: 2px solid var(--border-color, #333);
    image-rendering: pixelated;
  }

  .card-info {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .worker-name {
    font-size: 11px;
    color: var(--accent-green);
  }

  .card-task {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .label {
    color: var(--text-secondary);
  }

  .card-btns {
    display: flex;
    gap: 4px;
    margin-top: 4px;
  }

  .chat-btn {
    font-size: 7px !important;
    padding: 2px 8px !important;
  }
</style>
