<script>
  export let config = {}

  $: polling = config?.polling || {}
  $: decision = config?.decision || {}
  $: context = config?.context || {}
</script>

<div class="settings-panel">
  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Polling</p>
    <div class="field">
      <label>Interval (ms):</label>
      <span class="value">{polling.interval_ms || 500}</span>
    </div>
    <div class="field">
      <label>Context Lines:</label>
      <span class="value">{polling.context_lines || 100}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Decision</p>
    <div class="field">
      <label>Confidence Threshold:</label>
      <span class="value">{decision.confidence_threshold || 0.7}</span>
    </div>
    <div class="field">
      <label>Timeout (s):</label>
      <span class="value">{decision.timeout_seconds || 30}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Context</p>
    <div class="field">
      <label>Enabled:</label>
      <span class="value">{context.enabled ? 'Yes' : 'No'}</span>
    </div>
    <div class="field">
      <label>Max Decisions:</label>
      <span class="value">{context.max_decisions || 20}</span>
    </div>
    <div class="field">
      <label>Token Budget:</label>
      <span class="value">{context.token_budget || 2000}</span>
    </div>
  </div>
</div>

<style>
  .settings-panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .setting-group {
    font-size: 10px;
  }

  .field {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
    border-bottom: 1px solid var(--border-color);
  }

  label {
    color: var(--text-secondary);
  }

  .value {
    color: var(--accent-green);
  }
</style>
