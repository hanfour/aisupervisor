<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'

  let reports = []
  let projects = []
  let loading = false
  let retroRunning = false
  let selectedProjectId = ''
  let expandedReport = null

  onMount(async () => {
    await loadReports()
    await loadProjects()
  })

  async function loadReports() {
    try {
      reports = await window.go.gui.CompanyApp.GetRetroReports() || []
    } catch {
      reports = []
    }
  }

  async function loadProjects() {
    try {
      projects = await window.go.gui.CompanyApp.ListProjects() || []
    } catch {
      projects = []
    }
  }

  async function triggerRetro() {
    if (!selectedProjectId || retroRunning) return
    retroRunning = true
    try {
      await window.go.gui.CompanyApp.TriggerRetro(selectedProjectId)
      await loadReports()
    } catch (e) {
      console.error('retro failed:', e)
    } finally {
      retroRunning = false
    }
  }

  function toggleExpand(id) {
    expandedReport = expandedReport === id ? null : id
  }

  function formatDate(iso) {
    if (!iso) return ''
    return new Date(iso).toLocaleString()
  }
</script>

<div class="retro-panel">
  <!-- Manual trigger -->
  <div class="trigger-section">
    <select bind:value={selectedProjectId} class="nes-select is-dark">
      <option value="">-- {$t('nav.projects')} --</option>
      {#each projects as p}
        <option value={p.id}>{p.name}</option>
      {/each}
    </select>
    <button
      class="nes-btn is-primary"
      on:click={triggerRetro}
      disabled={!selectedProjectId || retroRunning}
    >
      {retroRunning ? $t('retro.running') : $t('retro.triggerRetro')}
    </button>
  </div>

  <!-- Reports list -->
  {#if reports.length === 0}
    <p class="empty">{$t('retro.noReports')}</p>
  {:else}
    {#each reports as report}
      <div class="report-card nes-container is-dark">
        <div
          class="report-header"
          on:click={() => toggleExpand(report.id)}
          on:keydown={(e) => e.key === 'Enter' && toggleExpand(report.id)}
          role="button"
          tabindex="0"
        >
          <span class="project-name">{report.projectName}</span>
          <span class="date">{$t('retro.appliedAt')}: {formatDate(report.appliedAt)}</span>
          <span class="toggle">{expandedReport === report.id ? '▼' : '▶'}</span>
        </div>

        {#if expandedReport === report.id}
          <div class="report-body">
            <!-- Summary -->
            <div class="section">
              <h4>{$t('retro.summary')}</h4>
              <p>{report.result.summary}</p>
            </div>

            <!-- Worker Feedback -->
            {#if report.result.workerFeedback?.length}
              <div class="section">
                <h4>{$t('retro.workerFeedback')}</h4>
                {#each report.result.workerFeedback as fb}
                  <div class="worker-feedback">
                    <span class="worker-id">{fb.workerId}</span>
                    {#if fb.strengths?.length}
                      <div class="fb-group">
                        <span class="fb-label good">{$t('retro.strengths')}:</span>
                        <ul>{#each fb.strengths as s}<li>{s}</li>{/each}</ul>
                      </div>
                    {/if}
                    {#if fb.weaknesses?.length}
                      <div class="fb-group">
                        <span class="fb-label warn">{$t('retro.weaknesses')}:</span>
                        <ul>{#each fb.weaknesses as w}<li>{w}</li>{/each}</ul>
                      </div>
                    {/if}
                    {#if fb.suggestions?.length}
                      <div class="fb-group">
                        <span class="fb-label">{$t('retro.suggestions')}:</span>
                        <ul>{#each fb.suggestions as s}<li>{s}</li>{/each}</ul>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}

            <!-- Skill Adjustments -->
            {#if report.result.skillAdjustments?.length}
              <div class="section">
                <h4>{$t('retro.skillAdjustments')}</h4>
                {#each report.result.skillAdjustments as adj}
                  <div class="adjustment">
                    <span class="worker-id">{adj.workerId}</span>
                    <span class="profile-id">({adj.profileId})</span>
                    {#if adj.promptAdditions?.length}
                      <div class="adj-detail">
                        <span class="adj-label">{$t('retro.promptAdditions')}:</span>
                        {#each adj.promptAdditions as p}<span class="tag add">+{p}</span>{/each}
                      </div>
                    {/if}
                    {#if adj.addTools?.length}
                      <div class="adj-detail">
                        <span class="adj-label">{$t('retro.addTools')}:</span>
                        {#each adj.addTools as tool}<span class="tag add">+{tool}</span>{/each}
                      </div>
                    {/if}
                    {#if adj.removeTools?.length}
                      <div class="adj-detail">
                        <span class="adj-label">{$t('retro.removeTools')}:</span>
                        {#each adj.removeTools as tool}<span class="tag remove">-{tool}</span>{/each}
                      </div>
                    {/if}
                    {#if adj.modelChange}
                      <div class="adj-detail">
                        <span class="adj-label">{$t('retro.modelChange')}:</span>
                        <span class="tag">{adj.modelChange}</span>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<style>
  .retro-panel {
    width: 100%;
  }

  .trigger-section {
    display: flex;
    gap: 12px;
    align-items: center;
    margin-bottom: 16px;
    flex-wrap: wrap;
  }

  .trigger-section select {
    flex: 1;
    min-width: 200px;
    font-size: 10px;
  }

  .trigger-section button {
    font-size: 10px !important;
    white-space: nowrap;
  }

  .empty {
    color: var(--text-secondary);
    font-size: 11px;
  }

  .report-card {
    margin-bottom: 12px;
    padding: 8px !important;
  }

  .report-header {
    display: flex;
    align-items: center;
    gap: 12px;
    cursor: pointer;
    font-size: 11px;
  }

  .project-name {
    font-weight: bold;
    color: var(--accent-blue);
  }

  .date {
    color: var(--text-secondary);
    font-size: 9px;
    margin-left: auto;
  }

  .toggle {
    font-size: 10px;
    color: var(--text-secondary);
  }

  .report-body {
    margin-top: 12px;
    font-size: 10px;
  }

  .section {
    margin-bottom: 12px;
  }

  .section h4 {
    color: var(--accent-green);
    font-size: 11px;
    margin-bottom: 6px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 4px;
  }

  .section p {
    margin: 0;
    line-height: 1.5;
  }

  .worker-feedback, .adjustment {
    border-left: 3px solid var(--border-color);
    padding-left: 8px;
    margin-bottom: 8px;
  }

  .worker-id {
    font-weight: bold;
    color: var(--accent-blue);
    font-size: 10px;
  }

  .profile-id {
    color: var(--text-secondary);
    font-size: 9px;
  }

  .fb-group {
    margin-top: 4px;
  }

  .fb-label {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .fb-label.good {
    color: var(--accent-green);
  }

  .fb-label.warn {
    color: var(--accent-red, #ff3860);
  }

  .fb-group ul {
    margin: 2px 0 0 16px;
    padding: 0;
  }

  .fb-group li {
    margin-bottom: 2px;
    list-style: disc;
  }

  .adj-detail {
    margin-top: 4px;
  }

  .adj-label {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .tag {
    display: inline-block;
    padding: 1px 6px;
    margin: 2px 4px 2px 0;
    font-size: 9px;
    border: 1px solid var(--border-color);
  }

  .tag.add {
    border-color: var(--accent-green);
    color: var(--accent-green);
  }

  .tag.remove {
    border-color: var(--accent-red, #ff3860);
    color: var(--accent-red, #ff3860);
  }
</style>
