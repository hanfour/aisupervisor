<script>
  import { onMount, onDestroy, tick } from 'svelte'

  export let workerId = ''
  export let workerName = ''
  export let onClose = () => {}

  let content = 'Loading...'
  let searchQuery = ''
  let autoScroll = true
  let scrollbackLines = 100
  let timer = null
  let logEl = null

  $: filteredContent = filterContent(content, searchQuery)
  $: scrollbackOptions = [100, 300, 500, 1000]

  function filterContent(text, query) {
    if (!query) return text
    const lines = text.split('\n')
    const lower = query.toLowerCase()
    const matched = lines.filter(line => line.toLowerCase().includes(lower))
    if (matched.length === 0) return '(no matches for "' + query + '")'
    return matched.join('\n')
  }

  async function fetchContent() {
    if (!workerId || !window.go?.gui?.CompanyApp) return
    try {
      content = await window.go.gui.CompanyApp.GetPaneContentLines(workerId, scrollbackLines)
    } catch (e) {
      content = 'No active session: ' + e.message
    }
    if (autoScroll) {
      await tick()
      scrollToBottom()
    }
  }

  function scrollToBottom() {
    if (logEl) {
      logEl.scrollTop = logEl.scrollHeight
    }
  }

  function handleScroll() {
    if (!logEl) return
    const atBottom = logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight < 30
    autoScroll = atBottom
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') onClose()
  }

  onMount(() => {
    fetchContent()
    timer = setInterval(fetchContent, 1500)
  })

  onDestroy(() => {
    if (timer) clearInterval(timer)
  })
</script>

<div class="overlay" on:click={onClose} on:keydown={handleKeydown} role="presentation">
  <div class="nes-dialog is-dark is-rounded log-dialog" on:click|stopPropagation role="presentation">
    <div class="log-header">
      <p class="title">{workerName} - Live Log</p>
      <button class="nes-btn is-error btn-close" on:click={onClose}>X</button>
    </div>

    <div class="log-toolbar">
      <div class="search-box">
        <input
          type="text"
          class="nes-input is-dark"
          placeholder="Search..."
          bind:value={searchQuery}
        />
      </div>
      <div class="toolbar-controls">
        <label class="scroll-label">
          <input type="checkbox" class="nes-checkbox is-dark" bind:checked={autoScroll} />
          <span>Auto-scroll</span>
        </label>
        <div class="nes-select is-dark scrollback-select">
          <select bind:value={scrollbackLines} on:change={fetchContent}>
            {#each scrollbackOptions as opt}
              <option value={opt}>{opt} lines</option>
            {/each}
          </select>
        </div>
      </div>
    </div>

    <pre
      class="log-content"
      bind:this={logEl}
      on:scroll={handleScroll}
    >{filteredContent}</pre>

    {#if searchQuery}
      <div class="search-status">
        {filteredContent === '(no matches for "' + searchQuery + '")' ? 'No matches' : 'Filtered'}
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0,0,0,0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }

  .log-dialog {
    width: 85vw;
    height: 80vh;
    padding: 16px !important;
    display: flex;
    flex-direction: column;
  }

  .log-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .log-header .title {
    margin: 0;
    font-size: 12px;
  }

  .btn-close {
    font-size: 8px;
    padding: 4px 8px !important;
  }

  .log-toolbar {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
    flex-wrap: wrap;
  }

  .search-box {
    flex: 1;
    min-width: 150px;
  }

  .search-box input {
    font-size: 9px;
    padding: 4px 8px !important;
    width: 100%;
  }

  .toolbar-controls {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .scroll-label {
    font-size: 9px;
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  .scroll-label input {
    margin: 0 !important;
  }

  .scrollback-select {
    min-width: 90px;
  }

  .scrollback-select select {
    font-size: 9px;
    padding: 2px 4px !important;
  }

  .log-content {
    flex: 1;
    overflow: auto;
    background: #111;
    color: #0f0;
    font-family: monospace;
    font-size: 10px;
    padding: 8px;
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    border: 2px solid #333;
  }

  .search-status {
    font-size: 9px;
    color: var(--text-secondary);
    margin-top: 4px;
    text-align: right;
  }
</style>
