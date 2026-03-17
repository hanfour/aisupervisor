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
  'nav.approvals': { en: 'Approvals', zh: '審批' },
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
  'taskForm.typeAdmin': { en: 'Admin', zh: '行政' },
  'taskForm.typeHR': { en: 'HR', zh: '人資' },
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
  'workers.recommended': { en: 'Recommended', zh: '推薦配置' },

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
  'settings.dangerZone': { en: 'Danger Zone', zh: '危險區域' },
  'settings.clearAllProjects': { en: 'Clear All Projects', zh: '清除全部專案' },
  'settings.clearAllProjectsDesc': { en: 'Delete all projects, tasks, and stop active workers.', zh: '刪除所有專案、任務，並停止正在工作的員工。' },
  'settings.clearConfirm': { en: 'Are you sure? This cannot be undone.', zh: '確定要清除嗎？此操作無法復原。' },
  'settings.clearForceConfirm': { en: '{count} worker(s) currently active. Force stop and clear all?', zh: '目前有 {count} 位員工正在工作中，要強制中斷並清除全部嗎？' },
  'settings.clearSuccess': { en: 'All projects cleared.', zh: '已清除全部專案。' },

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
  'common.confirm': { en: 'Confirm', zh: '確認' },
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
  'skills.searchMP': { en: 'Search SkillsMP', zh: '搜尋技能市場' },
  'skills.importSkill': { en: 'Import', zh: '匯入' },
  'skills.mergeOptimize': { en: 'Merge & Optimize', zh: '合併優化' },
  'skills.searching': { en: 'Searching...', zh: '搜尋中...' },
  'skills.merging': { en: 'AI merging skills...', zh: 'AI 合併技能中...' },
  'skills.mergeName': { en: 'Profile Name', zh: '配置名稱' },
  'skills.mergePreview': { en: 'Merge Preview', zh: '合併預覽' },
  'skills.searchResults': { en: 'Search Results', zh: '搜尋結果' },
  'skills.noResults': { en: 'No results found', zh: '沒有找到結果' },
  'skills.stars': { en: 'stars', zh: '星' },
  'skills.aiSearch': { en: 'AI Search', zh: 'AI 搜尋' },

  // --- Office ---
  'office.title': { en: 'PIXEL OFFICE', zh: 'PIXEL OFFICE' },
  'office.workers': { en: 'workers', zh: '位員工' },
  'office.overflow': { en: 'Workers without desks', zh: '沒有座位的員工' },
  'office.layout': { en: 'Office Layout', zh: '辦公室佈局' },
  'office.standard': { en: 'Standard Office', zh: '標準辦公室' },
  'office.startup': { en: 'Startup Studio', zh: '新創工作室' },
  'office.enterprise': { en: 'Enterprise Tower', zh: '企業大樓' },

  // --- Appearance Editor ---
  'appearance.title': { en: 'Appearance', zh: '外觀' },
  'appearance.skin': { en: 'Skin Tone', zh: '膚色' },
  'appearance.outfit': { en: 'Outfit', zh: '服裝' },
  'appearance.hair': { en: 'Hairstyle', zh: '髮型' },
  'appearance.preview': { en: 'Preview', zh: '預覽' },
  'appearance.save': { en: 'Save', zh: '儲存' },
  'appearance.reset': { en: 'Reset to Default', zh: '重設為預設' },

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
  'workerDetail.generateNarrative': { en: 'Generate Personality (AI)', zh: '生成性格描述 (AI)' },
  'workerDetail.generating': { en: 'Generating...', zh: '生成中...' },
  'workerDetail.narrativeError': { en: 'Failed to generate personality. Check AI backend settings.', zh: '生成性格描述失敗，請檢查 AI 後端設定。' },
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

  // --- Approvals ---
  'approvals.title': { en: 'Pending Approvals', zh: '待審批項目' },
  'approvals.empty': { en: 'No pending approvals', zh: '沒有待審批項目' },
  'approvals.reason': { en: 'Reason', zh: '原因' },
  'approvals.message': { en: 'Message', zh: '訊息' },
  'approvals.worker': { en: 'Worker', zh: '員工' },
  'approvals.task': { en: 'Task', zh: '任務' },
  'approvals.waiting': { en: 'Waiting', zh: '等待中' },
  'approvals.approve': { en: 'Approve', zh: '核准' },
  'approvals.deny': { en: 'Deny', zh: '拒絕' },

  // --- PRD Pipeline ---
  'prd.completed': { en: 'PRD Completed', zh: 'PRD 已完成' },
  'prd.approval': { en: 'PRD Approval Required', zh: '需要 PRD 核准' },
  'prd.viewDocument': { en: 'View PRD', zh: '查看 PRD' },

  // --- Dashboard Alerts ---
  'alerts.stuckWorkers': { en: 'Stuck Workers', zh: '卡住的員工' },
  'alerts.escalatedTasks': { en: 'Escalated Tasks', zh: '升級的任務' },
  'alerts.pendingApprovals': { en: 'Pending Approvals', zh: '待審批' },

  // --- Task Operations ---
  'task.reassign': { en: 'Reassign', zh: '重新分配' },
  'task.escalate': { en: 'Escalate', zh: '升級' },
  'task.markFailed': { en: 'Mark Failed', zh: '標記失敗' },
  'task.reassignConfirm': { en: 'Reassign this task?', zh: '重新分配此任務？' },
  'task.escalateConfirm': { en: 'Escalate this task?', zh: '升級此任務？' },
  'task.failConfirm': { en: 'Mark this task as failed?', zh: '標記此任務為失敗？' },

  // --- Review Queue ---
  'reviewQueue.drain': { en: 'Force Process', zh: '強制處理' },
  'reviewQueue.drainConfirm': { en: 'Force process all pending reviews?', zh: '強制處理所有待審查項目？' },

  // --- Worker Operations ---
  'workers.reset': { en: 'Reset', zh: '重設' },
  'workers.resetConfirm': { en: 'Reset worker {name} to idle?', zh: '重設員工 {name} 為閒置？' },
  'workerDetail.delete': { en: 'Delete Worker', zh: '刪除員工' },
  'workerDetail.deleteConfirm': { en: 'Delete worker {name}? This cannot be undone.', zh: '刪除員工 {name}？此操作無法復原。' },

  // --- Settings ---
  'settings.healthCheck': { en: 'Health Check', zh: '健康檢查' },
  'settings.runHealthCheck': { en: 'Run Health Check', zh: '執行健康檢查' },
  'settings.healthOk': { en: 'All systems healthy', zh: '所有系統正常' },
  'settings.version': { en: 'Version', zh: '版本' },
  'settings.checkUpdates': { en: 'Check for Updates', zh: '檢查更新' },
  'settings.upToDate': { en: 'You are on the latest version', zh: '已是最新版本' },
  'settings.updateAvailable': { en: 'New version available', zh: '有新版本可用' },
  'settings.download': { en: 'Download', zh: '下載' },
  'settings.checking': { en: 'Checking...', zh: '檢查中...' },
  'settings.skillsmpKey': { en: 'SkillsMP API Key', zh: 'SkillsMP API 金鑰' },
  'settings.skillsmpKeyHint': { en: 'Get free key at skillsmp.com/docs/api', zh: '在 skillsmp.com/docs/api 免費取得' },
  'settings.skillsmpKeySaved': { en: 'API Key saved', zh: 'API 金鑰已儲存' },

  // --- Board ---
  'board.escalation': { en: 'Escalation', zh: '升級' },

  // --- Objectives ---
  'nav.objectives': { en: 'Objectives', zh: '目標' },
  'objectives.title': { en: 'Company Objectives', zh: '公司目標' },
  'objectives.create': { en: '+ New Objective', zh: '+ 新增目標' },
  'objectives.empty': { en: 'No objectives yet. Create one to get started!', zh: '還沒有目標。建立一個開始吧！' },
  'objectives.titleLabel': { en: 'Title', zh: '標題' },
  'objectives.descLabel': { en: 'Description', zh: '描述' },
  'objectives.budgetLabel': { en: 'Token Budget Limit', zh: 'Token 預算上限' },
  'objectives.keyResults': { en: 'Key Results', zh: '關鍵結果' },
  'objectives.linkedProjects': { en: 'Projects', zh: '關聯專案' },
  'objectives.decompose': { en: 'AI Decompose', zh: 'AI 拆解' },
  'objectives.decomposing': { en: 'Decomposing...', zh: '拆解中...' },

  // --- Board Overview ---
  'nav.boardOverview': { en: 'Board', zh: '董事會' },
  'boardOverview.title': { en: 'Board Overview', zh: '董事會總覽' },
  'boardOverview.objectives': { en: 'Objectives', zh: '目標' },
  'boardOverview.projects': { en: 'Projects', zh: '專案' },
  'boardOverview.tasks': { en: 'Tasks', zh: '任務' },
  'boardOverview.workers': { en: 'Workers', zh: '員工' },
  'boardOverview.approvalRate': { en: 'Approval Rate', zh: '核准率' },
  'boardOverview.budget': { en: 'Monthly Budget', zh: '月度預算' },
  'boardOverview.tokensUsed': { en: 'Tokens Used', zh: 'Token 使用量' },
  'boardOverview.tasksThisMonth': { en: 'Tasks This Month', zh: '本月任務數' },
  'boardOverview.objectiveProgress': { en: 'Objective Progress', zh: '目標進度' },
  'boardOverview.performance': { en: 'Worker Performance', zh: '員工績效' },
  'boardOverview.worker': { en: 'Worker', zh: '員工' },
  'boardOverview.completed': { en: 'Completed', zh: '完成' },
  'boardOverview.failed': { en: 'Failed', zh: '失敗' },
  'boardOverview.approval': { en: 'Approval', zh: '核准' },
  'boardOverview.tokens': { en: 'Tokens', zh: 'Token' },

  // --- Worker Pause/Resume ---
  'workers.pause': { en: 'Pause', zh: '暫停' },
  'workers.resume': { en: 'Resume', zh: '恢復' },
  'workers.paused': { en: 'Paused', zh: '已暫停' },
  'workerDetail.titleLabel': { en: 'Title', zh: '職稱' },

  // --- Dashboard Budget ---
  'dashboard.budget': { en: 'Monthly Budget', zh: '月度預算' },
  'dashboard.tokensUsed': { en: 'Tokens Used', zh: 'Token 使用量' },
  'dashboard.objectiveProgress': { en: 'Objectives', zh: '目標進度' },

  // --- Setup Wizard ---
  'setup.welcome': { en: 'Welcome to AI Supervisor', zh: '歡迎使用 AI Supervisor' },
  'setup.languageSelect': { en: 'Select Language', zh: '選擇語言' },
  'setup.envCheck': { en: 'Environment Check', zh: '環境檢查' },
  'setup.teamSetup': { en: 'Team Setup', zh: '團隊設定' },
  'setup.complete': { en: 'Setup Complete', zh: '設定完成' },
  'setup.starterTeam': { en: 'Starter Team', zh: '入門團隊' },
  'setup.fullTeam': { en: 'Full Team', zh: '完整團隊' },
  'setup.customTeam': { en: 'Custom', zh: '自訂' },
  'setup.missingDep': { en: 'Missing dependency', zh: '缺少必要元件' },
  'setup.installGuide': { en: 'Install Guide', zh: '安裝指引' },
  'setup.recheck': { en: 'Recheck', zh: '重新檢查' },
  'setup.startUsing': { en: 'Start Using', zh: '開始使用' },
  'setup.workerCount': { en: 'Number of Workers', zh: '員工數量' },
  'setup.configureWorkers': { en: 'Configure Workers', zh: '配置員工' },
  'setup.creating': { en: 'Creating workers...', zh: '建立員工中...' },
  'setup.installAll': { en: 'Install All', zh: '一鍵全部安裝' },
  'setup.install': { en: 'Install', zh: '安裝' },
  'setup.installing': { en: 'Installing...', zh: '安裝中...' },
  'setup.installSuccess': { en: 'Installation complete', zh: '安裝完成' },
  'setup.installFailed': { en: 'Installation failed', zh: '安裝失敗' },
  'setup.depGitDesc': { en: 'Version control for branch management per task', zh: '版本控制，用於每個任務的分支管理' },
  'setup.depBrewDesc': { en: 'macOS package manager, required for installing tmux', zh: 'macOS 套件管理器，安裝 tmux 所需' },
  'setup.depTmuxDesc': { en: 'Terminal multiplexer for managing AI work sessions', zh: '終端多工器，管理 AI 工作階段' },
  'setup.depNodeDesc': { en: 'Required for installing Claude CLI', zh: 'Claude CLI 安裝所需' },
  'setup.depClaudeDesc': { en: 'AI programming assistant', zh: 'AI 程式助手' },
  'setup.downloading': { en: 'Downloading...', zh: '下載中...' },
  'setup.verifying': { en: 'Verifying...', zh: '驗證中...' },
  'setup.autoInstall': { en: 'Auto Install', zh: '自動安裝' },
  'setup.installed': { en: 'Installed', zh: '已安裝' },
  'setup.missing': { en: 'Missing', zh: '未安裝' },
  'setup.needsNode': { en: 'Requires Node.js', zh: '需先安裝 Node.js' },
  'setup.needsBrew': { en: 'Requires Homebrew', zh: '需先安裝 Homebrew' },

  // --- Agentic Training ---
  'settings.agenticTraining': { en: 'Agentic Training', zh: '自主訓練' },
  'settings.agenticEnabled': { en: 'Enable Agentic Loop', zh: '啟用自主迭代' },
  'settings.maxIterations': { en: 'Max Iterations', zh: '最大迭代次數' },
  'settings.defaultTestCmd': { en: 'Default Test Command', zh: '預設測試指令' },
  'settings.autoRollback': { en: 'Auto Rollback', zh: '自動回退' },
  'taskForm.typeTraining': { en: 'Training', zh: '訓練' },
  'taskForm.testCmd': { en: 'Test Command', zh: '測試指令' },
  'taskForm.maxIter': { en: 'Max Iterations', zh: '最大迭代' },
  'taskForm.passThreshold': { en: 'Pass Threshold', zh: '通過分數' },
  'training.iteration': { en: 'Iteration {n}/{total}', zh: '第 {n}/{total} 輪' },
  'training.improved': { en: 'Improved', zh: '有進步' },
  'training.rolledBack': { en: 'Rolled Back', zh: '已回退' },
  'training.bestScore': { en: 'Best Score', zh: '最佳分數' },

  // --- Worker Activity Feed ---
  'activity.title': { en: 'Worker Activity', zh: '員工活動' },
  'activity.noOutput': { en: 'No output captured', zh: '尚未捕捉到輸出' },
  'activity.noActive': { en: 'No active workers', zh: '沒有正在工作的員工' },

  // --- Task Timeline ---
  'timeline.created': { en: 'Created', zh: '已建立' },
  'timeline.assigned': { en: 'Assigned', zh: '已分配' },
  'timeline.inProgress': { en: 'Working', zh: '進行中' },
  'timeline.review': { en: 'Review', zh: '審查' },
  'timeline.done': { en: 'Done', zh: '完成' },
  'timeline.waitTime': { en: 'Wait', zh: '等待' },
  'timeline.workTime': { en: 'Work', zh: '工時' },
  'timeline.rejections': { en: 'Rejections', zh: '退回次數' },
  'timeline.retries': { en: 'Retries', zh: '重試次數' },

  // --- Recovery Events ---
  'alerts.recovered': { en: 'Auto-Recovered', zh: '已自動恢復' },
  'alerts.recoveryFailed': { en: 'Recovery Failed', zh: '恢復失敗' },
  'alerts.reviewTimeout': { en: 'Review Timeout', zh: '審查超時' },

  // --- Verification & Iteration ---
  'verify.passed': { en: 'Verification Passed', zh: '驗證通過' },
  'verify.failed': { en: 'Verification Failed', zh: '驗證失敗' },
  'verify.rollback': { en: 'Rolled Back', zh: '已回退' },
  'verify.retry': { en: 'Retrying', zh: '重試中' },
  'verify.plateau': { en: 'Plateau (Early Stop)', zh: '平台期（提前停止）' },
  'verify.iteration': { en: 'Iteration', zh: '迭代' },
  'verify.score': { en: 'Score', zh: '分數' },
  'verify.bestScore': { en: 'Best Score', zh: '最佳分數' },

  // --- Setup Wizard: Onboarding Chat (Step 3) ---
  'setup.chatPlaceholder': { en: 'Tell me what you\'d like to do...', zh: '說說你想做什麼...' },
  'setup.assistantName': { en: 'Assistant', zh: '小助理' },
  'setup.hiringHR': { en: 'Hiring HR...', zh: '正在招募 HR...' },
  'setup.hrHired': { en: '{name} has joined the team!', zh: '{name} 已加入團隊！' },
  'setup.confirmTeam': { en: 'Confirm Team', zh: '確認建立團隊' },
  'setup.buildingTeam': { en: 'Building team...', zh: '建立團隊中...' },
  'setup.recommendedTeam': { en: 'Recommended Team', zh: '推薦團隊' },
  'setup.chatError': { en: 'Assistant temporarily unavailable', zh: '助理暫時無法回應' },
  'setup.apiKeyHint': { en: 'Enter an API key to enable the AI assistant. You can use Claude CLI login or provide a key below.', zh: '請輸入 API 金鑰以啟用 AI 助理。你也可以使用 Claude CLI 登入，或在下方提供金鑰。' },
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
