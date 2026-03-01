<script>
  import { errors, dismissError } from '../stores/errors.js'
</script>

{#if $errors.length > 0}
  <div class="toast-container">
    {#each $errors as error (error.id)}
      <div class="nes-container is-rounded toast-item">
        <span class="toast-msg">{error.message}</span>
        <button class="nes-btn is-error toast-close" on:click={() => dismissError(error.id)}>X</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    top: 12px;
    right: 12px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 400px;
  }

  .toast-item {
    background: #300 !important;
    border-color: #f44 !important;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px !important;
    animation: slideIn 0.2s ease-out;
  }

  .toast-msg {
    flex: 1;
    font-size: 9px;
    color: #fcc;
  }

  .toast-close {
    font-size: 8px;
    padding: 2px 6px !important;
    min-height: unset !important;
  }

  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
</style>
