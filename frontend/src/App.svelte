<script>
  import { onMount } from 'svelte'
  import Sidebar from './lib/components/Sidebar.svelte'
  import Toast from './lib/components/Toast.svelte'
  import DashboardPage from './lib/pages/DashboardPage.svelte'
  import TerminalPage from './lib/pages/TerminalPage.svelte'
  import { initEventStore } from './lib/stores/events.js'
  import { loadSessions } from './lib/stores/sessions.js'
  import { loadRoles } from './lib/stores/roles.js'
  import { initDiscussionStore } from './lib/stores/discussions.js'
  import { initCompanyStore } from './lib/stores/company.js'
  import { addError } from './lib/stores/errors.js'
  import ProjectsPage from './lib/pages/ProjectsPage.svelte'
  import WorkersPage from './lib/pages/WorkersPage.svelte'

  let currentPage = 'dashboard'
  let selectedSessionId = ''
  let selectedProjectId = ''
  let darkMode = true

  // Lazy-loaded page components for Phase 5
  let RolesPage = null
  let GroupsPage = null
  let SettingsPage = null
  // Lazy-loaded company pages
  let ProjectBoardPage = null
  let HierarchyPage = null
  let OfficePage = null

  onMount(async () => {
    // Theme initialization
    const savedTheme = localStorage.getItem('theme')
    if (savedTheme === 'light') {
      darkMode = false
      document.body.classList.add('light')
    }

    initEventStore()
    initDiscussionStore()
    initCompanyStore()

    // Listen for supervisor error events
    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn('supervisor:error', (msg) => {
        addError(msg || 'Unknown supervisor error')
      })
    }

    try {
      await loadSessions()
    } catch (e) {
      addError('Failed to load sessions: ' + e.message)
    }
    try {
      await loadRoles()
    } catch (e) {
      addError('Failed to load roles: ' + e.message)
    }

    // Dynamically import Phase 5 pages if available
    try {
      const rolesModule = await import('./lib/pages/RolesPage.svelte')
      RolesPage = rolesModule.default
    } catch {}
    try {
      const groupsModule = await import('./lib/pages/GroupsPage.svelte')
      GroupsPage = groupsModule.default
    } catch {}
    try {
      const settingsModule = await import('./lib/pages/SettingsPage.svelte')
      SettingsPage = settingsModule.default
    } catch {}
    // Company board page
    try {
      const boardModule = await import('./lib/pages/ProjectBoardPage.svelte')
      ProjectBoardPage = boardModule.default
    } catch {}
    // Hierarchy page
    try {
      const hierarchyModule = await import('./lib/pages/HierarchyPage.svelte')
      HierarchyPage = hierarchyModule.default
    } catch {}
    // Office page
    try {
      const officeModule = await import('./lib/pages/OfficePage.svelte')
      OfficePage = officeModule.default
    } catch {}
  })

  function navigate(page, id) {
    currentPage = page
    if (page === 'terminal' && id) selectedSessionId = id
    if (page === 'board' && id) selectedProjectId = id
  }
</script>

<Toast />
<div class="app-layout">
  <Sidebar bind:currentPage bind:darkMode />

  <main class="main-content p-2">
    {#if currentPage === 'dashboard'}
      <DashboardPage onNavigate={navigate} />
    {:else if currentPage === 'projects'}
      <ProjectsPage onNavigate={navigate} />
    {:else if currentPage === 'board' && ProjectBoardPage}
      <svelte:component this={ProjectBoardPage} projectId={selectedProjectId} onNavigate={navigate} />
    {:else if currentPage === 'workers'}
      <WorkersPage />
    {:else if currentPage === 'hierarchy' && HierarchyPage}
      <svelte:component this={HierarchyPage} />
    {:else if currentPage === 'terminal'}
      <TerminalPage
        sessionId={selectedSessionId}
        onBack={() => navigate('dashboard')}
      />
    {:else if currentPage === 'roles' && RolesPage}
      <svelte:component this={RolesPage} />
    {:else if currentPage === 'groups' && GroupsPage}
      <svelte:component this={GroupsPage} />
    {:else if currentPage === 'office' && OfficePage}
      <svelte:component this={OfficePage} />
    {:else if currentPage === 'settings' && SettingsPage}
      <svelte:component this={SettingsPage} />
    {:else}
      <div class="nes-container is-dark with-title">
        <p class="title">{currentPage}</p>
        <p>Coming soon...</p>
      </div>
    {/if}
  </main>
</div>

<style>
  .app-layout {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  .main-content {
    flex: 1;
    overflow: auto;
    display: flex;
    flex-direction: column;
  }
</style>
