# AI Supervisor — Docker 開發環境使用手冊

在 Docker 容器中執行 aisupervisor，讓 worker（Claude Code CLI）在隔離的沙盒環境內擁有完整讀寫與執行權限，同時與本機系統隔離。

---

## 目錄

1. [架構總覽](#架構總覽)
2. [前置需求](#前置需求)
3. [首次部署](#首次部署)
4. [啟動與停止](#啟動與停止)
5. [Volume 掛載說明](#volume-掛載說明)
6. [專案管理與路徑對應](#專案管理與路徑對應)
7. [傳遞檔案、圖片與網址給 Worker](#傳遞檔案圖片與網址給-worker)
8. [環境變數](#環境變數)
9. [AI Backend 設定](#ai-backend-設定)
10. [容器內常用操作](#容器內常用操作)
11. [aisupervisor CLI 指令參考](#aisupervisor-cli-指令參考)
12. [進階用法](#進階用法)
13. [疑難排解](#疑難排解)
14. [檔案結構](#檔案結構)

---

## 架構總覽

```
┌─ 本機 (macOS / Linux) ───────────────────────────────────────┐
│                                                               │
│  ~/.claude/            ──(唯讀掛載)──┐                        │
│  ~/.config/aisupervisor/ ──(讀寫)────┤                        │
│  ~/.local/share/aisupervisor/ ─(讀寫)┤                        │
│  ~/Projects/           ──(讀寫)──────┤                        │
│                                      ▼                        │
│  ┌─ Docker Container (debian:bookworm-slim) ───────────────┐  │
│  │                                                         │  │
│  │  aisupervisor CLI          (Go binary)                  │  │
│  │  ├── tmux server                                        │  │
│  │  │   └── worker sessions   (Claude Code CLI)            │  │
│  │  ├── Node.js 20 LTS                                     │  │
│  │  ├── Claude Code CLI       (@anthropic-ai/claude-code)  │  │
│  │  └── git 2.39                                           │  │
│  │                                                         │  │
│  │  /workspace        ← ~/Projects                         │  │
│  │  /root/.claude     ← ~/.claude (ro)                     │  │
│  │  /root/.config/aisupervisor ← 設定檔                    │  │
│  │  /root/.local/share/aisupervisor ← 資料/審計紀錄        │  │
│  │                                                         │  │
│  │  OPENAI_API_KEY ──→ gpt-4o-mini (supervisor backend)    │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

**設計理念：**
- Worker（Claude Code）在容器內有完整權限（bypassPermissions），可自由讀寫 `/workspace` 下的程式碼
- Supervisor 的 AI 判斷透過 OpenAI gpt-4o-mini API，不需要本機 GPU 或 Ollama
- Claude Code 認證透過掛載 `~/.claude/`（唯讀），不需額外設定
- 容器銷毀後，程式碼變更保留在本機 `~/Projects/`

---

## 前置需求

| 需求 | 版本 | 確認方式 |
|------|------|----------|
| Docker Desktop | >= 4.0 | `docker --version` |
| Docker Compose | v2 (內建於 Docker Desktop) | `docker compose version` |
| OpenAI API Key | — | [platform.openai.com/api-keys](https://platform.openai.com/api-keys) |
| Claude Code 認證 | — | 本機 `~/.claude/` 目錄存在 |

### 檢查前置條件

```bash
# 確認 Docker 正在執行
docker info > /dev/null 2>&1 && echo "Docker OK" || echo "Docker 未啟動"

# 確認 Claude 認證存在
ls ~/.claude/ > /dev/null 2>&1 && echo "Claude 認證 OK" || echo "請先在本機執行 claude 完成認證"

# 確認設定檔存在
ls ~/.config/aisupervisor/config.yaml > /dev/null 2>&1 && echo "設定檔 OK" || echo "請先建立設定檔"
```

---

## 首次部署

### Step 1：設定 OpenAI API Key

```bash
# 加入 shell 設定檔（擇一）
echo 'export OPENAI_API_KEY="sk-proj-你的key"' >> ~/.zshrc
source ~/.zshrc

# 或者只在當前 session 設定
export OPENAI_API_KEY="sk-proj-你的key"
```

### Step 2：確認 config.yaml 已設定 openai-mini backend

`~/.config/aisupervisor/config.yaml` 中應包含：

```yaml
backends:
  - name: openai-mini
    type: openai
    api_key_env: OPENAI_API_KEY
    model: gpt-4o-mini
```

如果要讓 Docker 內預設使用此 backend，將 `default_backend` 改為：

```yaml
default_backend: openai-mini
```

### Step 3：構建 Docker Image

```bash
cd aisupervisor/
docker compose build
```

首次構建約需 3-5 分鐘（下載 base images + 編譯 Go + 安裝 Node.js + Claude Code CLI）。後續重複構建會利用快取，通常在 10 秒內完成。

**構建產物：**
- Image 名稱：`aisupervisor-aisupervisor:latest`
- Image 大小：約 845 MB
- 包含：Go binary + Node.js 20 + Claude Code CLI + tmux + git

### Step 4：驗證構建

```bash
# 確認 image 存在
docker images aisupervisor-aisupervisor

# 快速驗證所有工具
docker compose run --rm aisupervisor bash -c "
  aisupervisor --help > /dev/null && echo '✓ aisupervisor' &&
  claude --version > /dev/null && echo '✓ claude code' &&
  tmux -V > /dev/null && echo '✓ tmux' &&
  git --version > /dev/null && echo '✓ git' &&
  node --version > /dev/null && echo '✓ node'
"
```

---

## 啟動與停止

### 互動模式（推薦初次使用）

```bash
# 啟動容器，預設執行 aisupervisor company
docker compose run --rm aisupervisor

# 啟動容器，進入 bash shell
docker compose run --rm aisupervisor bash

# 啟動容器，執行指定指令
docker compose run --rm aisupervisor aisupervisor monitor --backend openai-mini --tui
```

- `--rm`：容器退出後自動刪除
- 按 `Ctrl+C` 或輸入 `exit` 退出

### 背景模式（長時間執行）

```bash
# 背景啟動
docker compose up -d

# 查看容器狀態
docker compose ps

# 查看即時 log
docker compose logs -f

# 進入正在執行的容器
docker exec -it aisupervisor-dev bash

# 停止容器
docker compose down
```

### 重新啟動

```bash
# 停止並重新啟動
docker compose restart

# 完全重建（程式碼有變更時）
docker compose down
docker compose build --no-cache
docker compose up -d
```

---

## Volume 掛載說明

| 本機路徑 | 容器路徑 | 權限 | 用途 |
|----------|----------|------|------|
| `~/.claude/` | `/root/.claude/` | **唯讀** | Claude Code OAuth 認證 token |
| `~/.config/aisupervisor/` | `/root/.config/aisupervisor/` | 讀寫 | config.yaml 設定檔 |
| `~/.local/share/aisupervisor/` | `/root/.local/share/aisupervisor/` | 讀寫 | 審計紀錄、context store、訓練資料 |
| `~/Projects/` | `/workspace/` | 讀寫 | 專案原始碼（worker 工作目錄） |

**注意事項：**
- `~/.claude/` 設為唯讀，防止容器意外修改認證資料
- `/workspace/` 的變更會**直接反映到本機** `~/Projects/`，請謹慎操作
- 如果需要掛載其他目錄，在 `docker-compose.yml` 的 `volumes` 區段新增

---

## 專案管理與路徑對應

### 專案不需要移動到容器內

本機 `~/Projects/` 透過 volume mount 直接映射到容器的 `/workspace/`，所有檔案即時同步：

```
本機                              容器
~/Projects/my-app/        ←→     /workspace/my-app/
~/Projects/api-server/    ←→     /workspace/api-server/
~/Projects/docs/          ←→     /workspace/docs/
```

你不需要複製或移動任何專案。在本機修改的檔案，容器內立即可見；worker 在容器內的變更，也會立即反映到本機。

### 建立 Project — 自動解析路徑（推薦）

`--repo` 為選填參數。省略時，aisupervisor 會自動在 workspace 目錄中尋找與 `--name` 相符的子目錄（大小寫不敏感）：

```bash
# 自動解析：只需 --name，自動找到 /workspace/my-app
aisupervisor company create-project --name "my-app"
# → Auto-resolved repo path: /workspace/my-app

# 手動指定：仍然可以用 --repo 覆蓋
aisupervisor company create-project --name "My App" --repo "/workspace/my-app"

# 找不到時會列出可用目錄
aisupervisor company create-project --name "nonexistent"
# → Error: no directory matching "nonexistent" in /workspace
# → available: my-app, api-server, docs
```

**自動解析的搜尋順序：**

1. 環境變數 `WORKSPACE_DIR`（若有設定）
2. `/workspace/`（Docker 容器內預設）
3. `~/Projects/`（本機環境 fallback）

可透過設定 `WORKSPACE_DIR` 環境變數來覆蓋預設的 workspace 目錄：

```bash
# 使用自訂 workspace 目錄
WORKSPACE_DIR=/other-repos aisupervisor company create-project --name "my-lib"
```

### 路徑對應速查表

| 用途 | 路徑 | 說明 |
|------|------|------|
| 建立 project 的 `--repo`（自動） | 自動解析為 `/workspace/<name>` | 省略 `--repo` 即可 |
| 建立 project 的 `--repo`（手動） | `/workspace/<project-name>` | 容器內路徑 |
| Worker 工作目錄 | `/workspace/<project-name>` | 自動 cd 到此 |
| Git 操作目錄 | `/workspace/<project-name>` | worker 在此建立 branch |
| 本機查看變更 | `~/Projects/<project-name>` | 即時同步 |

### 新增非 ~/Projects 下的專案

如果需要讓 worker 存取 `~/Projects` 以外的目錄，在 `docker-compose.yml` 新增掛載：

```yaml
volumes:
  - ~/Projects:/workspace
  - ~/other-repos:/other-repos    # 新增掛載點
```

然後用 `/other-repos/<project-name>` 作為 `--repo` 路徑。

---

## 傳遞檔案、圖片與網址給 Worker

### 通訊機制說明

Worker（Claude Code CLI）在容器內的 tmux session 中執行。aisupervisor 透過 `tmux send-keys` 將 **純文字 prompt** 傳送給 Claude Code，無法直接傳送二進位檔案或多媒體內容。

但 Claude Code CLI 本身具備強大的工具能力，可以透過以下方式間接存取各類資源：

### 傳遞檔案

將檔案放到專案目錄中（`~/Projects/<project>/` 下），worker 即可透過 Claude Code 的 `Read` 工具讀取：

```bash
# 本機操作：將參考檔案放到專案目錄
cp ~/Documents/spec.pdf ~/Projects/my-app/docs/
cp ~/Downloads/data.json ~/Projects/my-app/reference/

# 在 task description 中指示 worker
"請參考 docs/spec.pdf 的規格書來實作 API endpoint"
"讀取 reference/data.json 了解資料格式"
```

### 傳遞圖片（設計稿、截圖、Mockup）

Claude Code 支援多模態（multimodal），可直接讀取圖片檔案：

```bash
# 本機操作：將圖片放到專案目錄
cp ~/Desktop/login-mockup.png ~/Projects/my-app/docs/designs/
cp ~/Desktop/bug-screenshot.png ~/Projects/my-app/docs/issues/

# 在 task description 中指示 worker
"參考 docs/designs/login-mockup.png 的設計稿，實作登入頁面的 UI"
"查看 docs/issues/bug-screenshot.png 的截圖，修復顯示異常"
```

**支援格式：** PNG、JPG、GIF、WebP

### 傳遞網址

Claude Code 有 `WebFetch` 和 `WebSearch` 工具，worker 可以抓取公開網頁的內容：

```bash
# 在 task description 中直接附上網址
"API 規格文件在 https://api.example.com/docs ，請依此實作 client SDK"
"參考 https://github.com/example/lib 的 README 來整合這個套件"
"UI 設計參考 https://dribbble.com/shots/xxxxx 的風格"
```

**限制：**
- 只能存取**公開**網頁（無法存取需要登入的頁面）
- 需要 worker 的 skill profile 允許 `WebFetch` 工具

### 完整範例：帶有多種參考資料的 Task

```bash
# 1. 準備參考資料到專案目錄
cp ~/Desktop/design-v2.png ~/Projects/my-app/docs/
cp ~/Documents/api-spec.yaml ~/Projects/my-app/docs/

# 2. 建立任務時，在 description 中引用所有資源
aisupervisor company create-task \
  --project "my-app" \
  --title "實作使用者個人頁面" \
  --description "
    需求：
    1. 參考 docs/design-v2.png 的設計稿實作頁面 UI
    2. API 格式依照 docs/api-spec.yaml
    3. 第三方元件參考 https://ui.shadcn.com/docs/components/avatar
    4. 確保通過 docs/test-cases.md 中列出的所有測試案例
  "
```

### 資源傳遞方式摘要

| 資源類型 | 傳遞方式 | Worker 如何存取 |
|----------|----------|----------------|
| 原始碼 | 已在 `/workspace` 掛載中 | `Read`、`Grep`、`Glob` 工具 |
| 文件 (PDF/TXT/YAML) | 放到專案目錄 | `Read` 工具 |
| 圖片 (PNG/JPG) | 放到專案目錄 | `Read` 工具（multimodal） |
| 公開網址 | 寫在 task description | `WebFetch` 工具 |
| 私有網址 | 先下載內容到專案目錄 | `Read` 工具 |
| 大型資料集 | 放到專案目錄或另開掛載點 | `Read`、`Bash` 工具 |

---

## 環境變數

| 變數 | 必要性 | 說明 |
|------|--------|------|
| `OPENAI_API_KEY` | **必要**（使用 openai-mini backend 時） | OpenAI API key，從本機環境變數傳入 |
| `ANTHROPIC_API_KEY` | 選用 | Anthropic API key（若使用 claude-api backend） |

環境變數從本機 shell 透過 `docker-compose.yml` 的 `${OPENAI_API_KEY}` 語法傳入容器。**API key 不會寫入任何檔案或 image 中。**

如需新增環境變數，編輯 `docker-compose.yml`：

```yaml
environment:
  - OPENAI_API_KEY=${OPENAI_API_KEY}
  - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}  # 新增這行
```

---

## AI Backend 設定

aisupervisor 支援多種 AI backend，在 `config.yaml` 的 `backends` 區段設定：

| Backend | Type | 適用場景 | 成本 |
|---------|------|----------|------|
| `openai-mini` | `openai` | Docker 環境推薦，低成本高穩定性 | ~$0.15-0.60/1M tokens |
| `claude-oauth` | `anthropic_oauth` | 本機使用，需要 `~/.claude/` 認證 | 依方案 |
| `claude-api` | `anthropic_api` | 需要 ANTHROPIC_API_KEY | 依方案 |

### 切換 Backend

```bash
# 方式一：修改 config.yaml 的 default_backend
default_backend: openai-mini

# 方式二：啟動時用 --backend 參數覆蓋
aisupervisor monitor --backend openai-mini --tui
```

### 自動驗證管線設定

在 `config.yaml` 中加入 `verification` 區段，啟用任務完成後的自動化驗證（lint/build/test + 安全掃描）：

```yaml
verification:
  enabled: true
  docker_image: "golang:1.22"   # 驗證用 Docker image
  timeout_sec: 300              # 驗證超時秒數
  lint_cmd: "make lint"         # 自訂 lint 指令
  build_cmd: "make build"       # 自訂建構指令
  test_cmd: "make test"         # 自訂測試指令
```

### 人類介入控制設定

在 `config.yaml` 中加入 `human_gate` 區段，控制何時需要人類審核：

```yaml
human_gate:
  enabled: true
  token_budget_threshold: 1000000  # token 消耗超過此值需人類確認
  require_deploy_approval: true    # 部署需人類核准
  confidence_floor: 0.3            # 信心度下限
```

---

## 容器內常用操作

### aisupervisor 操作

```bash
# 公司管理模式（管理 projects、workers、tasks）
aisupervisor company

# TUI 監控模式（即時觀察 worker 活動）
aisupervisor monitor --backend openai-mini --tui

# 查看當前設定
aisupervisor config show

# 查看審計紀錄
aisupervisor audit list
```

### Claude Code 操作

```bash
# 啟動 Claude Code（完整權限模式）
claude --permission-mode full

# 在指定專案目錄執行
cd /workspace/my-project
claude
```

### tmux 操作（管理 worker sessions）

```bash
# 列出所有 tmux sessions
tmux list-sessions

# 附加到 worker session 觀察其操作
tmux attach -t <session-name>

# 從 tmux session 中脫離（不中斷 worker）
# 按 Ctrl+B 然後按 D

# 手動建立新 session
tmux new-session -d -s my-worker

# 終止指定 session
tmux kill-session -t <session-name>
```

### Git 操作

```bash
# 容器內 git 已自動設定：
# - user.name = "aisupervisor-docker"
# - user.email = "aisupervisor@docker.local"
# - safe.directory = * (信任所有掛載目錄)

# 查看 worker 的程式碼變更
cd /workspace/my-project
git status
git diff
git log --oneline -10
```

---

## aisupervisor CLI 指令參考

| 指令 | 說明 |
|------|------|
| `aisupervisor company` | 互動式公司管理（projects / workers / tasks） |
| `aisupervisor monitor --tui` | 啟動 TUI 監控 dashboard |
| `aisupervisor monitor --backend <name>` | 指定 AI backend 啟動監控 |
| `aisupervisor monitor --dry-run` | 偵測模式（只分析不操作） |
| `aisupervisor monitor --session <name>` | 只監控指定 tmux session |
| `aisupervisor config show` | 顯示當前設定 |
| `aisupervisor sessions list` | 列出被監控的 sessions |
| `aisupervisor backends list` | 列出可用的 AI backends |
| `aisupervisor roles list` | 列出 supervisor 角色 |
| `aisupervisor audit list` | 查看審計紀錄 |
| `aisupervisor training export` | 匯出訓練資料 |

---

## 進階用法

### 自訂掛載目錄

如果專案不在 `~/Projects/`，編輯 `docker-compose.yml`：

```yaml
volumes:
  - /path/to/your/repos:/workspace
```

### 多容器同時執行

```bash
# 啟動第二個容器（需指定不同 container_name）
docker compose run --rm --name aisupervisor-dev-2 aisupervisor bash
```

### 重建 Image（程式碼變更後）

```bash
# 只重建有變更的 layer（快速）
docker compose build

# 完全重建（排除快取）
docker compose build --no-cache
```

### Docker Desktop Proxy 問題

如果你的 Docker Desktop 啟用了 HTTP Proxy（常見於企業環境），構建時可能遇到網路錯誤。
本專案的 Dockerfile 已在每個 `RUN` 指令前加上 `HTTPS_PROXY="" HTTP_PROXY=""` 來繞過此問題。

如果仍然失敗，可嘗試：

```bash
# 在 Docker Desktop Settings > Resources > Proxies 中關閉 Manual proxy configuration
# 或使用以下方式構建：
docker build --build-arg http_proxy="" --build-arg https_proxy="" -t aisupervisor-aisupervisor .
```

---

## 疑難排解

### 構建失敗：`dial tcp 127.0.0.1:443: connect: connection refused`

**原因：** Docker Desktop 的 HTTP Proxy 攔截了網路請求。

**解法：** Dockerfile 中已處理。如果仍失敗，檢查 Docker Desktop > Settings > Resources > Proxies。

### 容器啟動顯示 `OPENAI_API_KEY: NOT SET`

**原因：** 本機環境變數未設定。

**解法：**
```bash
export OPENAI_API_KEY="sk-proj-你的key"
docker compose run --rm aisupervisor
```

### Claude Code 無法認證

**原因：** `~/.claude/` 目錄為空或認證已過期。

**解法：** 在本機先執行 `claude` 完成認證，確認 `~/.claude/` 內有認證檔案。

### tmux 相關錯誤

**原因：** tmux server 可能未正確啟動。

**解法：**
```bash
# 在容器內手動啟動
tmux start-server
tmux list-sessions
```

### 容器內看不到專案檔案

**原因：** `~/Projects/` 路徑不存在或為空。

**解法：** 確認本機 `~/Projects/` 目錄存在且包含專案，或修改 `docker-compose.yml` 中的掛載路徑。

### 需要完全清除重來

```bash
# 停止並移除容器
docker compose down

# 移除 image
docker rmi aisupervisor-aisupervisor

# 清除構建快取
docker builder prune

# 重新構建
docker compose build --no-cache
```

---

## 檔案結構

```
aisupervisor/
├── Dockerfile              # 多階段構建（Go 編譯 + Runtime 環境）
├── docker-compose.yml      # 一鍵啟動，Volume 掛載與環境變數
├── .dockerignore           # 構建時排除的檔案
└── docker/
    ├── entrypoint.sh       # 容器啟動腳本（tmux + git + 環境檢查）
    └── README.md           # 本文件
```

### Dockerfile 構建流程

```
Stage 1 (builder):  golang:1.25-bookworm
  → go mod download
  → go build → /aisupervisor binary

Stage 2 (runtime):  debian:bookworm-slim
  → apt-get: tmux, git, curl, ca-certificates, openssh-client, gnupg
  → Node.js 20 LTS (nodesource)
  → npm install -g @anthropic-ai/claude-code
  → COPY /aisupervisor binary
  → COPY entrypoint.sh
  → ENTRYPOINT ["/entrypoint.sh"]
```
