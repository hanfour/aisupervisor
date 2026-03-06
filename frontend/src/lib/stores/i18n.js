import { writable, derived } from 'svelte/store'

export const language = writable('zh-TW')

const translations = {
  // --- Sidebar ---
  'nav.dashboard': { en: 'Dashboard', zh: '儀表板' },
  'nav.projects': { en: 'Projects', zh: '專案' },
  'nav.board': { en: 'Board', zh: '看板' },
  'nav.workers': { en: 'Workers', zh: '員工' },
  'nav.hierarchy': { en: 'Hierarchy', zh: '組織架構' },
  'nav.terminal': { en: 'Terminal', zh: '終端機' },
  'nav.roles': { en: 'Roles', zh: '角色' },
  'nav.groups': { en: 'Groups', zh: '群組' },
  'nav.office': { en: 'Office', zh: '辦公室' },
  'nav.retro': { en: 'Retro', zh: '回顧' },
  'nav.settings': { en: 'Settings', zh: '設定' },
  'nav.skills': { en: 'Skills', zh: '技能' },
  'theme.light': { en: '☀ Light', zh: '☀ 亮色' },
  'theme.dark': { en: '☾ Dark', zh: '☾ 暗色' },

  // --- Dashboard ---
  'dashboard.company': { en: 'Company', zh: '公司' },
  'dashboard.projects': { en: 'Projects', zh: '專案' },
  'dashboard.inProgress': { en: 'In Progress', zh: '進行中' },
  'dashboard.idleWorkers': { en: 'Idle Workers', zh: '閒置員工' },
  'dashboard.reviewsPending': { en: 'Reviews Pending', zh: '待審查' },
  'dashboard.trainingPairs': { en: 'Training Pairs', zh: '訓練資料對' },
  'dashboard.reviewQueue': { en: 'Review Queue', zh: '審查佇列' },
  'dashboard.trainingStats': { en: 'Training Stats', zh: '訓練統計' },
  'dashboard.sessions': { en: 'Sessions', zh: '工作階段' },
  'dashboard.events': { en: 'Events', zh: '事件' },
  'dashboard.noSessions': { en: 'No sessions monitored', zh: '沒有監控的工作階段' },

  // --- Projects ---
  'projects.title': { en: 'Projects', zh: '專案' },
  'projects.newProject': { en: '+ New Project', zh: '+ 新增專案' },
  'projects.aiCreate': { en: 'AI Create', zh: 'AI 建立' },
  'projects.empty': { en: 'No projects yet. Create one to get started!', zh: '還沒有專案。建立一個開始吧！' },
  'projects.deleteTitle': { en: 'Delete Project', zh: '刪除專案' },
  'projects.deleteConfirm': { en: 'Are you sure you want to delete', zh: '確定要刪除' },
  'projects.deleteConfirmSuffix': { en: 'and all its tasks?', zh: '及其所有任務嗎？' },
  'projects.delete': { en: 'Delete', zh: '刪除' },
  'projects.repo': { en: 'repo:', zh: '儲存庫：' },
  'projects.branch': { en: 'branch:', zh: '分支：' },
  'projects.goals': { en: 'goals:', zh: '目標：' },

  // --- Project Form ---
  'projectForm.title': { en: 'New Project', zh: '新增專案' },
  'projectForm.name': { en: 'Name', zh: '名稱' },
  'projectForm.description': { en: 'Description', zh: '描述' },
  'projectForm.repoPath': { en: 'Repo Path', zh: '儲存庫路徑' },
  'projectForm.baseBranch': { en: 'Base Branch', zh: '基礎分支' },
  'projectForm.goalsLabel': { en: 'Goals (one per line)', zh: '目標（每行一個）' },
  'projectForm.create': { en: 'Create', zh: '建立' },

  // --- Task Form ---
  'taskForm.title': { en: 'New Task', zh: '新增任務' },
  'taskForm.titleLabel': { en: 'Title', zh: '標題' },
  'taskForm.description': { en: 'Description', zh: '描述' },
  'taskForm.prompt': { en: 'Prompt (for Claude Code)', zh: '提示詞（給 Claude Code）' },
  'taskForm.type': { en: 'Type', zh: '類型' },
  'taskForm.typeCode': { en: 'Code', zh: '程式碼' },
  'taskForm.typeResearch': { en: 'Research', zh: '研究' },
  'taskForm.priority': { en: 'Priority', zh: '優先級' },
  'taskForm.milestone': { en: 'Milestone', zh: '里程碑' },
  'taskForm.dependencies': { en: 'Dependencies', zh: '依賴項目' },
  'taskForm.create': { en: 'Create', zh: '建立' },

  // --- Workers ---
  'workers.title': { en: 'Workers', zh: '員工' },
  'workers.hire': { en: '+ Hire Worker', zh: '+ 雇用員工' },
  'workers.hireTitle': { en: 'Hire Worker', zh: '雇用員工' },
  'workers.name': { en: 'Name', zh: '名稱' },
  'workers.avatar': { en: 'Avatar', zh: '頭像' },
  'workers.tier': { en: 'Tier', zh: '等級' },
  'workers.parent': { en: 'Parent (Manager)', zh: '上級（管理員）' },
  'workers.cliTool': { en: 'CLI Tool', zh: 'CLI 工具' },
  'workers.skillProfile': { en: 'Skill Profile', zh: '技能配置' },
  'workers.backendId': { en: 'Backend ID (optional)', zh: '後端 ID（選填）' },
  'workers.hireBtn': { en: 'Hire', zh: '雇用' },
  'workers.promote': { en: 'Promote', zh: '升遷' },
  'workers.manager': { en: 'Manager:', zh: '管理員：' },
  'workers.none': { en: 'None', zh: '無' },
  'workers.noWorkers': { en: 'No workers', zh: '沒有員工' },

  // --- Board ---
  'board.addTask': { en: '+ Add Task', zh: '+ 新增任務' },
  'board.launchAll': { en: 'Launch All Ready', zh: '啟動所有就緒' },
  'board.backlog': { en: 'Backlog', zh: '待辦' },
  'board.ready': { en: 'Ready', zh: '就緒' },
  'board.assigned': { en: 'Assigned', zh: '已分配' },
  'board.inProgress': { en: 'In Progress', zh: '進行中' },
  'board.review': { en: 'Review', zh: '審查中' },
  'board.revision': { en: 'Revision', zh: '修改中' },
  'board.done': { en: 'Done', zh: '完成' },
  'board.failed': { en: 'Failed', zh: '失敗' },

  // --- AI Chat Project Creator ---
  'aiChat.title': { en: 'AI Project Creator', zh: 'AI 專案建立助手' },
  'aiChat.description': { en: 'Describe the project you want to create. The AI will ask follow-up questions if needed.', zh: '描述你想建立的專案。AI 會在需要時提出追問。' },
  'aiChat.thinking': { en: 'Thinking...', zh: '思考中...' },
  'aiChat.createProject': { en: 'Create Project', zh: '建立專案' },
  'aiChat.placeholder': { en: 'Describe your project...', zh: '描述你的專案...' },
  'aiChat.send': { en: 'Send', zh: '送出' },

  // --- Worker Chat ---
  'chat.placeholder': { en: 'Say something...', zh: '說點什麼...' },
  'chat.startConversation': { en: 'Start a conversation with', zh: '開始和' },
  'chat.startConversationSuffix': { en: '!', zh: '對話吧！' },

  // --- Confirm Dialog ---
  'confirm.lowConfidence': { en: 'Low Confidence Decision', zh: '低信心決策' },
  'confirm.session': { en: 'Session:', zh: '工作階段：' },
  'confirm.summary': { en: 'Summary:', zh: '摘要：' },
  'confirm.suggested': { en: 'Suggested:', zh: '建議：' },
  'confirm.reasoning': { en: 'Reasoning:', zh: '推理：' },
  'confirm.confidence': { en: 'Confidence:', zh: '信心：' },
  'confirm.approve': { en: 'Approve', zh: '核准' },
  'confirm.dismiss': { en: 'Dismiss', zh: '忽略' },

  // --- Settings ---
  'settings.language': { en: 'Language / 語言', zh: 'Language / 語言' },
  'settings.configuration': { en: 'Configuration', zh: '組態設定' },
  'settings.backends': { en: 'Backends', zh: '後端' },
  'settings.noBackends': { en: 'No backends configured', zh: '尚未設定後端' },
  'settings.autoApprove': { en: 'Auto-Approve Rules', zh: '自動核准規則' },
  'settings.chatBackend': { en: 'Chat Backend', zh: '聊天後端' },
  'settings.noAutoApprove': { en: 'No auto-approve rules', zh: '沒有自動核准規則' },
  'settings.nameCol': { en: 'Name', zh: '名稱' },
  'settings.typeCol': { en: 'Type', zh: '類型' },
  'settings.modelCol': { en: 'Model', zh: '模型' },
  'settings.labelCol': { en: 'Label', zh: '標籤' },
  'settings.patternCol': { en: 'Pattern', zh: '模式' },
  'settings.responseCol': { en: 'Response', zh: '回應' },

  // --- Hierarchy ---
  'hierarchy.title': { en: 'Hierarchy', zh: '組織架構' },
  'hierarchy.noSubs': { en: 'No subordinates', zh: '沒有下屬' },

  // --- Roles ---
  'roles.title': { en: 'Roles', zh: '角色' },
  'roles.noRoles': { en: 'No roles configured', zh: '尚未設定角色' },

  // --- Groups ---
  'groups.title': { en: 'Groups', zh: '群組' },
  'groups.noGroups': { en: 'No groups configured', zh: '尚未設定群組' },
  'groups.liveDiscussions': { en: 'Live Discussions', zh: '即時討論' },
  'groups.noLive': { en: 'No live discussions', zh: '沒有進行中的討論' },
  'groups.recentDecisions': { en: 'Recent Decisions', zh: '最近決策' },
  'groups.noRecent': { en: 'No active discussions', zh: '沒有進行中的討論' },

  // --- Review Queue ---
  'reviewQueue.noReviews': { en: 'No reviews pending', zh: '沒有待審查項目' },
  'reviewQueue.task': { en: 'Task:', zh: '任務：' },
  'reviewQueue.engineer': { en: 'Engineer:', zh: '工程師：' },
  'reviewQueue.reviewManager': { en: 'Manager:', zh: '管理員：' },

  // --- Training Stats ---
  'training.total': { en: 'Total Pairs', zh: '總資料對' },
  'training.accepted': { en: 'Accepted', zh: '已接受' },
  'training.rejected': { en: 'Rejected', zh: '已拒絕' },
  'training.approvalRate': { en: 'Approval Rate', zh: '核准率' },
  'training.noData': { en: 'No training data yet', zh: '尚無訓練資料' },

  // --- Research Report ---
  'report.summary': { en: 'Summary', zh: '摘要' },
  'report.keyFindings': { en: 'Key Findings', zh: '主要發現' },
  'report.recommendations': { en: 'Recommendations', zh: '建議' },
  'report.references': { en: 'References', zh: '參考資料' },
  'report.fullContent': { en: 'Full Content', zh: '完整內容' },

  // --- Worker Detail ---
  'workerDetail.status': { en: 'Status:', zh: '狀態：' },
  'workerDetail.tier': { en: 'Tier:', zh: '等級：' },
  'workerDetail.task': { en: 'Task:', zh: '任務：' },
  'workerDetail.personality': { en: 'Personality', zh: '性格' },
  'workerDetail.relationships': { en: 'Relationships', zh: '人際關係' },
  'workerDetail.log': { en: 'Worker Log', zh: '員工日誌' },
  'workerDetail.chat': { en: 'Chat', zh: '對話' },
  'workerDetail.delete': { en: 'Delete Worker', zh: '刪除員工' },
  'workerDetail.regenerate': { en: 'Regenerate', zh: '重新生成' },

  // --- Common ---
  'common.cancel': { en: 'Cancel', zh: '取消' },
  'common.save': { en: 'Save', zh: '儲存' },
  'common.close': { en: 'Close', zh: '關閉' },
  'common.loading': { en: 'Loading...', zh: '載入中...' },
  'common.error': { en: 'Error', zh: '錯誤' },
  'common.assign': { en: 'Assign', zh: '分配' },
  'common.complete': { en: 'Complete', zh: '完成' },
  'common.edit': { en: 'Edit', zh: '編輯' },
  'common.delete': { en: 'Delete', zh: '刪除' },

  // --- Skill Profiles ---
  'skills.title': { en: 'Skill Profiles', zh: '技能配置' },
  'skills.newProfile': { en: '+ New Profile', zh: '+ 新增配置' },
  'skills.builtIn': { en: 'Built-in', zh: '內建' },
  'skills.edit': { en: 'Edit', zh: '編輯' },
  'skills.editProfile': { en: 'Edit Skill Profile', zh: '編輯技能配置' },
  'skills.newProfileTitle': { en: 'New Skill Profile', zh: '新增技能配置' },
  'skills.noProfiles': { en: 'No skill profiles', zh: '沒有技能配置' },
  'skills.id': { en: 'ID', zh: 'ID' },
  'skills.name': { en: 'Name', zh: '名稱' },
  'skills.icon': { en: 'Icon', zh: '圖示' },
  'skills.model': { en: 'Model', zh: '模型' },
  'skills.permissionMode': { en: 'Permission Mode', zh: '權限模式' },
  'skills.description': { en: 'Description', zh: '描述' },
  'skills.systemPrompt': { en: 'System Prompt', zh: '系統提示' },
  'skills.allowedTools': { en: 'Allowed Tools (one per line)', zh: '允許工具（每行一個）' },
  'skills.disallowedTools': { en: 'Disallowed Tools (one per line)', zh: '禁用工具（每行一個）' },
  'skills.extraCliArgs': { en: 'Extra CLI Args', zh: '額外 CLI 參數' },
  'skills.builtInNoDelete': { en: 'Built-in profiles cannot be deleted', zh: '內建配置無法刪除' },

  // --- Office ---
  'office.title': { en: 'PIXEL OFFICE', zh: 'PIXEL OFFICE' },
  'office.workers': { en: 'workers', zh: '位員工' },
  'office.overflow': { en: 'Workers without desks', zh: '沒有座位的員工' },

  // --- Worker Detail Drawer ---
  'workerDetail.title': { en: 'Worker Detail', zh: '員工詳情' },
  'workerDetail.id': { en: 'ID', zh: 'ID' },
  'workerDetail.backend': { en: 'Backend', zh: '後端' },
  'workerDetail.cliTool': { en: 'CLI Tool', zh: 'CLI 工具' },
  'workerDetail.model': { en: 'Model', zh: '模型' },
  'workerDetail.skillProfile': { en: 'Skill Profile', zh: '技能配置' },
  'workerDetail.created': { en: 'Created', zh: '建立時間' },
  'workerDetail.manager': { en: 'Manager', zh: '管理員' },
  'workerDetail.subordinates': { en: 'Subordinates', zh: '下屬' },
  'workerDetail.viewLogs': { en: 'View Logs', zh: '查看日誌' },
  'workerDetail.noNone': { en: 'None', zh: '無' },
  'workerDetail.noSubs': { en: 'No subordinates', zh: '沒有下屬' },
  'workerDetail.noneClickToSet': { en: 'None (click to set)', zh: '無（點擊設定）' },
  'workerDetail.notFound': { en: 'Worker not found', zh: '找不到員工' },
  'workerDetail.personalitySection': { en: 'Personality', zh: '性格' },
  'workerDetail.noNarrative': { en: 'No personality description generated yet', zh: '尚未生成性格描述' },
  'workerDetail.mood': { en: 'Mood', zh: '情緒' },
  'workerDetail.moodCurrent': { en: 'Mood:', zh: '心情：' },
  'workerDetail.energy': { en: 'Energy:', zh: '能量：' },
  'workerDetail.morale': { en: 'Morale:', zh: '士氣：' },
  'workerDetail.traits': { en: 'Traits', zh: '特質' },
  'workerDetail.generateNarrative': { en: 'Generate Personality (Ollama)', zh: '生成性格描述 (Ollama)' },
  'workerDetail.relationshipsSection': { en: 'Relationships', zh: '人際關係' },
  'workerDetail.affinity': { en: 'Affinity', zh: '好感' },
  'workerDetail.trust': { en: 'Trust', zh: '信任' },

  // --- Gender & Birthday ---
  'workers.gender': { en: 'Gender', zh: '性別' },
  'workerDetail.gender': { en: 'Gender', zh: '性別' },
  'workerDetail.birthday': { en: 'Birthday', zh: '生日' },
  'workerDetail.age': { en: 'Age', zh: '年齡' },
  'gender.male': { en: 'Male', zh: '男性' },
  'gender.female': { en: 'Female', zh: '女性' },

  // --- Role ---
  'workerDetail.role': { en: 'Role', zh: '職能' },
  'role.coder': { en: 'Coder', zh: '程式員' },
  'role.architect': { en: 'Architect', zh: '架構師' },
  'role.qa': { en: 'QA', zh: '品質保證' },
  'role.security': { en: 'Security', zh: '安全' },
  'role.devops': { en: 'DevOps', zh: '維運' },
  'role.designer': { en: 'Designer', zh: '設計師' },

  // --- Habits ---
  'workerDetail.habits': { en: 'Habits', zh: '日常習慣' },
  'habit.coffeeTime': { en: 'Coffee Time', zh: '咖啡時間' },
  'habit.favoriteSpot': { en: 'Favorite Spot', zh: '喜愛的位置' },
  'habit.workStyle': { en: 'Work Style', zh: '工作風格' },
  'habit.socialPreference': { en: 'Social Preference', zh: '社交偏好' },
  'habit.quirks': { en: 'Quirks', zh: '小怪癖' },

  // --- Skill Scores ---
  'workerDetail.skillScores': { en: 'Skill Scores', zh: '技能分數' },
  'skill.carefulness': { en: 'Carefulness', zh: '細心度' },
  'skill.boundaryChecking': { en: 'Boundary Checking', zh: '邊界檢查' },
  'skill.testCoverageAware': { en: 'Test Coverage', zh: '測試覆蓋' },
  'skill.communicationClarity': { en: 'Communication', zh: '溝通清晰' },
  'skill.codeQuality': { en: 'Code Quality', zh: '程式品質' },
  'skill.securityAwareness': { en: 'Security', zh: '安全意識' },

  // --- Backstory ---
  'workerDetail.backstory': { en: 'Backstory', zh: '背景故事' },

  // --- Growth Log ---
  'workerDetail.growthLog': { en: 'Growth Log', zh: '成長紀錄' },
  'workerDetail.tasksCompleted': { en: 'Tasks Completed', zh: '完成任務數' },

  // --- Trait labels ---
  'trait.sociability': { en: 'Sociability', zh: '社交性' },
  'trait.focus': { en: 'Focus', zh: '專注力' },
  'trait.creativity': { en: 'Creativity', zh: '創造力' },
  'trait.empathy': { en: 'Empathy', zh: '同理心' },
  'trait.ambition': { en: 'Ambition', zh: '野心' },
  'trait.humor': { en: 'Humor', zh: '幽默感' },

  // --- Research Report ---
  'report.title': { en: 'Research Report', zh: '研究報告' },
  'report.researcher': { en: 'Researcher:', zh: '研究員：' },
  'report.date': { en: 'Date:', zh: '日期：' },
  'report.showFull': { en: 'Show Full Content', zh: '顯示完整內容' },
  'report.hideFull': { en: 'Hide Full Content', zh: '隱藏完整內容' },

  // --- Hierarchy Page ---
  'hierarchy.companyHierarchy': { en: 'Company Hierarchy', zh: '公司組織架構' },
  'hierarchy.reviewQueue': { en: 'Review Queue', zh: '審查佇列' },
  'hierarchy.trainingStats': { en: 'Training Stats', zh: '訓練統計' },
  'hierarchy.noHired': { en: 'not hired yet', zh: '尚未雇用' },

  // --- Groups ---
  'groups.id': { en: 'ID', zh: 'ID' },
  'groups.name': { en: 'Name', zh: '名稱' },
  'groups.leader': { en: 'Leader', zh: '組長' },
  'groups.roles': { en: 'Roles', zh: '角色' },
  'groups.threshold': { en: 'Threshold', zh: '閾值' },
  'groups.activeDiscussions': { en: 'Active Discussions', zh: '進行中的討論' },

  // --- Roles ---
  'roles.allRoles': { en: 'All Roles', zh: '所有角色' },
  'roles.id': { en: 'ID', zh: 'ID' },
  'roles.name': { en: 'Name', zh: '名稱' },
  'roles.mode': { en: 'Mode', zh: '模式' },
  'roles.priority': { en: 'Priority', zh: '優先級' },
  'roles.perTerminal': { en: 'Per-Terminal Assignment', zh: '終端分配' },
  'roles.session': { en: 'Session:', zh: '工作階段：' },
  'roles.noSessions': { en: 'No sessions available', zh: '沒有可用的工作階段' },

  // --- Retro ---
  'retro.title': { en: 'Project Retro', zh: '專案回顧' },
  'retro.summary': { en: 'Summary', zh: '摘要' },
  'retro.workerFeedback': { en: 'Worker Feedback', zh: '員工回饋' },
  'retro.skillAdjustments': { en: 'Skill Adjustments', zh: '技能調整' },
  'retro.triggerRetro': { en: 'Run Retro', zh: '執行回顧' },
  'retro.noReports': { en: 'No retro reports yet', zh: '尚無回顧報告' },
  'retro.strengths': { en: 'Strengths', zh: '優勢' },
  'retro.weaknesses': { en: 'Weaknesses', zh: '待改善' },
  'retro.suggestions': { en: 'Suggestions', zh: '建議' },
  'retro.promptAdditions': { en: 'Prompt Additions', zh: '提示詞新增' },
  'retro.addTools': { en: 'Add Tools', zh: '新增工具' },
  'retro.removeTools': { en: 'Remove Tools', zh: '移除工具' },
  'retro.modelChange': { en: 'Model Change', zh: '模型變更' },
  'retro.appliedAt': { en: 'Applied', zh: '套用時間' },
  'retro.running': { en: 'Running retro...', zh: '執行回顧中...' },
  'retro.skillOverride': { en: 'Skill Override', zh: '技能覆蓋' },

  // --- Terminal ---
  'terminal.info': { en: 'Info', zh: '資訊' },
  'terminal.sessionEvents': { en: 'Session Events', zh: '工作階段事件' },
  'terminal.assignedRoles': { en: 'Assigned roles:', zh: '已分配角色：' },
  'terminal.noEvents': { en: 'No events for this session', zh: '此工作階段沒有事件' },
  'terminal.back': { en: '< Back', zh: '< 返回' },
}

/** @type {import('svelte/store').Readable<(key: string) => string>} */
export const t = derived(language, ($lang) => {
  return (key) => {
    const entry = translations[key]
    if (!entry) return key
    return $lang === 'en' ? entry.en : entry.zh
  }
})

/** Initialize language from backend */
export async function initLanguage() {
  if (window.go?.gui?.CompanyApp) {
    try {
      const lang = await window.go.gui.CompanyApp.GetLanguage()
      if (lang) language.set(lang)
    } catch {
      // ignore
    }
  }
}

/** Set language and persist to backend */
export async function setLanguage(lang) {
  language.set(lang)
  if (window.go?.gui?.CompanyApp) {
    try {
      await window.go.gui.CompanyApp.SetLanguage(lang)
    } catch {
      // ignore
    }
  }
}
