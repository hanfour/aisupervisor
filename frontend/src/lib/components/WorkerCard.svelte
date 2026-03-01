<script>
  export let worker = {}

  const avatars = {
    'robot': 'i.nes-octocat',
    'cat': 'i.nes-octocat',
    'kirby': 'i.nes-kirby',
    'mario': 'i.nes-mario',
    'ash': 'i.nes-ash',
    'bulbasaur': 'i.nes-bulbasaur',
    'charmander': 'i.nes-charmander',
    'squirtle': 'i.nes-squirtle',
    'pokeball': 'i.nes-pokeball',
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

<div class="nes-container is-rounded worker-card">
  <div class="card-avatar">
    <i class={avatars[worker.avatar] || 'nes-octocat'}></i>
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

  .card-avatar i {
    transform: scale(1.5);
    display: inline-block;
    margin: 8px 0;
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
</style>
