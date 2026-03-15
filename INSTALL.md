# AI Supervisor 安裝手冊

## 使用者安裝指南

### 系統需求

- macOS 13.0 (Ventura) 以上
- Apple Silicon (M1/M2/M3/M4) 或 Intel Mac

### 從 .dmg 安裝

1. 下載 `aisupervisor-<version>.dmg`
2. 雙擊開啟 DMG
3. 將 `aisupervisor.app` 拖到 `Applications` 資料夾
4. 首次開啟時，macOS 會提示「無法驗證開發者」：
   - 到 **系統設定 → 隱私與安全性**，點擊「仍要打開」
   - 或右鍵 app → 「打開」

### 首次設定 (Setup Wizard)

App 首次啟動會進入設定精靈：

**Step 1 — 語言選擇**
- 選擇繁體中文或 English

**Step 2 — 依賴檢查**
- 自動檢查並安裝必要元件：
  - **Git** — 版本控制（通常已內建）
  - **Homebrew** — macOS 套件管理
  - **tmux** — 終端多工器（AI worker 執行環境）
  - **Node.js** — Claude CLI 的前提
  - **Claude CLI** — AI 程式助手
- 未安裝的元件可點擊「安裝全部」自動安裝
- Claude CLI 安裝後需執行 `claude login` 完成登入

**Step 3 — 團隊建立**
- AI 助理會透過對話了解你的需求
- 推薦適合的 AI 團隊配置
- 送出訊息：點擊「送出」按鈕，或按 `Cmd+Enter`
- 確認團隊後自動建立所有 worker

**Step 4 — 完成**
- 顯示已建立的團隊
- 點擊「開始使用」進入主畫面

### Claude CLI 登入

如果 Step 3 提示輸入 API Key，可改為使用 Claude CLI 登入：

```bash
# 在終端機執行
claude login
```

登入完成後重新啟動 app，即可自動使用 Claude CLI。

### 支援的 AI 後端

| 後端 | 說明 | 設定方式 |
|------|------|----------|
| Claude CLI | 推薦，使用 Claude Code 帳號 | `claude login` |
| Anthropic API | Claude API 直接呼叫 | 在 Step 3 輸入 API Key |
| OpenAI API | GPT 系列模型 | 在 Step 3 輸入 API Key |
| Gemini API | Google Gemini 模型 | 在 Step 3 輸入 API Key |
| Ollama | 本地模型（進階） | 安裝 Ollama + 模型 |

---

## 開發者 Build 指南

### 開發環境需求

- Go 1.23+
- Node.js 18+
- npm
- [Wails v2](https://wails.io/) CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 專案結構

```
cmd/aisupervisor-gui/    # Wails GUI 入口
frontend/                # Svelte + Vite 前端
internal/                # Go 後端邏輯
```

### 開發模式

```bash
# 啟動開發伺服器（前端 HMR + Go 後端）
make dev-gui

# 開發 URL
# App:      http://localhost:34115
# Frontend: http://localhost:41229
```

### Build 指令

```bash
# 完整 build（前端 + Go + .app bundle）
make package-mac

# Build + 安裝到指定目錄 + ad-hoc 簽名
make install-mac

# 安裝到自訂路徑
make install-mac INSTALL_DIR=/Applications

# 只編譯前端
make frontend-build

# 只編譯 Go（開發用，跳過前端）
make build-gui-full
```

### Build 流程說明

```
make package-mac
  ├── make frontend-build     # npm run build → cmd/aisupervisor-gui/frontend/dist/
  └── wails build -s          # -s = 跳過前端編譯（已自己做了）
       ├── go build           # 編譯 Go + embed 前端資源
       └── package .app       # 打包 macOS app bundle
```

> **重要**：Wails 有時會 cache 前端不重新編譯。`package-mac` 已設定先強制編譯前端（`frontend-build` 依賴），確保每次都用最新程式碼。

### 打包 DMG

```bash
# 需要先安裝 create-dmg
brew install create-dmg

# 打包
make package-dmg
```

### 正式發布流程

```bash
# 完整流程：build → bundle deps → 簽名 → DMG → 公證
make release
```

需要：
- Apple Developer ID 憑證（codesign）
- App Store Connect API Key（notarize）
- `create-dmg` 工具

### 測試

```bash
make test          # 完整測試
make test-short    # 快速測試
go vet ./...       # 靜態檢查
```

### 常見問題

**Q: App 啟動後白屏或「無法打開」**
- 確認前端有編譯：`ls cmd/aisupervisor-gui/frontend/dist/index.html`
- 如果只有 `.gitkeep`，執行 `make frontend-build` 再重新 build

**Q: 修改前端後 app 沒有更新**
- Wails 可能用了 cache。確保使用 `make package-mac`（會強制重編前端）
- 或手動：`cd frontend && npm run build` 再 `make package-mac`

**Q: Claude CLI 在 app 中找不到**
- App 從 Finder 啟動時 PATH 很小。程式會自動搜尋以下路徑：
  - `~/.local/bin/claude`
  - `~/.claude/local/bin/claude`
  - `/usr/local/bin/claude`
  - `/opt/homebrew/bin/claude`
  - nvm/volta/fnm 管理的路徑
- 如果 Claude CLI 安裝在非標準路徑，請建立 symlink：`ln -s /path/to/claude ~/.local/bin/claude`

**Q: 「OAuth authentication is currently not supported」錯誤**
- Claude Code 的 OAuth token 不能直接用於 API 呼叫
- 程式會自動 fallback 到 Claude CLI（`claude -p`）
- 或在 Step 3 輸入 API Key
