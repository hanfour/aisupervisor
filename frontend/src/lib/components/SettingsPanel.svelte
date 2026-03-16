<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'

  export let config = {}

  $: polling = config?.polling || {}
  $: decision = config?.decision || {}
  $: context = config?.context || {}

  let chatBackend = ''
  let availableChatBackends = []

  // Agentic training config
  let agenticEnabled = false
  let agenticMaxIter = 10
  let agenticTestCmd = ''
  let agenticAutoRollback = true

  onMount(async () => {
    if (window.go?.gui?.CompanyApp) {
      try {
        chatBackend = await window.go.gui.CompanyApp.GetChatBackend()
        availableChatBackends = await window.go.gui.CompanyApp.GetAvailableChatBackends()
      } catch {
        // ignore
      }
      try {
        const cfg = await window.go.gui.CompanyApp.GetAgenticLoopConfig()
        if (cfg) {
          agenticEnabled = cfg.enabled
          agenticMaxIter = cfg.maxIterations || 10
          agenticTestCmd = cfg.defaultTestCmd || ''
          agenticAutoRollback = cfg.autoRollback
        }
      } catch {
        // ignore
      }
    }
  })

  async function saveAgenticConfig() {
    if (window.go?.gui?.CompanyApp) {
      try {
        await window.go.gui.CompanyApp.SetAgenticLoopConfig({
          enabled: agenticEnabled,
          maxIterations: agenticMaxIter,
          defaultTestCmd: agenticTestCmd,
          autoRollback: agenticAutoRollback,
        })
      } catch (err) {
        console.error('Failed to save agentic config:', err)
      }
    }
  }

  async function onChatBackendChange(e) {
    const name = e.target.value
    if (window.go?.gui?.CompanyApp) {
      try {
        await window.go.gui.CompanyApp.SetChatBackend(name)
        chatBackend = name
      } catch (err) {
        console.error('Failed to set chat backend:', err)
      }
    }
  }
</script>

<div class="settings-panel">
  <div class="setting-group nes-container with-title is-dark">
    <p class="title">{$t('settings.chatBackend')}</p>
    <div class="field">
      <label for="chat-backend-select">{$t('settings.chatBackend')}:</label>
      {#if availableChatBackends.length > 0}
        <select id="chat-backend-select" class="nes-select is-dark chat-select" value={chatBackend} on:change={onChatBackendChange}>
          {#each availableChatBackends as backend}
            <option value={backend}>{backend}</option>
          {/each}
        </select>
      {:else}
        <span class="value">{chatBackend || 'N/A'}</span>
      {/if}
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Polling</p>
    <div class="field">
      <span class="field-label">Interval (ms):</span>
      <span class="value">{polling.interval_ms || 500}</span>
    </div>
    <div class="field">
      <span class="field-label">Context Lines:</span>
      <span class="value">{polling.context_lines || 100}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Decision</p>
    <div class="field">
      <span class="field-label">Confidence Threshold:</span>
      <span class="value">{decision.confidence_threshold || 0.7}</span>
    </div>
    <div class="field">
      <span class="field-label">Timeout (s):</span>
      <span class="value">{decision.timeout_seconds || 30}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">{$t('settings.agenticTraining')}</p>
    <div class="field">
      <label>
        <input type="checkbox" class="nes-checkbox is-dark" bind:checked={agenticEnabled} on:change={saveAgenticConfig} />
        <span>{$t('settings.agenticEnabled')}</span>
      </label>
    </div>
    {#if agenticEnabled}
      <div class="field">
        <span class="field-label">{$t('settings.maxIterations')}:</span>
        <input type="number" class="nes-input is-dark setting-input" bind:value={agenticMaxIter} min="1" max="100" on:change={saveAgenticConfig} />
      </div>
      <div class="field">
        <span class="field-label">{$t('settings.defaultTestCmd')}:</span>
        <input type="text" class="nes-input is-dark setting-input" bind:value={agenticTestCmd} placeholder="make test" on:change={saveAgenticConfig} />
      </div>
      <div class="field">
        <label>
          <input type="checkbox" class="nes-checkbox is-dark" bind:checked={agenticAutoRollback} on:change={saveAgenticConfig} />
          <span>{$t('settings.autoRollback')}</span>
        </label>
      </div>
    {/if}
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Context</p>
    <div class="field">
      <span class="field-label">Enabled:</span>
      <span class="value">{context.enabled ? 'Yes' : 'No'}</span>
    </div>
    <div class="field">
      <span class="field-label">Max Decisions:</span>
      <span class="value">{context.max_decisions || 20}</span>
    </div>
    <div class="field">
      <span class="field-label">Token Budget:</span>
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
    align-items: center;
    padding: 4px 0;
    border-bottom: 1px solid var(--border-color);
  }

  label, .field-label {
    color: var(--text-secondary);
  }

  .value {
    color: var(--accent-green);
  }

  .chat-select {
    font-size: 10px;
    max-width: 160px;
  }

  .setting-input {
    font-size: 10px;
    max-width: 140px;
    padding: 2px 4px;
  }
</style>
