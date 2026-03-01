<script>
  export let event = {}
  export let isLeader = false

  function phaseLabel(phase) {
    switch (phase) {
      case 'opinion': return 'Opinion'
      case 'roundtable': return 'Discussion'
      case 'decision': return 'Final Decision'
      default: return phase
    }
  }

  function avatarIcon(roleName) {
    const name = (roleName || '').toLowerCase()
    if (name.includes('gatekeeper')) return '★'
    if (name.includes('manager') || name.includes('rd')) return '♟'
    if (name.includes('pm') || name.includes('product')) return '♙'
    if (name.includes('security')) return '♜'
    if (name.includes('leader')) return '♚'
    return '♞'
  }

  function confidenceColor(conf) {
    if (conf >= 0.8) return 'status-active'
    if (conf >= 0.5) return 'status-paused'
    return 'status-error'
  }
</script>

<div class="message" class:is-decision={event.phase === 'decision'} class:is-leader={isLeader}>
  <div class="message-header">
    <span class="avatar">{avatarIcon(event.roleName)}</span>
    <span class="role-name">{event.roleName}</span>
    <span class="nes-badge">
      <span class="is-primary">{phaseLabel(event.phase)}</span>
    </span>
    {#if event.action}
      <span class="nes-badge">
        <span class="is-warning">{event.action}</span>
      </span>
    {/if}
    <span class={confidenceColor(event.confidence)}>
      {(event.confidence * 100).toFixed(0)}%
    </span>
  </div>

  <div class="nes-balloon from-left">
    <p>{event.message || '(no reasoning)'}</p>
  </div>
</div>

<style>
  .message {
    margin-bottom: 16px;
  }

  .message.is-decision {
    border: 2px solid var(--accent-green);
    padding: 8px;
  }

  .message-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
    font-size: 10px;
  }

  .avatar {
    font-size: 16px;
  }

  .role-name {
    color: var(--accent-blue);
    font-weight: bold;
  }

  .nes-balloon {
    font-size: 9px;
    max-width: 100%;
  }

  .nes-balloon p {
    margin: 0;
  }
</style>
