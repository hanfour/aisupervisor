<script>
  import { chatOpen, chatWorkerName, chatWorkerAvatar, chatMessages, chatLoading, closeChat, sendMessage } from '../stores/workerChat.js'
  import { t } from '../stores/i18n.js'

  let inputText = ''
  let messagesContainer

  $: if ($chatMessages && messagesContainer) {
    // Scroll to bottom on new messages
    setTimeout(() => {
      if (messagesContainer) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight
      }
    }, 0)
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
    if (e.key === 'Escape') {
      closeChat()
    }
  }

  async function handleSend() {
    if (!inputText.trim() || $chatLoading) return
    const text = inputText
    inputText = ''
    await sendMessage(text)
  }

  function avatarClass(avatar) {
    const map = {
      robot: 'nes-octocat',
      cat: 'nes-octocat',
      kirby: 'nes-kirby',
      mario: 'nes-mario',
      ash: 'nes-ash',
      bulbasaur: 'nes-bulbasaur',
      charmander: 'nes-charmander',
      squirtle: 'nes-squirtle',
      pokeball: 'nes-pokeball',
    }
    return map[avatar] || 'nes-octocat'
  }
</script>

{#if $chatOpen}
  <div class="drawer-overlay" on:click={closeChat} on:keydown={(e) => e.key === 'Escape' && closeChat()} role="presentation">
    <div class="drawer nes-container is-dark is-rounded" on:click|stopPropagation role="presentation">
      <!-- Header -->
      <div class="drawer-header">
        <div class="worker-info">
          <i class={avatarClass($chatWorkerAvatar)} style="transform: scale(0.6); transform-origin: left center;"></i>
          <span class="worker-name">{$chatWorkerName}</span>
        </div>
        <button class="nes-btn btn-sm" on:click={closeChat}>X</button>
      </div>

      <!-- Messages -->
      <div class="messages" bind:this={messagesContainer}>
        {#if $chatMessages.length === 0}
          <div class="empty-msg">{$t('chat.startConversation')} {$chatWorkerName} {$t('chat.startConversationSuffix')}</div>
        {/if}
        {#each $chatMessages as msg}
          <div class="msg" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
            {#if msg.role === 'assistant'}
              <i class={avatarClass($chatWorkerAvatar)} style="transform: scale(0.4); transform-origin: left top;"></i>
            {/if}
            <div class="bubble" class:user-bubble={msg.role === 'user'} class:worker-bubble={msg.role === 'assistant'}>
              {msg.content}
            </div>
          </div>
        {/each}
        {#if $chatLoading}
          <div class="msg assistant">
            <div class="bubble worker-bubble loading">...</div>
          </div>
        {/if}
      </div>

      <!-- Input -->
      <div class="input-area">
        <input
          type="text"
          class="nes-input is-dark"
          placeholder={$t('chat.placeholder')}
          bind:value={inputText}
          on:keydown={handleKeydown}
          disabled={$chatLoading}
        />
        <button class="nes-btn is-primary btn-sm" on:click={handleSend} disabled={$chatLoading || !inputText.trim()}>
          {$t('aiChat.send')}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .drawer-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 200;
    display: flex;
    justify-content: flex-end;
  }

  .drawer {
    width: 380px;
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: 12px !important;
    animation: slide-in 0.2s ease-out;
  }

  @keyframes slide-in {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: 8px;
    border-bottom: 2px solid var(--border-color);
    margin-bottom: 8px;
  }

  .worker-info {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .worker-name {
    font-size: 11px;
    color: var(--accent-green);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 8px 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .empty-msg {
    text-align: center;
    color: var(--text-secondary);
    font-size: 9px;
    margin-top: 24px;
  }

  .msg {
    display: flex;
    gap: 4px;
    align-items: flex-start;
  }

  .msg.user {
    justify-content: flex-end;
  }

  .msg.assistant {
    justify-content: flex-start;
  }

  .bubble {
    max-width: 75%;
    padding: 8px 10px;
    font-size: 9px;
    line-height: 1.4;
    word-break: break-word;
    white-space: pre-wrap;
  }

  .user-bubble {
    background: var(--bg-secondary);
    border: 2px solid var(--border-color);
    color: var(--text-primary);
  }

  .worker-bubble {
    background: rgba(0, 200, 100, 0.08);
    border: 2px solid var(--accent-green);
    color: var(--text-primary);
  }

  .loading {
    opacity: 0.6;
    animation: pulse 1s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.6; }
    50% { opacity: 1; }
  }

  .input-area {
    display: flex;
    gap: 6px;
    padding-top: 8px;
    border-top: 2px solid var(--border-color);
  }

  .input-area input {
    flex: 1;
    font-size: 10px;
  }

  .btn-sm {
    font-size: 9px !important;
    padding: 4px 8px !important;
  }
</style>
