<script>
  import { t } from '../stores/i18n.js'

  export let report = null
  export let workerName = ''
  export let onClose = () => {}

  let showRawContent = false
</script>

{#if report}
  <div class="report-overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
    <div class="report-card nes-container is-dark is-rounded" on:click|stopPropagation role="presentation">
      <div class="report-header">
        <span class="report-title">{$t('report.title')}</span>
        <button class="nes-btn btn-sm" on:click={onClose}>X</button>
      </div>

      {#if workerName}
        <div class="worker-line">
          <span class="label">{$t('report.researcher')}</span> {workerName}
        </div>
      {/if}

      {#if report.createdAt}
        <div class="meta-line">
          <span class="label">{$t('report.date')}</span> {new Date(report.createdAt).toLocaleString()}
        </div>
      {/if}

      <!-- Summary -->
      {#if report.summary}
        <div class="section">
          <h4 class="section-title">{$t('report.summary')}</h4>
          <p class="section-body">{report.summary}</p>
        </div>
      {/if}

      <!-- Key Findings -->
      {#if report.keyFindings && report.keyFindings.length > 0}
        <div class="section">
          <h4 class="section-title">{$t('report.keyFindings')}</h4>
          <ul class="nes-list is-disc">
            {#each report.keyFindings as finding}
              <li>{finding}</li>
            {/each}
          </ul>
        </div>
      {/if}

      <!-- Recommendations -->
      {#if report.recommendations && report.recommendations.length > 0}
        <div class="section">
          <h4 class="section-title">{$t('report.recommendations')}</h4>
          <ul class="nes-list is-circle">
            {#each report.recommendations as rec}
              <li>{rec}</li>
            {/each}
          </ul>
        </div>
      {/if}

      <!-- References -->
      {#if report.references && report.references.length > 0}
        <div class="section">
          <h4 class="section-title">{$t('report.references')}</h4>
          <ul class="ref-list">
            {#each report.references as ref}
              <li>
                {#if ref.startsWith('http')}
                  <a href={ref} target="_blank" rel="noopener">{ref}</a>
                {:else}
                  {ref}
                {/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}

      <!-- Raw Content (collapsible) -->
      {#if report.rawContent}
        <div class="section">
          <button class="nes-btn btn-sm toggle-raw" on:click={() => showRawContent = !showRawContent}>
            {showRawContent ? $t('report.hideFull') : $t('report.showFull')}
          </button>
          {#if showRawContent}
            <pre class="raw-content">{report.rawContent}</pre>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .report-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 150;
  }

  .report-card {
    width: 600px;
    max-height: 80vh;
    overflow-y: auto;
    padding: 20px !important;
  }

  .report-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .report-title {
    font-size: 13px;
    color: #f0c040;
  }

  .worker-line, .meta-line {
    font-size: 9px;
    margin-bottom: 4px;
  }

  .label {
    color: var(--text-secondary);
  }

  .section {
    margin-top: 14px;
  }

  .section-title {
    font-size: 10px;
    color: var(--accent-blue);
    margin-bottom: 6px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 3px;
  }

  .section-body {
    font-size: 9px;
    line-height: 1.5;
  }

  ul {
    font-size: 9px;
    padding-left: 16px;
  }

  li {
    margin: 3px 0;
    line-height: 1.4;
  }

  .ref-list {
    list-style: none;
    padding-left: 0;
  }

  .ref-list li {
    font-size: 8px;
  }

  .ref-list a {
    color: var(--accent-blue);
    word-break: break-all;
  }

  .toggle-raw {
    font-size: 8px !important;
    padding: 2px 8px !important;
  }

  .raw-content {
    font-size: 8px;
    background: var(--bg-secondary);
    padding: 10px;
    margin-top: 8px;
    overflow-x: auto;
    max-height: 300px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-word;
    border: 1px solid var(--border-color);
  }

  .btn-sm {
    font-size: 9px !important;
    padding: 4px 8px !important;
  }
</style>
