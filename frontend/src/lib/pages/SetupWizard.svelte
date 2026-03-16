<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { t, language, setLanguage } from '../stores/i18n.js'
  import { skillProfiles, loadSkillProfiles } from '../stores/workers.js'
  import InstallerAnimation from '../components/InstallerAnimation.svelte'
  import CharacterPortrait from '../components/CharacterPortrait.svelte'

  export let onComplete = () => {}

  let step = 1
  let selectedLang = 'zh-TW'
  let deps = []          // old-style missing dep names (kept for compat)
  let depStatuses = []   // new detailed DepStatus[]
  let checking = false
  let applying = false
  let createdWorkers = []
  let installingDep = null   // currently installing dep name
  let installProgress = {}   // { dep: InstallProgress }
  let installError = null
  let unsubProgress = null

  // --- Step 3: Onboarding Chat ---
  let chatMessages = []        // { role, content, sender? }
  let chatInput = ''
  let chatLoading = false
  let chatError = null
  let hrHired = false
  let hrName = ''
  let recommendedWorkers = []  // OnboardingWorkerDTO[]
  let teamReady = false
  let chatScrollEl = null

  // API key setup (when no AI backend available)
  let needApiKey = false
  let apiKeyProvider = 'anthropic'  // 'openai' | 'anthropic' | 'gemini'
  let apiKeyInput = ''
  let apiKeySaving = false
  let apiKeyError = null

  // Fake worker objects for CharacterPortrait
  const assistantWorker = { id: 'assistant', name: 'Assistant', gender: 'female', status: 'working', tier: 'consultant', skillProfile: 'analyst' }
  let hrWorker = null

  const depDescKeys = {
    git: 'setup.depGitDesc',
    brew: 'setup.depBrewDesc',
    tmux: 'setup.depTmuxDesc',
    node: 'setup.depNodeDesc',
    claude: 'setup.depClaudeDesc',
  }

  async function checkDepStatuses() {
    checking = true
    try {
      depStatuses = await window.go.gui.CompanyApp.GetDependencyStatus()
      deps = depStatuses.filter(d => !d.installed).map(d => d.name)
    } catch (e) {
      try {
        deps = await window.go.gui.CompanyApp.CheckDependencies()
        depStatuses = []
      } catch (e2) {
        deps = ['error: ' + e2.message]
        depStatuses = []
      }
    }
    checking = false
  }

  function nextStep() {
    step++
    if (step === 2) {
      checkDepStatuses()
      setupProgressListener()
    }
    if (step === 3) {
      startOnboardingChat()
    }
  }

  function prevStep() {
    if (step > 1) step--
  }

  function setupProgressListener() {
    if (unsubProgress) return
    if (window.runtime?.EventsOn) {
      unsubProgress = window.runtime.EventsOn('setup:progress', (progress) => {
        installProgress = { ...installProgress, [progress.dep]: progress }
        if (progress.phase === 'done' || progress.phase === 'error') {
          if (progress.phase === 'done') {
            setTimeout(() => checkDepStatuses(), 500)
          }
          if (progress.phase === 'error') {
            installError = progress.message
          }
          if (installingDep === progress.dep) {
            installingDep = null
          }
        }
      })
    }
  }

  onDestroy(() => {
    if (unsubProgress && window.runtime?.EventsOff) {
      window.runtime.EventsOff('setup:progress')
      unsubProgress = null
    }
  })

  async function installSingle(depName) {
    installError = null
    installingDep = depName
    try {
      await window.go.gui.CompanyApp.InstallDependency(depName)
    } catch (e) {
      installError = e.message
      installingDep = null
    }
  }

  async function installAll() {
    installError = null
    installingDep = '__all__'
    try {
      await window.go.gui.CompanyApp.InstallAllDependencies()
    } catch (e) {
      installError = e.message
    }
    installingDep = null
    await checkDepStatuses()
  }

  // --- Onboarding Chat Logic ---

  async function scrollChat() {
    await tick()
    if (chatScrollEl) {
      chatScrollEl.scrollTop = chatScrollEl.scrollHeight
    }
  }

  async function startOnboardingChat() {
    chatMessages = []
    hrHired = false
    hrName = ''
    hrWorker = null
    recommendedWorkers = []
    teamReady = false
    chatError = null
    // Send empty user message to trigger assistant greeting
    await sendOnboardingMessage('')
  }

  async function sendOnboardingMessage(userText) {
    chatError = null
    chatLoading = true

    // Build messages for backend (only role + content, skip display-only messages)
    const backendMessages = []
    for (const m of chatMessages) {
      if (m.role === 'user' || m.role === 'assistant') {
        backendMessages.push({ role: m.role, content: m.content })
      }
    }
    if (userText) {
      backendMessages.push({ role: 'user', content: userText })
      chatMessages = [...chatMessages, { role: 'user', content: userText, sender: 'user' }]
    }

    await scrollChat()

    try {
      const resp = await window.go.gui.CompanyApp.ChatOnboarding(backendMessages)

      if (resp.status === 'need_api_key') {
        needApiKey = true
        chatMessages = [...chatMessages, { role: 'assistant', content: resp.message, sender: 'assistant' }]
        chatLoading = false
        await scrollChat()
        return
      } else if (resp.status === 'hire_hr' && !hrHired) {
        hrName = resp.hrName || 'HR'
        chatMessages = [...chatMessages, { role: 'assistant', content: resp.message, sender: 'assistant' }]
        await scrollChat()
        // Show HR hiring animation
        chatMessages = [...chatMessages, { role: 'system', content: $t('setup.hrHired').replace('{name}', hrName), sender: 'system' }]
        hrHired = true
        hrWorker = { id: 'hr', name: hrName, gender: 'female', status: 'idle', tier: 'engineer', skillProfile: 'reviewer' }
        // Inject a context message so the LLM knows HR has joined and should speak as HR
        chatMessages = [...chatMessages, { role: 'user', content: `${hrName} 已加入團隊。請以 ${hrName}（HR）的身份繼續對話，了解我的需求後推薦團隊。`, sender: 'user' }]
        await scrollChat()
        // Continue the conversation automatically — HR introduces themselves
        chatLoading = false
        await sendOnboardingMessage('')
        return
      } else if (resp.status === 'hire_hr' && hrHired) {
        // HR already hired — treat as normal chat message from HR
        chatMessages = [...chatMessages, { role: 'assistant', content: resp.message, sender: 'hr' }]
      } else if (resp.status === 'ready') {
        chatMessages = [...chatMessages, { role: 'assistant', content: resp.message, sender: hrHired ? 'hr' : 'assistant' }]
        recommendedWorkers = resp.workers || []
        teamReady = true
      } else {
        // chatting
        chatMessages = [...chatMessages, { role: 'assistant', content: resp.message, sender: hrHired ? 'hr' : 'assistant' }]
      }
    } catch (e) {
      chatError = $t('setup.chatError')
    }

    chatLoading = false
    await scrollChat()
  }

  function handleChatSubmit() {
    if (!chatInput.trim() || chatLoading) return
    const text = chatInput.trim()
    chatInput = ''
    sendOnboardingMessage(text)
  }

  let composing = false
  function handleChatKeydown(e) {
    if (e.key === 'Enter') {
      if (e.metaKey || e.ctrlKey) {
        // Cmd+Enter / Ctrl+Enter always submits
        e.preventDefault()
        handleChatSubmit()
      } else if (!e.shiftKey && !composing && !e.isComposing && e.keyCode !== 229) {
        // Plain Enter submits only when not composing
        e.preventDefault()
        handleChatSubmit()
      } else if (!e.shiftKey) {
        // Prevent newline during IME
        e.preventDefault()
      }
    }
  }

  async function submitApiKey() {
    if (!apiKeyInput.trim()) return
    apiKeySaving = true
    apiKeyError = null
    try {
      await window.go.gui.CompanyApp.SetupChatBackendFromKey(apiKeyProvider, apiKeyInput.trim())
      needApiKey = false
      apiKeyInput = ''
      // Retry the onboarding chat
      chatMessages = []
      await sendOnboardingMessage('')
    } catch (e) {
      apiKeyError = e.message || $t('setup.chatError')
    }
    apiKeySaving = false
  }

  async function confirmTeam() {
    applying = true
    try {
      await setLanguage(selectedLang)
      await window.go.gui.CompanyApp.ApplyOnboarding({
        companyName: '',
        language: selectedLang,
        teamTemplate: 'custom',
        apiKeySource: ''
      })
      const result = await window.go.gui.CompanyApp.BatchCreateWorkers(recommendedWorkers)
      createdWorkers = result || []
      step = 4
    } catch (e) {
      alert('Setup failed: ' + e.message)
    }
    applying = false
  }

  function getDepProgress(depName) {
    return installProgress[depName] || null
  }

  function isInstalling(depName) {
    if (installingDep === '__all__') return true
    return installingDep === depName
  }

  function canInstallDep(dep) {
    if (!dep.canAutoInstall) return false
    if (installingDep) return false
    if (dep.name === 'claude') {
      const nodeDep = depStatuses.find(d => d.name === 'node')
      if (nodeDep && !nodeDep.installed) return false
    }
    return true
  }

  $: allDepsOk = depStatuses.length > 0
    ? depStatuses.every(d => d.installed)
    : deps.length === 0 || deps.every(d => d.startsWith('error'))
  $: canProceedStep2 = allDepsOk && !checking && !installingDep
  $: hasMissing = depStatuses.some(d => !d.installed)
  $: anyCanAutoInstall = depStatuses.some(d => !d.installed && d.canAutoInstall)
  $: animPhase = installingDep ? 'installing'
               : installError ? 'error'
               : allDepsOk ? 'done'
               : 'idle'
</script>

<div class="setup-wizard">
  <div class="nes-container is-dark with-title" style="max-width: 640px; margin: 2rem auto;">
    <p class="title">{$t('setup.welcome')}</p>

    <!-- Progress bar -->
    <div class="steps">
      {#each [1, 2, 3, 4] as s}
        <span class="step-dot" class:active={step >= s}>{s}</span>
        {#if s < 4}<span class="step-line" class:active={step > s}></span>{/if}
      {/each}
    </div>

    <!-- Step 1: Language -->
    {#if step === 1}
      <div class="step-content">
        <h3>{$t('setup.languageSelect')}</h3>
        <div class="lang-options">
          <label class="nes-radio-label">
            <input type="radio" class="nes-radio is-dark" value="zh-TW" bind:group={selectedLang} />
            <span>繁體中文</span>
          </label>
          <label class="nes-radio-label">
            <input type="radio" class="nes-radio is-dark" value="en" bind:group={selectedLang} />
            <span>English</span>
          </label>
        </div>
        <div class="step-actions">
          <button class="nes-btn is-primary" on:click={nextStep}>Next →</button>
        </div>
      </div>

    <!-- Step 2: Environment Check (Auto-Install) -->
    {:else if step === 2}
      <div class="step-content">
        <h3>{$t('setup.envCheck')}</h3>

        <InstallerAnimation phase={animPhase} />

        {#if depStatuses.length > 0}
          <div class="dep-list">
            {#each depStatuses as dep}
              <div class="dep-card" class:installed={dep.installed} class:missing={!dep.installed}>
                <div class="dep-card-header">
                  <span class="dep-icon">{dep.installed ? '✓' : '✗'}</span>
                  <span class="dep-label">{dep.label}</span>
                  {#if dep.installed}
                    <span class="dep-version">{dep.version}</span>
                    <span class="dep-source">({dep.source})</span>
                  {:else}
                    {#if dep.name === 'claude' && depStatuses.find(d => d.name === 'node' && !d.installed)}
                      <span class="dep-needs">{$t('setup.needsNode')}</span>
                    {:else if dep.canAutoInstall && !isInstalling(dep.name)}
                      <button class="nes-btn is-primary dep-install-btn"
                              disabled={!canInstallDep(dep)}
                              on:click={() => installSingle(dep.name)}>
                        {$t('setup.install')}
                      </button>
                    {:else if !dep.canAutoInstall}
                      <span class="dep-manual">{dep.helpText}</span>
                    {/if}
                  {/if}
                </div>
                <p class="dep-desc">{$t(depDescKeys[dep.name] || '')}</p>

                <!-- Per-dep progress -->
                {#if getDepProgress(dep.name) && !dep.installed}
                  {@const prog = getDepProgress(dep.name)}
                  <div class="dep-progress">
                    <div class="progress-bar">
                      <div class="progress-fill"
                           class:error={prog.phase === 'error'}
                           style="width: {prog.percent}%"></div>
                    </div>
                    <span class="progress-msg">{prog.message}</span>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {:else}
          <!-- Fallback: old-style dep list -->
          <div class="dep-list">
            {#each ['git', 'tmux', 'claude'] as dep}
              <div class="dep-item">
                {#if deps.includes(dep)}
                  <span class="dep-status missing">✗</span>
                {:else}
                  <span class="dep-status ok">✓</span>
                {/if}
                <span class="dep-name">{dep}</span>
              </div>
            {/each}
          </div>
        {/if}

        <!-- Install error message -->
        {#if installError}
          <div class="install-error">
            <span class="error-icon">!</span> {installError}
          </div>
        {/if}

        {#if checking}
          <p class="checking-msg">{$t('common.loading')}</p>
        {/if}

        <div class="step-actions">
          <button class="nes-btn" on:click={prevStep}>← Back</button>
          <button class="nes-btn is-warning" on:click={checkDepStatuses} disabled={!!installingDep}>
            {$t('setup.recheck')}
          </button>
          {#if hasMissing && anyCanAutoInstall}
            <button class="nes-btn is-success" on:click={installAll} disabled={!!installingDep}>
              {#if installingDep}{$t('setup.installing')}{:else}{$t('setup.installAll')}{/if}
            </button>
          {/if}
          <button class="nes-btn is-primary" disabled={!canProceedStep2} on:click={nextStep}>Next →</button>
        </div>
      </div>

    <!-- Step 3: Onboarding Chat -->
    {:else if step === 3}
      <div class="step-content">
        <h3>{$t('setup.teamSetup')}</h3>

        <!-- Character portraits -->
        <div class="onboarding-characters">
          <div class="character-slot">
            <CharacterPortrait worker={assistantWorker} scale={2} />
            <span class="character-label">{$t('setup.assistantName')}</span>
          </div>
          {#if hrHired && hrWorker}
            <div class="character-slot hr-entrance">
              <CharacterPortrait worker={hrWorker} scale={2} />
              <span class="character-label">{hrName}</span>
            </div>
          {/if}
        </div>

        <!-- Chat area -->
        <div class="onboarding-chat" bind:this={chatScrollEl}>
          {#each chatMessages as msg}
            <div class="chat-bubble {msg.sender || msg.role}">
              {#if msg.sender === 'system'}
                <div class="system-msg">{msg.content}</div>
              {:else if msg.role === 'user'}
                <div class="user-msg">{msg.content}</div>
              {:else}
                <div class="assistant-msg">
                  <span class="msg-sender">{msg.sender === 'hr' ? hrName : $t('setup.assistantName')}</span>
                  {msg.content}
                </div>
              {/if}
            </div>
          {/each}
          {#if chatLoading}
            <div class="chat-bubble assistant">
              <div class="assistant-msg typing">...</div>
            </div>
          {/if}
        </div>

        {#if chatError}
          <div class="install-error">
            <span class="error-icon">!</span> {chatError}
          </div>
        {/if}

        <!-- Recommended team preview -->
        {#if teamReady && recommendedWorkers.length > 0}
          <div class="recommended-team">
            <h4>{$t('setup.recommendedTeam')}</h4>
            <div class="team-preview">
              {#each recommendedWorkers as rw}
                <span class="team-member">{rw.name} ({rw.skillProfile} / {rw.tier})</span>
              {/each}
            </div>
          </div>
        {/if}

        <!-- API Key form (shown when no AI backend available) -->
        {#if needApiKey}
          <div class="api-key-form">
            <p class="api-key-hint">{$t('setup.apiKeyHint')}</p>
            <div class="api-key-provider-row">
              <label class="nes-radio-label">
                <input type="radio" class="nes-radio is-dark" value="anthropic" bind:group={apiKeyProvider} />
                <span>Claude (Anthropic)</span>
              </label>
              <label class="nes-radio-label">
                <input type="radio" class="nes-radio is-dark" value="openai" bind:group={apiKeyProvider} />
                <span>OpenAI</span>
              </label>
              <label class="nes-radio-label">
                <input type="radio" class="nes-radio is-dark" value="gemini" bind:group={apiKeyProvider} />
                <span>Gemini</span>
              </label>
            </div>
            <div class="chat-input-row">
              <input
                type="password"
                class="nes-input is-dark chat-input"
                bind:value={apiKeyInput}
                placeholder="API Key"
                disabled={apiKeySaving}
              />
              <button class="nes-btn is-primary" on:click={submitApiKey} disabled={apiKeySaving || !apiKeyInput.trim()}>
                {#if apiKeySaving}{$t('common.loading')}{:else}{$t('common.confirm')}{/if}
              </button>
            </div>
            {#if apiKeyError}
              <div class="install-error" style="margin-top: 0.5rem;">
                <span class="error-icon">!</span> {apiKeyError}
              </div>
            {/if}
          </div>

        <!-- Normal chat input -->
        {:else if !teamReady}
          <div class="chat-input-row">
            <textarea
              class="nes-textarea is-dark chat-input"
              bind:value={chatInput}
              on:keydown={handleChatKeydown}
              on:compositionstart={() => composing = true}
              on:compositionend={() => setTimeout(() => composing = false, 300)}
              placeholder={$t('setup.chatPlaceholder')}
              disabled={chatLoading}
              rows="1"
            />
            <button class="nes-btn is-primary" on:click={handleChatSubmit} disabled={chatLoading || !chatInput.trim()}>
              {$t('aiChat.send')}
            </button>
          </div>
        {/if}

        <div class="step-actions">
          <button class="nes-btn" on:click={prevStep}>← Back</button>
          {#if teamReady}
            <button class="nes-btn is-success" disabled={applying} on:click={confirmTeam}>
              {#if applying}{$t('setup.buildingTeam')}{:else}{$t('setup.confirmTeam')}{/if}
            </button>
          {/if}
        </div>
      </div>

    <!-- Step 4: Complete -->
    {:else if step === 4}
      <div class="step-content">
        <h3>{$t('setup.complete')}</h3>
        {#if createdWorkers.length > 0}
          <div class="created-team">
            {#each createdWorkers as w}
              <span class="team-member">{w.avatar} {w.name} ({w.tier})</span>
            {/each}
          </div>
        {:else}
          <p>{$t('setup.customTeam')} — {$t('workers.hire')}</p>
        {/if}
        <div class="step-actions">
          <button class="nes-btn is-success" on:click={onComplete}>{$t('setup.startUsing')}</button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .setup-wizard {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 1rem;
  }

  .steps {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    margin-bottom: 1.5rem;
  }

  .step-dot {
    width: 2rem;
    height: 2rem;
    border: 2px solid #555;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
    color: #555;
  }

  .step-dot.active {
    border-color: #92cc41;
    color: #92cc41;
    background: rgba(146, 204, 65, 0.1);
  }

  .step-line {
    width: 2rem;
    height: 2px;
    background: #555;
  }

  .step-line.active {
    background: #92cc41;
  }

  .step-content {
    padding: 1rem 0;
  }

  .step-content h3 {
    margin-bottom: 1rem;
  }

  .lang-options,
  .template-options {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-bottom: 1.5rem;
  }

  .nes-radio-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }

  .step-actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
    margin-top: 1rem;
    flex-wrap: wrap;
  }

  /* --- Dependency cards (Step 2) --- */
  .dep-list {
    margin-bottom: 1rem;
  }

  .dep-card {
    border: 1px solid #555;
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.5rem;
  }

  .dep-card.installed {
    border-color: rgba(146, 204, 65, 0.4);
  }

  .dep-card.missing {
    border-color: rgba(231, 110, 85, 0.4);
  }

  .dep-card-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .dep-icon {
    font-size: 1rem;
    width: 1.2rem;
    text-align: center;
  }

  .dep-card.installed .dep-icon {
    color: #92cc41;
  }

  .dep-card.missing .dep-icon {
    color: #e76e55;
  }

  .dep-label {
    font-weight: bold;
    font-size: 0.85rem;
  }

  .dep-version {
    color: #92cc41;
    font-size: 0.7rem;
    font-family: monospace;
  }

  .dep-source {
    color: #888;
    font-size: 0.65rem;
  }

  .dep-needs {
    color: #f7d51d;
    font-size: 0.7rem;
  }

  .dep-manual {
    color: #999;
    font-size: 0.65rem;
    font-family: monospace;
  }

  .dep-desc {
    color: #999;
    font-size: 0.7rem;
    margin: 2px 0 0 1.7rem;
  }

  .dep-install-btn {
    font-size: 0.6rem !important;
    padding: 2px 8px !important;
    margin-left: auto;
  }

  /* --- Progress bar --- */
  .dep-progress {
    margin: 0.4rem 0 0 1.7rem;
  }

  .progress-bar {
    width: 100%;
    height: 8px;
    background: #333;
    border: 1px solid #555;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: #92cc41;
    transition: width 0.3s ease;
  }

  .progress-fill.error {
    background: #e76e55;
  }

  .progress-msg {
    font-size: 0.65rem;
    color: #aaa;
    display: block;
    margin-top: 2px;
  }

  /* --- Install error --- */
  .install-error {
    background: rgba(231, 110, 85, 0.1);
    border: 1px solid #e76e55;
    padding: 0.4rem 0.6rem;
    font-size: 0.75rem;
    color: #e76e55;
    margin-bottom: 0.5rem;
  }

  .error-icon {
    font-weight: bold;
  }

  .checking-msg {
    font-size: 0.8rem;
    color: #aaa;
  }

  /* --- Old-style dep fallback --- */
  .dep-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
    font-size: 0.9rem;
  }

  .dep-status.ok {
    color: #92cc41;
  }

  .dep-status.missing {
    color: #e76e55;
  }

  /* --- Team setup (Step 3) --- */
  .created-team {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }

  .team-member {
    background: rgba(255, 255, 255, 0.05);
    padding: 0.25rem 0.5rem;
    border: 1px solid #555;
    font-size: 0.8rem;
  }

  .custom-count {
    margin-bottom: 1rem;
    padding: 0.5rem;
    border: 1px solid #555;
  }

  .custom-count label {
    font-size: 0.8rem;
    display: block;
    margin-bottom: 0.5rem;
  }

  .custom-count input[type="range"] {
    width: 100%;
    cursor: pointer;
  }

  .custom-workers-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 1rem;
    max-height: 320px;
    overflow-y: auto;
  }

  .custom-worker-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
  }

  .cw-num {
    color: #92cc41;
    min-width: 1.5rem;
    font-size: 0.7rem;
  }

  .cw-name {
    width: 100px;
    font-size: 0.7rem !important;
    padding: 4px 6px !important;
  }

  .nes-select-inline {
    font-size: 0.7rem;
    padding: 3px 4px;
    background: var(--bg-secondary, #212529);
    color: inherit;
    border: 2px solid #555;
  }

  /* --- Onboarding Chat (Step 3) --- */
  .onboarding-characters {
    display: flex;
    gap: 1.5rem;
    justify-content: center;
    margin-bottom: 1rem;
  }

  .character-slot {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .character-label {
    font-size: 0.7rem;
    color: #aaa;
  }

  .hr-entrance {
    animation: slideIn 0.5s ease-out;
  }

  @keyframes slideIn {
    from { opacity: 0; transform: translateX(30px); }
    to { opacity: 1; transform: translateX(0); }
  }

  .onboarding-chat {
    border: 1px solid #555;
    padding: 0.5rem;
    max-height: 240px;
    min-height: 120px;
    overflow-y: auto;
    margin-bottom: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .chat-bubble.user {
    align-self: flex-end;
  }

  .chat-bubble.assistant,
  .chat-bubble.hr {
    align-self: flex-start;
  }

  .chat-bubble.system {
    align-self: center;
  }

  .user-msg {
    background: rgba(146, 204, 65, 0.15);
    border: 1px solid rgba(146, 204, 65, 0.3);
    padding: 0.3rem 0.5rem;
    font-size: 0.8rem;
    max-width: 80%;
    word-break: break-word;
  }

  .assistant-msg {
    background: rgba(100, 149, 237, 0.15);
    border: 1px solid rgba(100, 149, 237, 0.3);
    padding: 0.3rem 0.5rem;
    font-size: 0.8rem;
    max-width: 80%;
    word-break: break-word;
  }

  .msg-sender {
    font-size: 0.65rem;
    color: #6495ed;
    display: block;
    margin-bottom: 2px;
    font-weight: bold;
  }

  .system-msg {
    color: #f7d51d;
    font-size: 0.75rem;
    text-align: center;
    padding: 0.2rem 0.5rem;
    border: 1px dashed rgba(247, 213, 29, 0.3);
  }

  .typing {
    animation: blink 1s infinite;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
  }

  .chat-input-row {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }

  .chat-input {
    flex: 1;
    font-size: 0.8rem !important;
    padding: 6px 8px !important;
    resize: none;
    min-height: 32px;
    max-height: 32px;
    overflow: hidden;
    line-height: 1.4;
  }

  .recommended-team {
    border: 1px solid rgba(146, 204, 65, 0.4);
    padding: 0.5rem;
    margin-bottom: 0.75rem;
  }

  .recommended-team h4 {
    margin: 0 0 0.4rem 0;
    font-size: 0.85rem;
    color: #92cc41;
  }

  .team-preview {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  /* --- API Key Form --- */
  .api-key-form {
    border: 1px solid rgba(247, 213, 29, 0.3);
    padding: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .api-key-hint {
    font-size: 0.75rem;
    color: #ccc;
    margin: 0 0 0.5rem 0;
  }

  .api-key-provider-row {
    display: flex;
    gap: 1rem;
    margin-bottom: 0.5rem;
    flex-wrap: wrap;
  }

  .api-key-provider-row .nes-radio-label {
    font-size: 0.75rem;
  }
</style>
