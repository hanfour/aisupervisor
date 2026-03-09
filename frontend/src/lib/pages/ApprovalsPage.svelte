<script>
  import { onMount, onDestroy } from 'svelte'
  import { pendingGates, loadPendingGates, respondToGate } from '../stores/approvals.js'
  import { addError } from '../stores/errors.js'
  import { t } from '../stores/i18n.js'

  let eventCleanup = null
  let prdContent = null
  let showingPRD = false

  onMount(() => {
    loadPendingGates()
    if (window.runtime) {
      eventCleanup = window.runtime.EventsOn('company:event', () => {
        loadPendingGates()
      })
    }
  })

  onDestroy(() => {
    if (eventCleanup) eventCleanup()
  })

  function timeSince(ts) {
    if (!ts) return '—'
    const seconds = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
    if (seconds < 60) return seconds + 's'
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return minutes + 'm'
    const hours = Math.floor(minutes / 60)
    return hours + 'h ' + (minutes % 60) + 'm'
  }

  async function handleRespond(id, status) {
    try {
      await respondToGate(id, status)
      if (showingPRD) {
        showingPRD = false
        prdContent = null
      }
    } catch (e) {
      addError('Failed to respond: ' + (e.message || e))
    }
  }

  async function viewPRD(gate) {
    try {
      prdContent = await window.go.gui.CompanyApp.GetPRDContentByTask(gate.taskId)
      showingPRD = true
    } catch (e) {
      addError('Failed to load PRD: ' + (e.message || e))
    }
  }
</script>

<div class="approvals-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('approvals.title')}</p>

    {#if $pendingGates.length === 0}
      <p class="empty-msg">{$t('approvals.empty')}</p>
    {:else}
      <div class="table-wrap">
        <table class="nes-table is-bordered is-dark">
          <thead>
            <tr>
              <th>{$t('approvals.reason')}</th>
              <th>{$t('approvals.taskWorker')}</th>
              <th>{$t('approvals.message')}</th>
              <th>{$t('approvals.waiting')}</th>
              <th>{$t('approvals.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {#each $pendingGates as gate}
              <tr>
                <td class="reason">{gate.reason || '—'}</td>
                <td class="ids">
                  {#if gate.taskId}<span class="mono">{gate.taskId}</span>{/if}
                  {#if gate.workerId}<br/><span class="mono">{gate.workerId}</span>{/if}
                </td>
                <td class="message">{gate.message || '—'}</td>
                <td class="waiting">{timeSince(gate.createdAt)}</td>
                <td class="actions">
                  {#if gate.reason === 'prd_approval'}
                    <button class="nes-btn is-primary btn-sm" on:click={() => viewPRD(gate)}>
                      {$t('prd.viewDocument')}
                    </button>
                  {/if}
                  <button class="nes-btn is-success btn-sm" on:click={() => handleRespond(gate.id, 'approved')}>
                    {$t('approvals.approve')}
                  </button>
                  <button class="nes-btn is-error btn-sm" on:click={() => handleRespond(gate.id, 'denied')}>
                    {$t('approvals.deny')}
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  {#if showingPRD && prdContent}
    <section class="nes-container with-title is-dark prd-preview">
      <p class="title">{$t('prd.viewDocument')}</p>
      <button class="nes-btn btn-sm close-btn" on:click={() => { showingPRD = false; prdContent = null }}>✕</button>
      <pre class="prd-content">{prdContent}</pre>
    </section>
  {/if}
</div>

<style>
  .approvals-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
    overflow-y: auto;
  }

  .table-wrap {
    overflow-x: auto;
    width: 100%;
  }

  table {
    width: 100%;
    font-size: 9px;
  }

  th, td {
    padding: 6px 8px !important;
  }

  th {
    color: var(--accent-blue);
    font-size: 9px;
    white-space: nowrap;
  }

  .reason {
    color: var(--accent-yellow, #f0c040);
    font-weight: bold;
  }

  .ids {
    font-size: 8px;
  }

  .mono {
    font-family: monospace;
  }

  .message {
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .waiting {
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .actions {
    white-space: nowrap;
  }

  .btn-sm {
    font-size: 7px !important;
    padding: 2px 6px !important;
    margin: 0 2px;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
    text-align: center;
    padding: 24px;
  }

  .prd-preview {
    position: relative;
    max-height: 400px;
    overflow-y: auto;
  }

  .prd-content {
    white-space: pre-wrap;
    word-wrap: break-word;
    font-size: 9px;
    line-height: 1.5;
    color: var(--text-primary, #fff);
  }

  .close-btn {
    position: absolute;
    top: 4px;
    right: 4px;
    font-size: 8px !important;
    padding: 2px 6px !important;
  }
</style>
