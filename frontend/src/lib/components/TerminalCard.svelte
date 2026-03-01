<script>
  export let session = {}
  export let onClick = () => {}

  function statusClass(status) {
    switch (status) {
      case 'active': return 'is-success'
      case 'paused': return 'is-warning'
      case 'stopped': return 'is-error'
      default: return ''
    }
  }
</script>

<div
  class="nes-container is-rounded terminal-card"
  on:click={onClick}
  on:keydown={(e) => e.key === 'Enter' && onClick()}
  role="button"
  tabindex="0"
>
  <div class="card-header">
    <span class="session-name">{session.name || session.id}</span>
    <span class="nes-badge"><span class={statusClass(session.status)}>{session.status || 'unknown'}</span></span>
  </div>
  <div class="card-body">
    <div class="card-info">
      <span class="label">tmux:</span>
      <span>{session.tmuxSession}:{session.window}.{session.pane}</span>
    </div>
    {#if session.toolType}
      <div class="card-info">
        <span class="label">tool:</span>
        <span>{session.toolType}</span>
      </div>
    {/if}
    {#if session.projectDir}
      <div class="card-info">
        <span class="label">dir:</span>
        <span class="truncate">{session.projectDir}</span>
      </div>
    {/if}
  </div>
</div>

<style>
  .terminal-card {
    cursor: pointer;
    padding: 12px !important;
    margin: 0 !important;
    transition: border-color 0.1s;
    min-width: 240px;
  }

  .terminal-card:hover {
    border-color: var(--accent-blue) !important;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .session-name {
    font-size: 11px;
    color: var(--accent-green);
  }

  .card-body {
    font-size: 9px;
  }

  .card-info {
    margin: 4px 0;
  }

  .label {
    color: var(--text-secondary);
  }

  .truncate {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-block;
    vertical-align: bottom;
  }
</style>
