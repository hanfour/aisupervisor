<script>
  import { onMount, onDestroy } from 'svelte'

  export let workerId = ''
  export let workerName = ''
  export let onClose = () => {}

  let content = 'Loading...'
  let timer = null

  async function fetchContent() {
    if (!workerId || !window.go?.gui?.CompanyApp) return
    try {
      content = await window.go.gui.CompanyApp.GetPaneContent(workerId)
    } catch (e) {
      content = 'No active session: ' + e.message
    }
  }

  onMount(() => {
    fetchContent()
    timer = setInterval(fetchContent, 1500)
  })

  onDestroy(() => {
    if (timer) clearInterval(timer)
  })
</script>

<div class="overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
  <div class="nes-dialog is-dark is-rounded log-dialog" on:click|stopPropagation role="presentation">
    <div class="log-header">
      <p class="title">{workerName} - Live Log</p>
      <button class="nes-btn is-error btn-close" on:click={onClose}>X</button>
    </div>
    <pre class="log-content">{content}</pre>
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
    width: 80vw;
    height: 70vh;
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
</style>
