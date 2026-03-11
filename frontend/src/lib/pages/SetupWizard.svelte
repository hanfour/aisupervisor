<script>
  import { t, language, setLanguage } from '../stores/i18n.js'

  export let onComplete = () => {}

  let step = 1
  let selectedLang = 'zh-TW'
  let deps = []
  let checking = false
  let selectedTemplate = 'starter'
  let applying = false
  let createdWorkers = []

  async function checkDeps() {
    checking = true
    try {
      deps = await window.go.gui.CompanyApp.CheckDependencies()
    } catch (e) {
      deps = ['error: ' + e.message]
    }
    checking = false
  }

  function nextStep() {
    step++
    if (step === 2) {
      checkDeps()
    }
  }

  function prevStep() {
    if (step > 1) step--
  }

  async function applySetup() {
    applying = true
    try {
      await setLanguage(selectedLang)
      await window.go.gui.CompanyApp.ApplyOnboarding({
        companyName: '',
        language: selectedLang,
        teamTemplate: selectedTemplate,
        apiKeySource: ''
      })
      // Load the created workers
      const workers = await window.go.gui.CompanyApp.ListWorkers()
      createdWorkers = workers
      step = 4
    } catch (e) {
      alert('Setup failed: ' + e.message)
    }
    applying = false
  }

  const requiredDeps = ['tmux', 'claude', 'git']

  $: allDepsOk = requiredDeps.every(d => !deps.includes(d))
  $: canProceedStep2 = allDepsOk && !checking
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

    <!-- Step 2: Environment Check -->
    {:else if step === 2}
      <div class="step-content">
        <h3>{$t('setup.envCheck')}</h3>
        <div class="dep-list">
          {#each requiredDeps as dep}
            <div class="dep-item">
              {#if deps.includes(dep)}
                <span class="dep-status missing">✗</span>
              {:else}
                <span class="dep-status ok">✓</span>
              {/if}
              <span class="dep-name">{dep}</span>
              {#if deps.includes(dep)}
                <div class="dep-install-guide">
                  {#if dep === 'claude'}
                    <p class="dep-hint">Claude Code CLI is required to run AI workers.</p>
                    <code class="dep-cmd">npm install -g @anthropic-ai/claude-code</code>
                    <p class="dep-hint">After installing, run <code>claude</code> to set up your API key.</p>
                  {:else if dep === 'tmux'}
                    <p class="dep-hint">tmux is used to manage worker sessions.</p>
                    <code class="dep-cmd">brew install tmux</code>
                    <p class="dep-hint-sub">Note: tmux may be bundled with the app. If this shows missing, install via Homebrew.</p>
                  {:else if dep === 'git'}
                    <p class="dep-hint">git is needed for branch management per task.</p>
                    <code class="dep-cmd">xcode-select --install</code>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        </div>
        {#if checking}
          <p>{$t('common.loading')}</p>
        {/if}
        <div class="step-actions">
          <button class="nes-btn" on:click={prevStep}>← Back</button>
          <button class="nes-btn is-warning" on:click={checkDeps}>{$t('setup.recheck')}</button>
          <button class="nes-btn is-primary" disabled={!canProceedStep2} on:click={nextStep}>Next →</button>
        </div>
      </div>

    <!-- Step 3: Team Setup -->
    {:else if step === 3}
      <div class="step-content">
        <h3>{$t('setup.teamSetup')}</h3>
        <div class="template-options">
          <label class="nes-radio-label">
            <input type="radio" class="nes-radio is-dark" value="starter" bind:group={selectedTemplate} />
            <span>🌟 {$t('setup.starterTeam')} (1 + 2)</span>
          </label>
          <label class="nes-radio-label">
            <input type="radio" class="nes-radio is-dark" value="full" bind:group={selectedTemplate} />
            <span>🏢 {$t('setup.fullTeam')} (1 + 3 + 12)</span>
          </label>
          <label class="nes-radio-label">
            <input type="radio" class="nes-radio is-dark" value="custom" bind:group={selectedTemplate} />
            <span>⚙️ {$t('setup.customTeam')}</span>
          </label>
        </div>
        <div class="step-actions">
          <button class="nes-btn" on:click={prevStep}>← Back</button>
          <button class="nes-btn is-primary" disabled={applying} on:click={applySetup}>
            {#if applying}{$t('common.loading')}{:else}{$t('setup.startUsing')}{/if}
          </button>
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
  }

  .dep-list {
    margin-bottom: 1rem;
  }

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

  .dep-hint {
    color: #999;
    font-size: 0.75rem;
    margin: 0;
  }

  .dep-hint-sub {
    color: #666;
    font-size: 0.65rem;
    margin: 2px 0 0 0;
  }

  .dep-install-guide {
    margin-left: 0.5rem;
    padding: 0.25rem 0;
  }

  .dep-cmd {
    display: block;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid #444;
    padding: 4px 8px;
    margin: 4px 0;
    font-size: 0.7rem;
    color: #92cc41;
  }

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
</style>
