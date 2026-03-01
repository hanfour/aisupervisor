<script>
  export let visible = false
  export let event = null
  export let onApprove = () => {}
  export let onDismiss = () => {}
</script>

{#if visible && event}
  <div class="dialog-overlay">
    <div class="nes-dialog is-dark is-rounded" id="confirm-dialog">
      <p class="title">Low Confidence Decision</p>
      <div class="dialog-content">
        <p><strong>Session:</strong> {event.sessionName}</p>
        <p><strong>Summary:</strong> {event.summary}</p>
        <p><strong>Suggested:</strong> {event.chosenKey}</p>
        <p><strong>Reasoning:</strong> {event.reasoning}</p>
        <p><strong>Confidence:</strong>
          <span class="status-paused">
            {(event.confidence * 100).toFixed(0)}%
          </span>
        </p>
      </div>
      <menu class="dialog-menu">
        <button class="nes-btn is-success" on:click={onApprove}>Approve</button>
        <button class="nes-btn is-error" on:click={onDismiss}>Dismiss</button>
      </menu>
    </div>
  </div>
{/if}

<style>
  .dialog-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .nes-dialog {
    max-width: 500px;
    width: 90%;
    padding: 24px !important;
  }

  .title {
    color: var(--accent-yellow);
    margin-bottom: 16px;
  }

  .dialog-content {
    font-size: 10px;
    margin-bottom: 16px;
  }

  .dialog-content p {
    margin: 6px 0;
  }

  .dialog-menu {
    display: flex;
    gap: 12px;
    justify-content: flex-end;
    padding: 0;
    margin: 0;
  }
</style>
