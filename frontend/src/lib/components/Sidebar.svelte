<script>
  import { t } from '../stores/i18n.js'

  export let currentPage = 'dashboard'
  export let darkMode = true

  function toggleTheme() {
    darkMode = !darkMode
    document.body.classList.toggle('light', !darkMode)
    localStorage.setItem('theme', darkMode ? 'dark' : 'light')
  }

  const navKeys = [
    { id: 'dashboard', key: 'nav.dashboard', icon: '⊞' },
    { id: 'objectives', key: 'nav.objectives', icon: '◎' },
    { id: 'projects', key: 'nav.projects', icon: '◈' },
    { id: 'board', key: 'nav.board', icon: '▦' },
    { id: 'boardOverview', key: 'nav.boardOverview', icon: '▩' },
    { id: 'workers', key: 'nav.workers', icon: '☺' },
    { id: 'hierarchy', key: 'nav.hierarchy', icon: '⊿' },
    { id: 'terminal', key: 'nav.terminal', icon: '⊟' },
    { id: 'roles', key: 'nav.roles', icon: '★' },
    { id: 'groups', key: 'nav.groups', icon: '♦' },
    { id: 'office', key: 'nav.office', icon: '▣' },
    { id: 'retro', key: 'nav.retro', icon: '↻' },
    { id: 'approvals', key: 'nav.approvals', icon: '⚑' },
    { id: 'skills', key: 'nav.skills', icon: '◆' },
    { id: 'settings', key: 'nav.settings', icon: '⚙' },
  ]
</script>

<nav class="sidebar">
  <div class="logo">
    <span class="logo-text">AI<br/>SUP</span>
  </div>
  <ul class="nes-list is-disc">
    {#each navKeys as item}
      <li
        class:active={currentPage === item.id}
        on:click={() => { currentPage = item.id; window.location.hash = item.id }}
        on:keydown={(e) => { if (e.key === 'Enter') { currentPage = item.id; window.location.hash = item.id } }}
        role="button"
        tabindex="0"
      >
        <span class="nav-icon">{item.icon}</span>
        <span class="nav-label">{$t(item.key)}</span>
      </li>
    {/each}
  </ul>
  <div class="theme-toggle">
    <button class="nes-btn theme-btn" on:click={toggleTheme}>
      {darkMode ? $t('theme.light') : $t('theme.dark')}
    </button>
  </div>
</nav>

<style>
  .sidebar {
    width: 180px;
    min-width: 180px;
    background-color: var(--bg-secondary);
    border-right: 4px solid var(--border-color);
    display: flex;
    flex-direction: column;
    padding: 16px 8px;
    overflow-y: auto;
  }

  .logo {
    text-align: center;
    margin-bottom: 24px;
    padding: 12px;
    border: 4px solid var(--accent-blue);
    image-rendering: pixelated;
  }

  .logo-text {
    font-size: 16px;
    color: var(--accent-blue);
    line-height: 1.4;
  }

  ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  li {
    padding: 10px 8px;
    margin: 4px 0;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 8px;
    border: 2px solid transparent;
    transition: border-color 0.1s;
  }

  li:hover {
    border-color: var(--accent-blue);
  }

  li.active {
    border-color: var(--accent-green);
    color: var(--accent-green);
  }

  .nav-icon {
    font-size: 14px;
  }

  .nav-label {
    font-size: 10px;
  }

  .theme-toggle {
    margin-top: auto;
    padding-top: 16px;
  }

  .theme-btn {
    font-size: 8px !important;
    padding: 6px 8px !important;
    width: 100%;
  }
</style>
