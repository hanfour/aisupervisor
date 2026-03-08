<script>
  import { openChat } from '../stores/workerChat.js'
  import CharacterPortrait from './CharacterPortrait.svelte'

  export let worker = {}
  export let onClick = null

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
  <button class="nes-btn is-primary chat-btn" on:click|stopPropagation={() => openChat(worker.id, worker.name, worker.avatar)}>
    Chat
  </button>
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

  .chat-btn {
    font-size: 7px !important;
    padding: 2px 8px !important;
    margin-top: 4px;
  }
</style>
