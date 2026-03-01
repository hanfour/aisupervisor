<script>
  export let branchName = ''
</script>

{#if branchName}
  <div class="branch-status">
    <span class="branch-icon">⎇</span>
    <span class="branch-name">{branchName}</span>
  </div>
{/if}

<style>
  .branch-status {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 8px;
    padding: 2px 6px;
    border: 2px solid var(--accent-blue);
    color: var(--accent-blue);
  }

  .branch-icon {
    font-size: 10px;
  }

  .branch-name {
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
