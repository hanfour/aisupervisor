# Council Review Engine — 統一審查管線設計

> 日期: 2026-04-22
> 狀態: Approved
> 靈感來源: [Carmack-Council](https://github.com/SamJHudson01/Carmack-Council)

## 概要

借鑒 Carmack-Council 的「專家委員會」模式，對 aisupervisor 的審查管線進行統一重構。核心改進：

1. **Phase 0 自動化預審** — 機械驗證在 AI 審查之前
2. **Context Brief** — 統一上下文系統，取代散落的 prompt 組裝
3. **Expert Registry** — 10 個領域專家，動態選擇
4. **Council Engine** — 多專家平行審查，混合 API/CLI 執行模式
5. **Synthesis + Carmack Filter** — 去重、經濟性過濾、AI 合成
6. **Conventions 學習系統** — 跨 session 知識累積

交付方式：單一 feature branch，一次到位。

---

## 1. Phase 0：自動化預審檢查

### 新檔案: `internal/company/phase0.go`

在 `review.go` 的 `executeReview` 入口處新增 Phase 0。

```go
type Phase0Check struct {
    Name    string           // "typecheck", "lint", "test"
    Command string           // 從 project.VerifyCmd 或自動偵測
    Timeout time.Duration
}

type Phase0Result struct {
    Check   Phase0Check
    Passed  bool
    Output  string           // truncated to 2000 chars
    Elapsed time.Duration
}

type Phase0Report struct {
    Results  []Phase0Result
    AllGreen bool
    Summary  string          // 人類可讀摘要
}
```

### 執行邏輯

1. 從 `project.VerifyCmd` 解析指令，外加自動偵測（`package.json` → `npm run lint`、`go.mod` → `go vet ./...`）
2. 在 worktree 目錄下**平行執行**（`sync.WaitGroup`）
3. 任一失敗 → 產生 `Finding{Severity: "CRITICAL", Source: "phase0"}`
4. 全部綠燈 → 附加到 Context Brief
5. **全部失敗** → 跳過 AI 審查，直接 reject

### 與現有系統的關係

- 擴展 `project.VerifyCmd` 的使用場景（目前僅在 `verifyAndIterate` 中使用）
- Phase 0 findings 帶 `Source: "phase0"` 標記，合成階段不與 AI findings 去重衝突

### 新增 Event

- `EventPhase0Completed`

---

## 2. Context Brief 統一上下文系統

### 新檔案: `internal/company/context_brief.go`

取代目前散落在 `buildPromptForTier`、`buildKarpathyOverlay`、`runChatReview` 中的多處 prompt 組裝。

```go
type ContextBrief struct {
    TaskID      string
    ProjectID   string
    GeneratedAt time.Time
    Sections    []BriefSection
    FilePath    string          // 暫存檔路徑（CLI 模式用）
}

type BriefSection struct {
    Name        string          // "project_summary", "architecture", "diff_stats",
                                // "phase0_results", "conventions", "graph_context",
                                // "rejection_history", "karpathy_overlay"
    Content     string
    TokenBudget int             // 該區塊最大 token 數
    Priority    int             // 衝突時低優先級被截斷
}

type ContextBriefBuilder struct {
    totalBudget   int           // 預設 6000 tokens (~24000 chars)
    graph         *knowledge.CodeGraph
    injector      *knowledge.Injector
    conventions   *ConventionStore
    task          *project.Task
    project       *project.Project
    phase0        *Phase0Report
    diff          string
}

func (b *ContextBriefBuilder) Build() (*ContextBrief, error)
```

### Build 流程（各區塊預算）

| 區塊 | Token 預算 | Priority | 來源 |
|------|-----------|----------|------|
| Project Summary | 500 | 1 | `project.Project` |
| Diff Stats | 200 | 1 | `getDiffStats` |
| Architecture Context | 1500 | 2 | L2 知識注入 + GRAPH_REPORT 社群/God Node |
| Phase 0 Results | 800 | 1 | `Phase0Report` |
| Conventions | 1000 | 3 | `ConventionStore.FindRelevant()` |
| Rejection History | 500 | 4 | `compressRejectionHistory` |
| Karpathy Overlay | 500 | 4 | `buildKarpathyOverlay` |

### Token 預算管理

- 各區塊先按 `TokenBudget` 截斷
- 總和超過 `totalBudget` → 從最低 `Priority` 開始壓縮（保留前 30% + 後 30%，中間省略）
- 永遠不截斷 priority 1 區塊

### 檔案式通訊

- Brief 渲染成 Markdown 後寫入 `{worktree}/.ais-review/context-brief.md`
- API call 模式：直接放進 system message
- CLI 子代理模式：子代理啟動後第一步讀取該檔案

### 與現有系統的關係

- 取代 `buildPromptForTier` 中散落的知識注入
- 取代 `runChatReview` 中的 `pkbContext` 組裝
- `buildKarpathyOverlay` 和 `compressRejectionHistory` 成為 builder 的私有方法
- **審查和任務 spawn 都使用同一個 builder**

---

## 3. Expert Registry 與動態選擇

### 新檔案: `internal/company/expert.go`

```go
type ExpertDomain string

const (
    DomainSecurity     ExpertDomain = "security"
    DomainPerformance  ExpertDomain = "performance"
    DomainRefactoring  ExpertDomain = "refactoring"
    DomainTesting      ExpertDomain = "testing"
    DomainFrontend     ExpertDomain = "frontend"
    DomainBackend      ExpertDomain = "backend"
    DomainDatabase     ExpertDomain = "database"
    DomainAPI          ExpertDomain = "api_contract"
    DomainConcurrency  ExpertDomain = "concurrency"
    DomainArchitecture ExpertDomain = "architecture"
)

type Expert struct {
    Domain       ExpertDomain
    Name         string        // 報告歸因名稱
    SystemPrompt string        // 領域專屬審查指引
    Severity     []string      // 可發出的 severity 等級
    FilePatterns []string      // glob 觸發，如 "*.go", "*.svelte"
    Keywords     []string      // 內容關鍵字觸發
    Model        string        // 建議模型（空=預設）
}

type SelectedExpert struct {
    Expert
    AssignedFiles []string
    Reason        string
    Mode          ExecMode     // "api" 或 "cli"
}

type ExecMode string
const (
    ExecAPI ExecMode = "api"
    ExecCLI ExecMode = "cli"
)
```

### 動態選擇演算法 `SelectExperts`

| 規則 | 觸發條件 |
|------|----------|
| **必選** | Phase 0 失敗 → 選對應領域專家 |
| **檔案模式** | 變更檔案 match `FilePatterns` |
| **關鍵字** | diff 內容含 `Keywords` |
| **God Node** | 變更檔案是 god node → 強制加 `DomainArchitecture` |
| **跨社群** | 涉及 3+ communities → 強制加 `DomainArchitecture` |
| **最低保障** | 至少 2 個專家 |
| **上限** | 最多 5 個（成本控制） |

### 執行模式判定

```
AssignedFiles ≤ 3 且 diffLines ≤ 100 → ExecAPI
否則 → ExecCLI
```

### 預設專家組（10 個）

| Domain | 核心關注 | FilePatterns | Keywords |
|--------|----------|-------------|----------|
| security | OWASP, injection, auth | `*` | `password`, `token`, `crypto`, `exec` |
| performance | O(n²), memory, bundle | `*` | `loop`, `append`, `goroutine`, `cache` |
| refactoring | DRY, naming, complexity | `*` | — |
| testing | coverage, mock hygiene | `*_test.go`, `*.test.*` | `mock`, `assert`, `expect` |
| frontend | 元件品質, a11y, CSS | `*.svelte`, `*.tsx`, `*.css` | `bind:`, `$:`, `dispatch` |
| backend | error handling, goroutine | `*.go` | `goroutine`, `chan`, `context` |
| database | N+1, migration, indexing | `*.sql`, `*.prisma` | `SELECT`, `INSERT`, `migration` |
| api_contract | 破壞性變更, 版本相容 | `*.go`, `*.proto` | `json:`, `yaml:`, `Handler` |
| concurrency | 競態, deadlock, channel | `*.go` | `sync.`, `Mutex`, `chan `, `atomic` |
| architecture | 模組邊界, 依賴方向 | `*` | — |

### 替換關係

- `shouldEscalateReview` 邏輯吸收進 `SelectExperts`
- `selectStrategy` (Light/Standard/Debate) 被替換：策略隱含在專家數量和模式中

---

## 4. Council Review Engine

### 新檔案: `internal/company/council.go`

```go
type CouncilEngine struct {
    chatProvider  ai.ChatProvider
    spawner       *worker.Spawner
    tmuxClient    tmux.TmuxClient
    registry      *ExpertRegistry
    conventions   *ConventionStore
    graph         *knowledge.CodeGraph
    language      string
    reviewCfg     config.ReviewConfig
}

type CouncilRequest struct {
    Task      *project.Task
    Project   *project.Project
    Diff      string
    DiffLines int
    FileCount int
    Brief     *ContextBrief
    Phase0    *Phase0Report
}

type ExpertFinding struct {
    Finding                      // 嵌入現有 Finding
    Expert     ExpertDomain
    Principle  string            // 依據的原則（歸因）
    Confidence float64           // 0.0-1.0
}

type CouncilResult struct {
    Status      string           // "APPROVED" 或 "CHANGES_REQUESTED"
    Summary     string
    Findings    []ExpertFinding
    ExpertCount int
    Phase0      *Phase0Report
    Duration    time.Duration
    TokensUsed  int64
}
```

### 主流程 `RunCouncil`

```
Phase 0 (已完成) → Phase 1 (diff 分析) → Phase 2 (brief 已備)
→ Phase 3 (SelectExperts) → Phase 4 (dispatchExperts) → Phase 5 (synthesis)
```

### 兩種執行模式

#### API 模式 `runExpertAPI`

- system = expert.SystemPrompt + brief.Content，user = AssignedFiles 的 diff 片段
- `ChatWithModelOrFallback(ctx, cp, messages, expert.Model)`
- 超時 60 秒
- ~1 API call / 專家

#### CLI 模式 `spawnExpertAgent`

- 暫存 tmux session `ais-review-{domain}-{timestamp}`
- Claude Code CLI 帶：
  - `--append-system-prompt` = expert.SystemPrompt
  - `--dangerously-skip-permissions`
  - `--disallowedTools Edit,Write,NotebookEdit,Bash`（**唯讀**）
- 等待完成後擷取 pane 輸出，解析 JSON
- 超時 5 分鐘，完成後 kill session

### 平行控制

- `errgroup.WithContext()` + `SetLimit(5)`
- 單一專家失敗不中止整體（記錄 warning，繼續）

### 新增 Events

- `EventCouncilStarted` — 附帶選中專家列表
- `EventExpertCompleted` — 每個專家完成
- `EventCouncilSynthesized` — 合成完成

---

## 5. 結果合成、去重與 Carmack Filter

### 新檔案: `internal/company/synthesis.go`

### 三步合成流程

#### Step 1: 機械去重 `mergeCouncilFindings`

| 條件 | 處理 |
|------|------|
| 同 file:line | 保留最高 severity；同級保留最高 confidence |
| 同 file 不同 line，body 相似度 > 80% | 合併，line 取較早的 |
| 不同 file 但 body 幾乎相同 | 保留兩個 |

**領域優先權**（同 file:line 衝突時）：

```go
var domainPriority = map[domainPair]ExpertDomain{
    {"security", "backend"}:      "security",
    {"security", "api_contract"}: "security",
    {"concurrency", "backend"}:   "concurrency",
    {"frontend", "refactoring"}:  "frontend",
    {"testing", "refactoring"}:   "testing",
    {"architecture", "backend"}:  "architecture",
    {"architecture", "frontend"}: "architecture",
    {"database", "backend"}:      "database",
    {"database", "performance"}:  "database",
}
```

#### Step 2: Carmack Filter `applyCarmackFilter`

```go
type CarmackFilterConfig struct {
    MaxFindings     int           // 預設 15
    ProjectScale    ProjectScale  // small / medium / large
    ConventionStore *ConventionStore
}

type ProjectScale string  // "small" (<50 files), "medium" (50-500), "large" (>500)
```

過濾規則：

1. **Conventions 過濾** — finding 指出的問題已被 conventions 記載為已知模式 → 移除
2. **規模過濾** — Small: 移除 MEDIUM+performance；Medium: MEDIUM 上限 5 個
3. **重複模式壓縮** — 3+ 個同類 finding → 合併為 1 個，列出所有位置
4. **上限截斷** — 超過 MaxFindings 按 severity 排序取 top N

#### Step 3: AI 合成 `synthesizeFindings`

- 單次 API call：確認無矛盾、寫 Summary、決定 Status
- 裁決規則：
  - 0 個 CRITICAL/HIGH → APPROVED
  - 任何 CRITICAL → CHANGES_REQUESTED
  - ≥3 HIGH → CHANGES_REQUESTED
  - 1-2 HIGH → APPROVED + 標註建議改善
- **快速路徑**（跳過 AI）：
  - 0 findings → 直接 APPROVED
  - 全 MEDIUM → 直接 APPROVED + 列出建議
  - Phase0 CRITICAL → 直接 CHANGES_REQUESTED

### 替換關係

- `mergeFindings()` → `mergeCouncilFindings()`（擴展支援 N 來源 + 領域優先權）
- `tallyVotes()` → 移除（Carmack Filter 取代投票）
- `runSynthesis()` → `synthesizeFindings()`（更豐富輸入）
- `severityRank()` → 保留共用

---

## 6. Conventions 學習系統

### 新檔案: `internal/company/conventions.go`

### 儲存結構

```
~/.local/share/aisupervisor/company/
├── conventions/
│   ├── index.yaml              ← YAML metadata 索引
│   └── conventions.md          ← 人類可讀主檔
```

```go
type Convention struct {
    ID          string       `yaml:"id"`
    Domain      ExpertDomain `yaml:"domain"`
    Pattern     string       `yaml:"pattern"`       // 機器可比對
    Description string       `yaml:"description"`   // 人類可讀
    FileGlob    string       `yaml:"file_glob"`
    Source      string       `yaml:"source"`         // "review:{taskID}"
    AcceptedAt  time.Time    `yaml:"accepted_at"`
    AcceptCount int          `yaml:"accept_count"`
    LastUsed    time.Time    `yaml:"last_used"`
}

type ConventionStore struct {
    mu          sync.RWMutex
    conventions []Convention
    indexPath   string
    mdPath      string
    idSeq       int
}
```

### 核心方法

```go
func NewConventionStore(dataDir string) (*ConventionStore, error)
func (cs *ConventionStore) Save() error
func (cs *ConventionStore) FindRelevant(domain ExpertDomain, filePath string) []Convention
func (cs *ConventionStore) MatchesFinding(f ExpertFinding) *Convention
func (cs *ConventionStore) Propose(conv Convention) string
func (cs *ConventionStore) Accept(id string)
func (cs *ConventionStore) Decay(maxAge time.Duration, minUses int)
```

### 漸進式學習流程

| 場景 | 行為 |
|------|------|
| 第一次出現新模式 | 記為 `proposed`，不過濾 |
| 第二次同模式被 approved | 自動升級為 accepted，開始過濾 |
| 連續 30 天未匹配 | `Decay()` 移除 |
| 人類手動編輯 conventions.md | 下次 `Save()` 時 index.yaml 同步更新 |

### 整合點

| 整合點 | 用途 |
|--------|------|
| Carmack Filter | `MatchesFinding()` 過濾已知模式 |
| Context Brief | `FindRelevant()` 注入相關 conventions |
| Worker Spawn | 相關 conventions 注入 system prompt |
| Knowledge Graph | FileGlob 可用 community 擴展 |
| Personality | 遵循 conventions → 加 Reliability |
| Growth Engine | 學到新 convention → 額外 EXP |

### 新增 Events

- `EventConventionProposed`
- `EventConventionAccepted`
- `EventConventionDecayed`

---

## 7. 整合架構

### review.go 重寫後完整流程

```
executeReview(req)
→ Phase 0: runPhase0Checks()           [phase0.go]
→ Phase 1: getDiffStats()              [review.go 現有]
→ Phase 2: ContextBriefBuilder.Build() [context_brief.go]
→ Phase 3: registry.SelectExperts()    [expert.go]
→ Phase 4: council.dispatchExperts()   [council.go]
→ Phase 5: merge + filter + synthesize [synthesis.go]
→ Phase 6: handleCouncilResult()       [review.go]
          + learnFromReview()          [conventions.go]
```

### Manager 新增欄位

```go
type Manager struct {
    // ... 現有 ...
    council      *CouncilEngine
    conventions  *ConventionStore
    expertReg    *ExpertRegistry
}
```

### Fallback 策略

- Council 引擎失敗 → 降級到現有 `runChatReview`（debate 系統完整保留）
- debate 也失敗 → 降級到 `fallbackTmuxReview`

### handleCouncilResult

- `CouncilResult` 轉換為 `DebateResult` 格式
- 複用現有 `handleDebateResult`（personality、growth、reject/approve 邏輯不動）
- Council 特有：APPROVED 後觸發 `learnFromReview`

### Worker Spawn 整合

- `spawner.go` 的 `buildPromptForTier` 改用 `ContextBriefBuilder`
- Spawner 新增 `conventions *ConventionStore` 欄位

### Config 擴展

```go
type ReviewConfig struct {
    // ... 現有 ...
    CouncilEnabled     bool   `yaml:"council_enabled"`       // 預設 true
    MaxExperts         int    `yaml:"max_experts"`            // 預設 5
    Phase0Enabled      bool   `yaml:"phase0_enabled"`         // 預設 true
    CarmackFilterScale string `yaml:"carmack_filter_scale"`   // "auto"/"small"/"medium"/"large"
    CLIExpertTimeout   int    `yaml:"cli_expert_timeout_s"`   // 預設 300
    APIExpertTimeout   int    `yaml:"api_expert_timeout_s"`   // 預設 60
    ConventionDecayDays int   `yaml:"convention_decay_days"`  // 預設 30
}
```

---

## 檔案清單

### 新增（6 個）

| 檔案 | 職責 | 預估行數 |
|------|------|---------|
| `internal/company/phase0.go` | Phase 0 自動檢查 | ~150 |
| `internal/company/context_brief.go` | Context Brief + token 預算 | ~250 |
| `internal/company/expert.go` | Expert Registry + 動態選擇 | ~300 |
| `internal/company/council.go` | Council Engine + 派遣 | ~400 |
| `internal/company/synthesis.go` | 去重 + Carmack Filter + AI 合成 | ~300 |
| `internal/company/conventions.go` | Conventions 學習系統 | ~350 |

### 修改（7 個）

| 檔案 | 改動 |
|------|------|
| `internal/company/review.go` | `executeReview` 路由到 council；保留 fallback |
| `internal/company/company.go` | Manager 新增 3 欄位 + New() 初始化 |
| `internal/company/events.go` | 新增 6 個 EventType |
| `internal/company/debate.go` | 保留完整作為 fallback，不刪除 |
| `internal/worker/spawner.go` | `buildPromptForTier` 改用 ContextBriefBuilder |
| `internal/config/config.go` | ReviewConfig 擴展 |
| `frontend/src/lib/stores/i18n.js` | 新 event 中文翻譯 |

### 不動

- `debate.go` 全部函式（保留作為 fallback）
- `personality/`（透過 handleDebateResult 自動觸發）
- `growth/`（同上）
- `knowledge/`（只讀取）
- `tmux/`（council CLI 模式用現有 client）
