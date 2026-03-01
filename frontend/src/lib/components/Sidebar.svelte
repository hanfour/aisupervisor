<script>
  export let currentPage = 'dashboard'

  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: '⊞' },
    { id: 'projects', label: 'Projects', icon: '◈' },
    { id: 'board', label: 'Board', icon: '▦' },
    { id: 'workers', label: 'Workers', icon: '☺' },
    { id: 'terminal', label: 'Terminal', icon: '⊟' },
    { id: 'roles', label: 'Roles', icon: '★' },
    { id: 'groups', label: 'Groups', icon: '♦' },
    { id: 'settings', label: 'Settings', icon: '⚙' },
  ]
</script>

<nav class="sidebar">
  <div class="logo">
    <span class="logo-text">AI<br/>SUP</span>
  </div>
  <ul class="nes-list is-disc">
    {#each navItems as item}
      <li
        class:active={currentPage === item.id}
        on:click={() => currentPage = item.id}
        on:keydown={(e) => e.key === 'Enter' && (currentPage = item.id)}
        role="button"
        tabindex="0"
      >
        <span class="nav-icon">{item.icon}</span>
        <span class="nav-label">{item.label}</span>
      </li>
    {/each}
  </ul>
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
</style>
