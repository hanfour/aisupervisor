# AI Supervisor GUI 操作手冊

AI Supervisor 是一套 8-bit 復古風格的 AI 公司管理系統，使用 Wails (Go) + Svelte 4 + NES.css 打造。本手冊涵蓋所有 GUI 頁面的功能與操作方式。

---

## 目錄

1. [啟動應用程式](#1-啟動應用程式)
2. [Dashboard（儀表板）](#2-dashboard儀表板)
3. [Projects（專案管理）](#3-projects專案管理)
4. [Board（看板）](#4-board看板)
5. [Workers（員工管理）](#5-workers員工管理)
6. [Hierarchy（階層視圖）](#6-hierarchy階層視圖)
7. [Terminal（終端機）](#7-terminal終端機)
8. [Roles（角色管理）](#8-roles角色管理)
9. [Groups（群組討論）](#9-groups群組討論)
10. [Settings（設定）](#10-settings設定)

---

## 1. 啟動應用程式

### 前置需求
- Go 1.21+
- Node.js 18+
- tmux（需在背景執行）
- Wails CLI v2（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）

### 啟動方式

**開發模式**（前端即時更新）：
```bash
cd cmd/aisupervisor-gui
~/go/bin/wails dev
```

**正式建構**：
```bash
cd cmd/aisupervisor-gui
~/go/bin/wails build
./build/bin/aisupervisor-gui
```

### 啟動參數
| 參數 | 說明 | 範例 |
|------|------|------|
| `-config` | 指定設定檔路徑 | `-config ~/.config/aisupervisor/config.yaml` |
| `-session` | 監控特定 tmux session | `-session mysession:0.0` |
| `-backend` | 覆蓋預設 AI 後端 | `-backend anthropic` |
| `-dry-run` | 偵測與決策但不送出按鍵 | `-dry-run` |

---

## 2. Dashboard（儀表板）

![Dashboard](screenshots/01-dashboard.png)

Dashboard 是應用程式的首頁，提供公司整體運作狀態的即時概覽。

### Company 統計卡片
頂部顯示 5 張統計卡片：
- **Projects** — 目前專案總數
- **In Progress** — 進行中的專案數量
- **Idle Workers** — 閒置員工數量
- **Reviews Pending** — 待審核的 code review 數量
- **Training Pairs** — 訓練資料對數

這些數據會在任何公司事件發生時自動刷新。

### Review Queue
顯示目前待審核的 review 列表，以表格呈現：
- Task（任務名稱）
- Project（所屬專案）
- Engineer（負責工程師）
- Manager（審核管理者）
- Created（建立時間）

若無待審項目，顯示「No pending reviews」。

### Training Stats
顯示模型訓練資料的統計：
- **Total Pairs** — 訓練對數總量
- **Accepted** — 已接受的對數（綠色）
- **Rejected** — 已拒絕的對數（紅色）
- **Approval Rate** — 通過率進度條
  - 80%+ 綠色、50-80% 黃色、<50% 紅色

### Sessions
列出所有被監控的 tmux sessions。點擊任一 session 卡片可跳轉至 Terminal 頁面查看詳情。

### Events
底部的事件日誌表格，顯示最近 200 筆 Supervisor 和 Company 事件。欄位：
- Time（時間戳）
- Type（事件類型，如 DETECT、DECIDE、HIRE、COMMIT 等）
- Source（來源）
- Detail（詳細內容）

### 低信心確認對話框
當 Supervisor 偵測到低信心度的決策（低於設定的 threshold）時，會自動彈出確認對話框，顯示：
- Session 名稱
- 建議動作
- 理由與信心百分比
- **Approve** / **Dismiss** 按鈕

---

## 3. Projects（專案管理）

![Projects](screenshots/05-projects.png)

管理 AI 公司中的所有軟體專案。

### 建立新專案
1. 點擊 **+ New Project** 按鈕
2. 填寫表單：
   - **Name** — 專案名稱
   - **Description** — 專案描述
   - **Repo Path** — Git 倉庫路徑
   - **Base Branch** — 基礎分支（如 `main`）
   - **Goals** — 專案目標（每行一個）
3. 點擊 **Create** 送出

### 專案列表
每張專案卡片顯示：
- 專案名稱與狀態
- 描述
- 點擊卡片可進入 Board 看板頁面

---

## 4. Board（看板）

看板頁面以 Kanban 風格展示指定專案的任務流程。

### 看板欄位
任務看板依據完整生命週期狀態機顯示，欄位如下：

| 欄位 | 包含狀態 | 說明 |
|------|----------|------|
| **Backlog** | `backlog`, `draft` | 尚未就緒的任務草稿 |
| **Spec Review** | `spec_review`, `approved` | 規格審查與核准 |
| **Ready** | `ready`, `assigned` | 已就緒、待分配或已分配的任務 |
| **In Progress** | `in_progress` | 進行中的任務 |
| **Review** | `code_review`, `testing`, `security_scan` | 程式碼審核、測試與安全掃描 |
| **Staging** | `staging`, `accepted` | 預備部署與已接受 |
| **Done** | `done`, `deployed` | 已完成或已部署 |
| **Revision** | `revision` | 被退回修改的任務 |
| **Escalation** | `failed` | 失敗/升級處理的任務 |

#### 狀態流程
```
backlog → draft → spec_review → approved → ready → assigned → in_progress
  → code_review → testing → security_scan → staging → accepted → done → deployed
```
- 任何審核階段可退回至 `revision`（附退回原因）
- 退回次數 ≥ 3 次觸發自動升級（escalation）
- 簡單任務可跳過中間階段：`backlog → ready`（向後相容）

#### 死循環偵測
系統內建 Circuit Breaker 機制：
- 同一對 agent 來回退回上限：3 次
- 總退回次數上限：6 次
- 超過上限時自動升級給 Consultant 層級重新拆解

### 建立任務
1. 點擊 **+ New Task** 按鈕
2. 填寫表單：
   - **Title** — 任務標題
   - **Description** — 任務描述
   - **Prompt** — 給 Claude Code 的指令
   - **Priority** — 優先級（1-9，1 最高）
   - **Milestone** — 里程碑（選填）
   - **Dependencies** — 依賴的前置任務
3. 點擊 **Create** 送出

### 任務操作
- **Assign**（在 Ready 狀態）— 將任務指派給閒置的 Worker
- **Advance**（在各審核階段）— 推進任務到下一階段
- **Reject**（在審核階段）— 退回任務並附上原因
- **Done**（在 Accepted 狀態）— 標記任務完成
- **Deploy**（在 Done 狀態）— 標記任務已部署（需人類核准，見 Human Gate）
- 任務卡片顯示優先級 badge（P1-P3）、分配者、分支狀態、退回次數

---

## 5. Workers（員工管理）

![Workers - 階層視圖](screenshots/02-workers-hierarchy.png)

Workers 頁面以 3 欄階層視圖展示所有 AI 員工。

### 三層架構
| 欄位 | 說明 |
|------|------|
| **Consultants** | 最高層級，負責策略與最終決策 |
| **Managers** | 中間層級，負責 code review 和任務管理 |
| **Engineers** | 執行層級，負責實際撰寫程式碼 |

每欄標題顯示 tier 名稱與該層人數。

### Worker 卡片
每張卡片包含：
- **Avatar** — NES.css 像素角色圖示
- **Name** — 員工名稱
- **Status Badge** — 狀態指示
  - 🟢 idle（閒置）
  - 🔵 working（工作中）
  - 🟡 waiting（等待中）
  - 🔴 error（錯誤）
- **Current Task** — 目前執行的任務 ID
- **Role Badge** — 職務角色（見下方角色說明）
- **Parent Link** — 上級管理者名稱（如「↑ Manager: Alice」）
- **Skill Scores** — 技能分數摘要（懸停可查看詳情）
- **Promote 按鈕** — 將員工升級到下一個 tier

### Worker 角色（Role）
每個 Worker 除了 tier（Consultant / Manager / Engineer）外，還有職務角色：

| 角色 | 說明 | 負責階段 |
|------|------|----------|
| `architect` | 架構師 | spec_review 階段 |
| `coder` | 程式開發（預設） | in_progress 階段 |
| `qa` | 品質保證 | testing 階段 |
| `security` | 安全審核 | security_scan 階段 |
| `devops` | 運維部署 | staging 階段 |
| `designer` | UI/UX 設計 | 設計相關任務 |

一個 Worker 可同時具備 SkillProfile（如 `hacker`）和 Role（如 `security`）。各階段的自動分配依 Role 匹配。未設定 Role 的 Worker 預設為 `coder`。

### 技能分數系統
系統追蹤每個 Worker 的 6 維度技能分數（0-100，預設 50）：

| 維度 | 說明 |
|------|------|
| Carefulness | 細心程度 |
| BoundaryChecking | 邊界檢查意識 |
| TestCoverageAware | 測試覆蓋意識 |
| CommunicationClarity | 溝通清晰度 |
| CodeQuality | 程式碼品質 |
| SecurityAwareness | 安全意識 |

- 分數會依事件自動調整（如審核被退回 → 相關分數降低）
- 每完成 10 個任務，所有分數向 50 回歸 10%（衰減機制）
- 分數 < 40 時系統自動在 Worker prompt 中注入警告指引
- 分數 > 80 時注入正面強化

### 點擊 Worker → Log Panel
點擊任一 Worker 卡片會彈出 Log Panel 對話框：
- 85vw x 80vh 的大型終端機風格視窗
- 即時顯示該 Worker 的執行日誌
- 搜尋過濾功能
- 可調整 scrollback 行數（100-1000）
- 每 1.5 秒自動刷新

### 招募新員工

![Hire Worker Dialog](screenshots/03-hire-worker-dialog.png)

1. 點擊 **+ Hire Worker** 按鈕
2. 填寫表單：
   - **Name** — 員工名稱
   - **Avatar** — 選擇像素角色（Robot, Kirby, Mario, Ash, Bulbasaur, Charmander, Squirtle, Pokeball）
   - **Tier** — 選擇層級（Consultant / Manager / Engineer）
   - **Parent (Manager)** — 選擇上級管理者（下拉選單列出所有 Consultant 和 Manager）
   - **Role** — 選擇職務角色（Architect / Coder / QA / Security / DevOps / Designer）
   - **CLI Tool** — 選擇使用的 AI 工具（Claude / Codex / Gemini）
   - **Backend ID** — 選填，指定後端模型 ID（如 `gpt-4`）
3. 點擊 **Hire** 完成招募

### 模型選配策略
系統依以下優先順序自動選擇 AI 模型：
1. Worker 個人設定的 `ModelVersion`
2. 任務類型覆寫（如 research 類任務使用 Opus）
3. SkillProfile 設定的模型
4. Tier 預設（Consultant/Manager → Opus, Engineer → Sonnet）

### 升級員工
- Engineer → Manager → Consultant
- 在 Worker 卡片上點擊 **Promote** 按鈕即可升級
- Consultant 已是最高層級，不顯示 Promote 按鈕

---

## 6. Hierarchy（階層視圖）

![Hierarchy](screenshots/04-hierarchy-page.png)

Hierarchy 頁面是專門的全頁階層視覺化頁面，提供更詳細的公司組織結構。

### 階層欄位
以三欄橫向排列，中間有箭頭（→）表示指揮鏈：

| 欄位 | 圖示 | 顏色 |
|------|------|------|
| Consultants | ★ | 黃色 |
| Managers | ♦ | 藍色 |
| Engineers | ⚙ | 綠色 |

每個 tier 都有彩色邊框的 badge 標頭，顯示圖示、名稱和人數。

### Worker 卡片
與 Workers 頁面相同，但額外顯示：
- **Tier Badge** — 以 tier 顏色標示 `[consultant]` / `[manager]` / `[engineer]`
- **Parent Name** — 顯示上級名稱（如「↑ Alice」）

點擊卡片同樣可以開啟 Log Panel。

### 底部面板
頁面底部並排顯示兩個面板：
- **Review Queue** — 與 Dashboard 相同的待審核列表
- **Training Stats** — 與 Dashboard 相同的訓練統計

---

## 7. Terminal（終端機）

![Terminal](screenshots/06-terminal.png)

Terminal 頁面顯示特定 tmux session 的詳細資訊。

### 導航
- 從 Dashboard 點擊 session 卡片進入
- 點擊 **< Back** 按鈕返回 Dashboard

### 資訊區
- **tmux** — 顯示 session:window.pane 格式
- **tool** — 使用的工具類型

### Session Events
顯示該 session 的事件歷程（偵測、決策、送出等）。

---

## 8. Roles（角色管理）

管理 Supervisor 的 AI 決策角色。

### 角色系統
每個角色有不同的職責和決策模式：
- **Gatekeeper** — 把關者，判斷是否批准動作
- **Manager** — 管理者，策略級決策
- 自訂角色 — 透過 config 或 `~/.config/aisupervisor/roles/` 目錄載入

### Session 角色指派
1. 選擇一個 Session
2. 勾選要啟用的角色
3. 角色會以其 mode（observe / intervene）和 priority 參與決策

---

## 9. Groups（群組討論）

當偵測到需要多角色討論的情境時，Groups 頁面顯示 AI 群組討論的完整流程。

### 三階段討論
1. **Opinion** — 各角色提出初步意見
2. **Roundtable** — 圓桌討論，角色之間交換看法
3. **Decision** — 最終決策

### 討論訊息
每則訊息包含：
- 角色名稱與圖示
- 信心度百分比（紅 <50% / 黃 50-80% / 綠 80%+）
- 階段標籤
- 建議動作 badge

---

## 10. Settings（設定）

![Settings](screenshots/07-settings.png)

唯讀顯示目前的 Supervisor 設定值。

### Polling（輪詢）
- **Interval (ms)** — 輪詢間隔，預設 500ms
- **Context Lines** — 每次讀取的上下文行數，預設 100

### Decision（決策）
- **Confidence Threshold** — 信心度門檻，低於此值需人工確認，預設 0.7
- **Timeout (s)** — 決策超時秒數，預設 30

### Context（上下文記憶）
- **Enabled** — 是否啟用上下文記憶
- **Max Decisions** — 記憶的最大決策數，預設 20
- **Token Budget** — Token 預算，預設 2000

### Verification（自動驗證管線）
- **Enabled** — 是否啟用自動化驗證
- **Docker Image** — 執行驗證的 Docker image（預設 `golang:1.22`）
- **Timeout (s)** — 驗證超時秒數（預設 300）
- **Lint Command** — 自訂 lint 指令
- **Build Command** — 自訂建構指令
- **Test Command** — 自訂測試指令

啟用後，任務進入 `code_review` 前會自動執行 lint/build/test 驗證。驗證失敗會直接回饋給 Engineer（不經 Manager），節省 token。通過後進入 Manager code review，再通過後執行靜態安全掃描（semgrep/gosec）。

### Human Gate（人類介入控制）
- **Enabled** — 是否啟用人類介入機制
- **Token Budget Threshold** — token 消耗超過此閾值需人類確認
- **Require Deploy Approval** — `staging → deployed` 是否需人類核准
- **Confidence Floor** — 信心度下限

觸發 Human Gate 的情境：
- 部署至 production（`staging → deployed`）
- Circuit Breaker 觸發升級
- 累計 token 消耗超過閾值
- `git push --force` 偵測
- 破壞性 DB schema 變更

當 Human Gate 被觸發時，GUI 會彈出待審核通知，任務暫停直到人類回覆。

### Backends
列出已設定的 AI 後端（Anthropic、OpenAI、Gemini、Ollama）。

### Auto-Approve Rules
列出自動批准規則，符合規則的動作無需人工確認。

---

## 側邊欄導航

左側的側邊欄提供快速導航，包含 9 個頁面入口：

| 圖示 | 頁面 | 說明 |
|------|------|------|
| ⊞ | Dashboard | 總覽儀表板 |
| ◈ | Projects | 專案管理 |
| ▦ | Board | 任務看板 |
| ☺ | Workers | 員工管理（階層視圖） |
| ⊿ | Hierarchy | 公司階層視覺化 |
| ⊟ | Terminal | 終端機詳情 |
| ★ | Roles | 角色管理 |
| ♦ | Groups | 群組討論 |
| ⚙ | Settings | 系統設定 |

目前所在的頁面會以綠色邊框高亮顯示。

---

## 即時事件系統

所有頁面透過 Wails Runtime Events 接收即時更新：

| 事件 | 說明 |
|------|------|
| `company:event` | 公司事件（專案、任務、員工變動） |
| `company:human_gate` | 人類介入請求通知（需人類審核的事件） |
| `supervisor:error` | Supervisor 錯誤通知 |
| `supervisor:event` | Supervisor 事件（偵測、決策） |
| `discussion:event` | 群組討論事件 |

### 新增事件類型
v2 版新增以下 company 事件類型：

| 事件類型 | 說明 |
|----------|------|
| `spec_review_started` | 規格審查開始 |
| `spec_approved` | 規格審查通過 |
| `testing_started` | 自動測試開始 |
| `testing_passed` | 自動測試通過 |
| `security_scan_start` | 安全掃描開始 |
| `security_passed` | 安全掃描通過 |
| `staging_started` | 預備部署開始 |
| `staging_accepted` | 預備部署接受 |
| `task_escalated` | 任務被升級（循環偵測觸發） |
| `task_deployed` | 任務已部署 |
| `human_intervention_required` | 需要人類介入 |

事件現在支援結構化訊息（StructuredMessage），包含發送者、接收者、優先級、上下文引用等欄位。

事件觸發時，相關的 store 會自動重新載入資料，確保 UI 始終顯示最新狀態。錯誤事件會以 Toast 通知的方式在右上角短暫顯示。

---

## 鍵盤快捷鍵

| 按鍵 | 功能 |
|------|------|
| `Enter` | 觸發選取的 sidebar 項目 |
| `Escape` | 關閉任何開啟的 dialog |

---

## 技術架構

```
┌─────────────────────────────────────────────┐
│              Wails Desktop App              │
├──────────────┬──────────────────────────────┤
│  Go Backend  │      Svelte Frontend         │
│              │                              │
│  CompanyApp  │  Stores (reactive)           │
│  - Workers   │  - workers.js + hierarchy    │
│  - Projects  │  - company.js + reviewQueue  │
│  - Tasks     │  - sessions.js              │
│  - Hierarchy │  - events.js                │
│  - Reviews   │  - roles.js                 │
│  - Training  │  - discussions.js           │
│              │                              │
│  App         │  Pages                       │
│  - Sessions  │  - DashboardPage            │
│  - Roles     │  - WorkersPage              │
│  - Groups    │  - HierarchyPage            │
│  - Events    │  - ProjectsPage             │
│              │  - ProjectBoardPage          │
│  tmux ←→ AI  │  - TerminalPage             │
│              │  - RolesPage                 │
│              │  - GroupsPage               │
│              │  - SettingsPage             │
├──────────────┴──────────────────────────────┤
│          NES.css (8-bit Retro Theme)        │
└─────────────────────────────────────────────┘
```
