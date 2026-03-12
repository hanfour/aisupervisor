<script>
  import { onMount } from 'svelte'
  import SettingsPanel from '../components/SettingsPanel.svelte'
  import { t, setLanguage as setI18nLanguage } from '../stores/i18n.js'
  import { loadSessions } from '../stores/sessions.js'

  let config = {}
  let language = 'zh-TW'
  let clearStatus = '' // '', 'confirming', 'force-confirming'
  let activeCount = 0
  let clearMessage = ''
  let healthResults = null
  let healthLoading = false
  let appVersion = ''
  let updateStatus = '' // '', 'checking', 'up-to-date', 'available'
  let updateInfo = null
  let skillsmpKey = ''
  let skillsmpSaveMsg = ''

  onMount(async () => {
    if (window.go?.gui?.App) {
      config = (await window.go.gui.App.GetConfig()) || {}
    }
    if (window.go?.gui?.CompanyApp) {
      language = (await window.go.gui.CompanyApp.GetLanguage()) || 'zh-TW'
      appVersion = (await window.go.gui.CompanyApp.GetVersion()) || 'dev'
      skillsmpKey = (await window.go.gui.CompanyApp.GetSkillsMPAPIKey()) || ''
    }
  })

  async function checkForUpdates() {
    updateStatus = 'checking'
    updateInfo = null
    try {
      if (window.go?.gui?.CompanyApp?.CheckForUpdates) {
        const info = await window.go.gui.CompanyApp.CheckForUpdates()
        if (info && info.version) {
          updateInfo = info
          updateStatus = 'available'
        } else {
          updateStatus = 'up-to-date'
        }
      } else {
        updateStatus = 'up-to-date'
      }
    } catch (e) {
      updateStatus = 'up-to-date'
    }
  }

  async function downloadUpdate() {
    if (updateInfo?.download_url) {
      if (window.go?.gui?.CompanyApp?.DownloadUpdate) {
        await window.go.gui.CompanyApp.DownloadUpdate(updateInfo.download_url)
      }
    }
  }

  async function handleLanguageChange() {
    await setI18nLanguage(language)
  }

  async function handleClearAll() {
    clearMessage = ''
    if (!window.go?.gui?.CompanyApp) return
    activeCount = await window.go.gui.CompanyApp.ActiveWorkerCount()
    clearStatus = activeCount > 0 ? 'force-confirming' : 'confirming'
  }

  async function confirmClear(force) {
    try {
      await window.go.gui.CompanyApp.ClearAllProjects(force)
      if (window.go?.gui?.App?.ClearSessions) {
        await window.go.gui.App.ClearSessions()
      }
      await loadSessions()
      clearMessage = $t('settings.clearSuccess')
      clearStatus = ''
    } catch (e) {
      clearMessage = 'Error: ' + (e.message || e)
      clearStatus = ''
    }
  }

  function cancelClear() {
    clearStatus = ''
    clearMessage = ''
  }

  async function handleHealthCheck() {
    healthLoading = true
    healthResults = null
    try {
      if (window.go?.gui?.CompanyApp?.RunHealthCheck) {
        healthResults = await window.go.gui.CompanyApp.RunHealthCheck()
      } else {
        healthResults = [$t('settings.healthOk')]
      }
    } catch (e) {
      healthResults = ['Error: ' + (e.message || e)]
    }
    healthLoading = false
  }
</script>

<div class="settings-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.version')}</p>
    <div class="version-row">
      <span class="version-label">AI Supervisor v{appVersion}</span>
      <div class="version-actions">
        {#if updateStatus === 'checking'}
          <span class="update-status">{$t('settings.checking')}</span>
        {:else if updateStatus === 'up-to-date'}
          <span class="update-status ok">{$t('settings.upToDate')}</span>
        {:else if updateStatus === 'available' && updateInfo}
          <span class="update-status new">{$t('settings.updateAvailable')}: v{updateInfo.version}</span>
          <button class="nes-btn is-success btn-sm" on:click={downloadUpdate}>{$t('settings.download')}</button>
        {/if}
        <button class="nes-btn is-primary btn-sm" on:click={checkForUpdates} disabled={updateStatus === 'checking'}>
          {$t('settings.checkUpdates')}
        </button>
      </div>
    </div>
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.language')}</p>
    <div class="lang-select">
      <label>
        <input type="radio" class="nes-radio is-dark" name="lang" value="en"
          bind:group={language} on:change={handleLanguageChange} />
        <span>English</span>
      </label>
      <label>
        <input type="radio" class="nes-radio is-dark" name="lang" value="zh-TW"
          bind:group={language} on:change={handleLanguageChange} />
        <span>繁體中文</span>
      </label>
    </div>
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.skillsmpKey')}</p>
    <div class="skillsmp-row">
      <input type="password" class="nes-input is-dark skillsmp-input" bind:value={skillsmpKey} placeholder="sk-..." />
      <button class="nes-btn is-primary btn-sm" on:click={async () => {
        try {
          await window.go.gui.CompanyApp.SetSkillsMPAPIKey(skillsmpKey)
          skillsmpSaveMsg = $t('settings.skillsmpKeySaved')
          setTimeout(() => skillsmpSaveMsg = '', 3000)
        } catch (e) {
          skillsmpSaveMsg = 'Error: ' + (e.message || e)
        }
      }}>{$t('common.save')}</button>
    </div>
    <p class="hint-text">{$t('settings.skillsmpKeyHint')}</p>
    {#if skillsmpSaveMsg}
      <p class="save-msg" class:error={skillsmpSaveMsg.startsWith('Error')}>{skillsmpSaveMsg}</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.configuration')}</p>
    <SettingsPanel {config} />
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.backends')}</p>
    {#if config.backends && config.backends.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>{$t('settings.nameCol')}</th>
            <th>{$t('settings.typeCol')}</th>
            <th>{$t('settings.modelCol')}</th>
          </tr>
        </thead>
        <tbody>
          {#each config.backends as backend}
            <tr>
              <td>{backend.name}</td>
              <td>{backend.type}</td>
              <td>{backend.model || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="empty">{$t('settings.noBackends')}</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.autoApprove')}</p>
    {#if config.auto_approve_rules && config.auto_approve_rules.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>{$t('settings.labelCol')}</th>
            <th>{$t('settings.patternCol')}</th>
            <th>{$t('settings.responseCol')}</th>
          </tr>
        </thead>
        <tbody>
          {#each config.auto_approve_rules as rule}
            <tr>
              <td>{rule.label}</td>
              <td>{rule.pattern_contains}</td>
              <td>{rule.response}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="empty">{$t('settings.noAutoApprove')}</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.healthCheck')}</p>
    <div class="danger-row">
      <div class="danger-info">
        <p class="danger-desc">{$t('settings.healthCheck')}</p>
      </div>
      <button class="nes-btn is-primary btn-danger" on:click={handleHealthCheck} disabled={healthLoading}>
        {healthLoading ? '...' : $t('settings.runHealthCheck')}
      </button>
    </div>
    {#if healthResults}
      <div class="health-results">
        {#each healthResults as item}
          <p class="health-item" class:error={item.startsWith('Error')}>{item}</p>
        {/each}
        {#if healthResults.length === 0}
          <p class="health-ok">{$t('settings.healthOk')}</p>
        {/if}
      </div>
    {/if}
  </section>

  <section class="nes-container with-title is-dark danger-zone">
    <p class="title">{$t('settings.dangerZone')}</p>
    <div class="danger-row">
      <div class="danger-info">
        <strong>{$t('settings.clearAllProjects')}</strong>
        <p class="danger-desc">{$t('settings.clearAllProjectsDesc')}</p>
      </div>
      {#if clearStatus === ''}
        <button class="nes-btn is-error btn-danger" on:click={handleClearAll}>
          {$t('settings.clearAllProjects')}
        </button>
      {:else if clearStatus === 'confirming'}
        <div class="confirm-group">
          <p class="confirm-msg">{$t('settings.clearConfirm')}</p>
          <div class="confirm-actions">
            <button class="nes-btn is-error" on:click={() => confirmClear(false)}>{$t('common.confirm')}</button>
            <button class="nes-btn" on:click={cancelClear}>{$t('common.cancel')}</button>
          </div>
        </div>
      {:else if clearStatus === 'force-confirming'}
        <div class="confirm-group">
          <p class="confirm-msg warning-text">
            {$t('settings.clearForceConfirm').replace('{count}', activeCount)}
          </p>
          <div class="confirm-actions">
            <button class="nes-btn is-error" on:click={() => confirmClear(true)}>{$t('common.confirm')}</button>
            <button class="nes-btn" on:click={cancelClear}>{$t('common.cancel')}</button>
          </div>
        </div>
      {/if}
    </div>
    {#if clearMessage}
      <p class="clear-msg" class:error={clearMessage.startsWith('Error')}>{clearMessage}</p>
    {/if}
  </section>
</div>

<style>
  .settings-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow-y: auto;
    height: 100%;
  }

  table {
    width: 100%;
    font-size: 10px;
  }

  th, td {
    padding: 6px 8px !important;
  }

  .empty {
    color: var(--text-secondary);
    font-size: 10px;
  }

  .lang-select {
    display: flex;
    gap: 24px;
  }

  .lang-select label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 12px;
  }

  .danger-zone {
    border-color: #e74c3c !important;
  }

  .danger-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
  }

  .danger-info {
    flex: 1;
    font-size: 10px;
  }

  .danger-desc {
    color: var(--text-secondary);
    font-size: 9px;
    margin-top: 4px;
  }

  .btn-danger {
    white-space: nowrap;
    font-size: 9px !important;
  }

  .confirm-group {
    text-align: right;
  }

  .confirm-msg {
    font-size: 9px;
    margin-bottom: 8px;
  }

  .warning-text {
    color: #e74c3c;
    font-weight: bold;
  }

  .confirm-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .confirm-actions button {
    font-size: 9px !important;
  }

  .clear-msg {
    font-size: 9px;
    margin-top: 8px;
    color: #2ecc71;
  }

  .clear-msg.error {
    color: #e74c3c;
  }

  .health-results {
    margin-top: 8px;
    padding: 8px;
    border: 2px solid var(--border-color);
  }

  .health-item {
    font-size: 9px;
    margin: 2px 0;
    color: var(--text-primary);
  }

  .health-item.error {
    color: #e74c3c;
  }

  .health-ok {
    font-size: 9px;
    color: #2ecc71;
  }

  .version-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }

  .version-label {
    font-size: 12px;
    font-weight: bold;
  }

  .version-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .update-status {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .update-status.ok {
    color: #2ecc71;
  }

  .update-status.new {
    color: #f39c12;
  }

  .btn-sm {
    font-size: 9px !important;
    padding: 4px 8px !important;
  }

  .skillsmp-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .skillsmp-input {
    flex: 1;
    font-size: 10px !important;
  }

  .hint-text {
    font-size: 8px;
    color: var(--text-secondary, #888);
    margin-top: 4px;
  }

  .save-msg {
    font-size: 9px;
    margin-top: 4px;
    color: #2ecc71;
  }

  .save-msg.error {
    color: #e74c3c;
  }
</style>
