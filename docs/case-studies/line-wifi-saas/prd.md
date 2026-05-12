# PRD：LINE Wi-Fi 精準廣告平台

| 欄位       | 內容                                                     |
|-----------|---------------------------------------------------------|
| 文件版本   | 1.0                                                     |
| 建立日期   | 2026-04-27                                              |
| 專案名稱   | LINE Wi-Fi 精準廣告平台（LINE Wi-Fi Ad SaaS）            |
| 產品代號   | LWAP（LINE Wi-Fi Ad Platform）                          |
| 負責人     | Product Team                                            |
| 狀態       | Draft（MVP 規劃階段）                                   |

---

## 0. 文件導讀

本 PRD 針對「以 LINE 官方帳號 + Chatbot + LIFF 為核心，整合店家 Wi-Fi 登入流程」的 SaaS 平台。MVP 目標為：可在 3–5 家實體店面（餐飲、咖啡、零售）落地驗證「Wi-Fi 連線 → 加入官方帳號 → 廣告觸達 → 行為紀錄 → 推播再行銷」完整閉環。

讀者建議閱讀順序：第 1 章（背景與目標） → 第 2 章（市場分析） → 第 3 章（用戶與場景） → 第 4 章（功能需求） → 第 8 章（建議任務）。

---

## 1. 產品概述

### 1.1 背景

實體店家（餐飲、咖啡、零售、美業）長期以 SSID + 密碼方式提供免費 Wi-Fi，缺乏：

1. **第一方數據資產**：無法掌握到店人次、回訪率、客群輪廓
2. **再行銷管道**：客人離店即斷聯，無法主動推播優惠
3. **跨店分析能力**：連鎖品牌無法整合分店流量做總部決策

LINE 在台灣月活躍用戶超過 2,100 萬，覆蓋率超過 90%，是台灣最普及的通訊與生活服務入口。透過 LINE Login + LIFF + Messaging API，可在「使用者連 Wi-Fi」這個高動機場景，自然完成「加好友 → 取得授權 → 累積行為 → 後續推播」的完整 funnel，而成本顯著低於 Beacon/POS 整合方案。

### 1.2 產品定位

> 「店家 Wi-Fi 即廣告位，LINE 即 CRM」——一套讓中小型實體店家以 Wi-Fi 為入口，3 分鐘上手 LINE 數位行銷的 SaaS 工具。

### 1.3 商業目標（北極星指標）

| 指標                          | MVP（6 個月）目標 | 第二年目標         |
|-------------------------------|-------------------|--------------------|
| 簽約店家數                    | 30                | 500                |
| 月活躍 Wi-Fi 連線使用者（MAU）| 5,000             | 200,000            |
| 廣告 CTR（LIFF 內 banner）    | ≥ 8%              | ≥ 12%              |
| 推播訊息開信率                | ≥ 40%             | ≥ 50%              |
| 月經常性收入（MRR）           | NT$ 60,000        | NT$ 1,500,000      |

### 1.4 產品目標（功能層）

1. 開發 LIFF 前端，串接 Wi-Fi 登入流程，展示廣告並記錄行為
2. 設計支援多租戶（Tenant）的後端，承載店家管理、用戶行為、推播
3. 建置店家管理後台（Web），可設定廣告、推播、叫號
4. 整合 LINE Messaging API 進行推播與自動化通知
5. 核心資料庫支援用戶**跨店**行為紀錄與精準分析
6. 在 3–5 家實體場域完成 MVP 驗證

---

## 2. 市場分析

### 2.1 目標市場規模（TAM/SAM/SOM）

- **TAM**（台灣中小型實體服務業）：約 70 萬家
- **SAM**（提供 Wi-Fi 且有 LINE 官方帳號之店家）：約 12 萬家
- **SOM**（MVP 12 個月可觸及，月費 NT$ 990–2,990 區間）：約 3,000 家

### 2.2 競品分析

| 產品                         | 定位             | 優勢                          | 劣勢                                |
|------------------------------|------------------|-------------------------------|-------------------------------------|
| **Cisco Meraki + Splash Page** | 企業級 Wi-Fi   | 硬體穩定、授權機制成熟        | 硬體成本高、缺乏 LINE 整合          |
| **CHT Hami Wi-Fi**           | 電信 Wi-Fi 點   | 點位多、品牌信任              | 不開放 SaaS、無法客製               |
| **Vpon DMP**                 | 行動廣告        | 大數據能力強                  | 依賴第三方 cookie，2026 年起受限    |
| **iCHEF / 簡單記**           | 餐飲 POS        | POS 滲透高、有會員系統        | 不主打 Wi-Fi，無 LIFF 廣告位        |
| **LINE 官方帳號（純）**      | 訊息推播        | 用戶覆蓋廣                    | 無 Wi-Fi 入口、無 funnel 設計       |
| **本產品**                   | LINE × Wi-Fi SaaS | LINE 觸達 + 行為數據 + 自助化 | 品牌新、需教育市場                  |

### 2.3 差異化定位

1. **入口優勢**：Wi-Fi 是顧客「主動」要連的，動機強、不擾人，相較被動掃 QR Code 的轉換率高 3–5 倍（業界經驗值）
2. **數據主權**：店家擁有第一方行為數據，不依賴第三方 cookie
3. **跨店圖譜**：同一 LINE userId 在不同分店的行為可被串接，連鎖品牌看得到「客人也常去誰家店」
4. **自助上架**：店家自行於後台上架廣告，毋須業務介入

### 2.4 風險與假設

- **假設一**：LINE 仍維持 Messaging API + LIFF 開放政策（風險：LINE 政策變動，2024 已調整訊息計費）
- **假設二**：店家願意開放 Wi-Fi DHCP/Captive Portal 設定（風險：使用消費者級路由器之店家配置複雜度高）
- **假設三**：消費者願意以 LINE 登入換 Wi-Fi（風險：隱私疑慮，需明確同意流程）

---

## 3. 目標用戶與使用情境

### 3.1 用戶角色（Personas）

#### Persona A — 店家老闆「阿凱」（30–50 歲）

- 經營 1–3 家餐飲店，已有 LINE 官方帳號但訊息開信率不到 10%
- 痛點：不知道誰來過、不知道何時推播、推播沒人看
- 期望：花 5 分鐘設定就能用，每月效果報表看得懂

#### Persona B — 連鎖總部行銷「Vivian」（28–40 歲）

- 連鎖品牌 10–50 家分店行銷主管
- 痛點：分店各自為政、客人跨店行為看不到、廣告無法 A/B
- 期望：總部後台一次設定多店、能看到跨店漏斗

#### Persona C — 終端消費者「小美」（18–60 歲）

- 到店想連 Wi-Fi 上網
- 痛點：每家店密碼都要問、加 LINE 怕被一直洗訊息
- 期望：3 秒內連上網、訊息少而精準、有實際優惠

### 3.2 核心使用情境（User Journeys）

#### Journey 1：消費者首次連線（30 秒內）

1. 小美到「咖啡店 A」，點選 Wi-Fi SSID `CafeA-Free`
2. 手機自動彈出 Captive Portal → 跳轉 LIFF 頁面
3. 頁面顯示「使用 LINE 登入即可免費上網」按鈕
4. 點按 → LINE 授權頁 → 同意 → 取得 `userId` + `displayName`
5. 後端呼叫 RADIUS / MikroTik Hotspot API 開通 MAC 上網權限（時長 180 分鐘）
6. LIFF 顯示「連線成功」+ 店家當期廣告（圖、影片、優惠券）
7. 自動加入該店家 LINE 官方帳號好友（一鍵）

#### Journey 2：消費者再次到訪（同店）

1. MAC 已被記錄，跳過 LINE 授權，直接通行
2. LIFF 顯示「歡迎回來，第 N 次到訪」+ 個人化推薦廣告
3. 行為紀錄寫入 `visit` 表

#### Journey 3：消費者跨店造訪

1. 小美第一次到「咖啡店 B」（同平台另一店家）
2. 因 LINE userId 已存在，跳過授權
3. 後台跨店分析顯示「客人小美也常去咖啡店 A」（去識別化呈現）

#### Journey 4：店家設定推播

1. 阿凱登入店家後台
2. 建立活動「下週三全店 8 折」
3. 選擇受眾：「最近 30 天到店且超過 2 次的客人」
4. 系統估算推播數 87 人 → 預扣 LINE 訊息額度
5. 排程於週二晚上 8 點發送
6. 隔日後台顯示開信率、點擊率、到店轉換

#### Journey 5：叫號服務（餐廳排隊）

1. 小美到熱門餐廳，連 Wi-Fi 後 LIFF 顯示「目前候位 12 組」
2. 按「我要候位」→ 取號 A013
3. 店員後台叫號，A013 號自動透過 LINE 訊息通知
4. 小美回店報到，店員後台標記「已入座」

---

## 4. 功能需求（MVP 範圍）

### 4.1 功能總覽

| 模組             | 功能                                          | 優先級 |
|------------------|-----------------------------------------------|--------|
| LIFF 前端        | Wi-Fi 登入頁、廣告展示、叫號介面、會員卡      | P0     |
| 平台後端         | 多租戶 API、用戶行為紀錄、Webhook、推播任務   | P0     |
| 店家管理後台     | 店家設定、廣告管理、推播排程、報表            | P0     |
| LINE 整合        | LINE Login、Messaging API、LIFF、Rich Menu    | P0     |
| 跨店分析         | 用戶圖譜、漏斗、留存、跨店熱度                | P1     |
| 叫號服務         | 取號、叫號通知、店員操作介面                  | P1     |
| Captive Portal 整合 | RADIUS / MikroTik / Ruijie / OpenWrt 介面 | P0     |
| 計費 / 帳務      | 訂閱方案、訊息額度、發票                      | P2     |

### 4.2 詳細需求 — User Stories + 驗收標準

#### EPIC-1：LIFF Wi-Fi 登入

**US-1.1**：身為消費者，我想用 LINE 登入換取 Wi-Fi，省去問密碼的麻煩。

驗收標準：
- 開啟 LIFF 頁面 1.5 秒內完成首屏渲染（4G 環境）
- LINE 授權成功後 5 秒內取得網路存取權
- 失敗時顯示明確錯誤訊息（含店員可協助的客服 LINE）
- 同一 MAC 在 24 小時內再次連線無須重複授權

**US-1.2**：身為消費者，我想知道 Wi-Fi 還剩多少時間。

驗收標準：
- LIFF 顯示剩餘時間倒數
- 剩餘 10 分鐘時透過 LINE 訊息通知「即將斷線，點此續用」

#### EPIC-2：廣告展示

**US-2.1**：身為店家，我想在 LIFF 頁面展示輪播廣告。

驗收標準：
- 支援 1–5 張圖片輪播（每張可設外連 URL）
- 支援單一 MP4（≤ 10 MB、≤ 15 秒）
- 廣告曝光（impression）與點擊（click）分別記錄
- 後台可設定每張廣告的「展示時段」（例：午餐時段限定）

**US-2.2**：身為店家，我想針對不同客群推不同廣告。

驗收標準：
- 可建立至少 3 種受眾標籤（新客、回頭客、VIP）
- 標籤規則由系統依造訪次數、最近造訪時間自動判定
- 後台 A/B 測試：同一活動可掛 2 組創意，自動 50/50 分流

#### EPIC-3：行為紀錄與分析

**US-3.1**：身為店家，我想知道每天有多少不重複客人到訪。

驗收標準：
- 後台首頁顯示今日 / 7 日 / 30 日的 UV、PV、新客比、回訪率
- 每日 04:00（台北時區）產出昨日報表
- 可匯出 CSV

**US-3.2**：身為連鎖總部，我想看到客人跨店行為。

驗收標準：
- 跨店熱度圖（哪些店之間有共同客人）
- 客群重疊百分比（A 店與 B 店共同客人佔 A 店 N%）
- 個人資料以雜湊化 userId 呈現，不揭露個資

#### EPIC-4：推播訊息

**US-4.1**：身為店家，我想對特定客群發推播。

驗收標準：
- 受眾條件：最近造訪日、造訪次數、標籤
- 訊息形式：文字、圖文訊息（Imagemap）、Flex Message 模板
- 排程：立即發送 / 指定時間 / 重複（每週 X 幾點）
- 預估觸及數即時顯示
- 發送後可看開信、點擊、回到店轉換

**US-4.2**：身為平台，需控管 LINE 訊息額度避免超發。

驗收標準：
- 店家方案內的免費額度即時扣抵
- 超量需先儲值或升級方案
- 達 80% / 100% 額度時 Email + LINE 通知店家

#### EPIC-5：店家後台管理

**US-5.1**：身為店家，我想自助開通使用平台。

驗收標準：
- 線上註冊 → Email 驗證 → 填寫店家資料 → 連接 LINE 官方帳號（OAuth）→ 建立 Wi-Fi 站點
- 全程 ≤ 10 分鐘
- 提供 Captive Portal 設定教學（MikroTik / Ruijie / Ubiquiti / OpenWrt 各一份）

**US-5.2**：身為店家，我想管理多家分店。

驗收標準：
- 一個帳號可掛多店（Site）
- 角色：擁有者 / 店長 / 行銷 / 店員（不同權限）
- 切換店家不需重新登入

#### EPIC-6：叫號服務

**US-6.1**：身為消費者，我想透過 LIFF 取號排隊。

驗收標準：
- LIFF 顯示目前候位數
- 取號後配發號碼牌（含 QR Code 給店員核對）
- 叫到號時 LINE 訊息通知 + Push Notification
- 過號可重新領號或取消

**US-6.2**：身為店員，我想用平板/手機叫號。

驗收標準：
- 後台叫號介面：下一號 / 跳過 / 入座 / 取消
- 叫號動作 1 秒內推送 LINE 訊息
- 候位列表即時更新（WebSocket / SSE）

### 4.3 範圍外（Out of Scope，MVP 不做）

- POS 整合（Phase 2）
- 線上點餐
- 電子發票自動歸戶
- 第三方廣告聯播網
- iOS / Android 原生 App（以 LIFF 為唯一前端）
- 中國大陸 / 海外市場

---

## 5. 非功能需求（NFR）

### 5.1 效能

| 場景                    | 指標                                         |
|-------------------------|---------------------------------------------|
| LIFF 首屏 LCP           | ≤ 1.8s（4G）                                |
| Wi-Fi 開通 API 回應     | P95 ≤ 800ms                                 |
| 後台主要查詢 API        | P95 ≤ 1.2s                                  |
| 推播任務啟動延遲        | ≤ 30s                                       |
| 同時線上店家            | 500（MVP），水平擴展可達 5,000              |
| 同時線上消費者連線/秒   | 200 RPS（MVP）                              |

### 5.2 可用性

- 系統 SLA：月 99.5%（MVP），上線 6 個月後提升至 99.9%
- 計畫性維護需提前 24 小時通知，控制在每月 ≤ 30 分鐘
- LIFF 與 Wi-Fi 開通服務優先保活，後台允許短暫降級

### 5.3 安全

- **傳輸層**：全站 HTTPS（TLS 1.2+），HSTS preload
- **認證**：店家後台採 OAuth2 + MFA（選用）；消費者端使用 LINE Login OIDC
- **授權**：RBAC，租戶隔離以 `tenant_id` 在每個查詢強制 scope
- **資料**：DB at-rest AES-256；個資欄位（手機、姓名）欄位級加密
- **個資**：符合《個人資料保護法》與 LINE Terms；提供「下載我的資料」與「刪除我的資料」自助介面
- **稽核**：所有店家後台寫入動作記錄 audit log，保留 1 年
- **滲透測試**：上線前完成 OWASP Top 10 自動化掃描 + 第三方 pentest 一次

### 5.4 可擴展性

- 服務拆分：邊界明確的 API Gateway + 後端微服務（Auth、Tenant、Ads、Tracking、Messaging、Queue）
- 資料庫：主資料用 PostgreSQL（含 Row-Level Security 做租戶隔離），行為事件用 ClickHouse 或 BigQuery
- 訊息佇列：Redis Streams（MVP）→ Kafka（Phase 2 ≥ 1k RPS）
- 部署：Kubernetes（GKE / EKS）；多 region 預留設計但 MVP 單 region（asia-east1）

### 5.5 無障礙設計（Accessibility）

- 遵循 WCAG 2.1 AA
- LIFF 字級可調整（系統偏好）
- 顏色對比 ≥ 4.5:1
- 主要操作可由螢幕閱讀器朗讀（VoiceOver / TalkBack 測試）
- 影像廣告須提供 `alt` 文字
- 叫號通知同時以 LINE 訊息送達，避免僅靠視覺/聽覺

### 5.6 法規遵循

- 個人資料保護法（台灣）
- 通訊保障及監察法（連線紀錄保留期）
- LINE Plugin / Messaging API ToS
- 兒童保護：< 13 歲使用者拒絕收集行為資料

### 5.7 國際化（i18n）

- MVP 僅支援繁體中文（zh-TW）
- 文字資源外部化，預留英文（en）、日文（ja）骨架

### 5.8 可觀測性

- Logs：結構化 JSON，集中於 Loki / CloudWatch
- Metrics：Prometheus + Grafana，核心 SLI 看板
- Tracing：OpenTelemetry（API → DB）
- 告警：PagerDuty / LINE Notify 雙通道

---

## 6. 技術架構與限制

### 6.1 系統架構（高階）

```
[消費者手機]
  └─ LIFF（Svelte / React）
        ├─ LINE Login (OIDC)
        ├─ LIFF SDK
        └─ HTTPS → API Gateway
                         ├─ Auth Service (LINE OIDC verify)
                         ├─ Wi-Fi Gateway Service ──► RADIUS / MikroTik / Ruijie API
                         ├─ Tenant / Ads Service ──► PostgreSQL (RLS)
                         ├─ Tracking Service     ──► Redis Stream → ClickHouse
                         └─ Messaging Service    ──► LINE Messaging API
                                                   └─ Webhook Handler ◄──┐
[店家平板 / 店員手機]                                                       │
  └─ 店家後台（Web，Svelte / Next.js） ── HTTPS → API Gateway              │
                                                                          │
[店家路由器 (CPE)]                                                         │
  └─ Captive Portal redirect ── LIFF                                       │
  └─ RADIUS / API authorize ◄── Wi-Fi Gateway Service                      │
                                                                          │
[排程 Worker] ── 推播 Job ── LINE Messaging API ──────────────────────────┘
```

### 6.2 技術棧建議

| 層級           | 技術                                           | 備註                         |
|----------------|-----------------------------------------------|------------------------------|
| LIFF 前端      | Svelte / SvelteKit + Vite                     | 與本團隊現有技能對齊         |
| 後台前端       | Svelte / SvelteKit                            | 同上                         |
| 後端           | Go 1.23（Gin / Chi）                          | 與本團隊現有技能對齊         |
| API Gateway    | Kong / Traefik                                | OSS、可水平擴展              |
| 主資料庫       | PostgreSQL 16 + Row-Level Security            | 多租戶硬隔離                 |
| 事件資料庫     | ClickHouse（或 BigQuery）                     | 高吞吐分析                   |
| 快取 / Queue   | Redis 7（含 Streams）                         |                              |
| 訊息排程       | Asynq（Go） / Sidekiq（Ruby）                 | MVP 採 Asynq                 |
| 容器           | Docker + Kubernetes（GKE）                    |                              |
| CI/CD          | GitHub Actions + ArgoCD                       |                              |
| 觀測           | Prometheus / Grafana / Loki / Tempo           |                              |
| 雲端           | GCP（首選）/ AWS                              | GCP 在台灣有地端 region      |

### 6.3 整合點

| 整合對象                  | 用途                          | 風險與備註                         |
|---------------------------|-------------------------------|------------------------------------|
| LINE Login (OIDC)         | 消費者身份識別                | 需申請 Channel；流量大需注意 QPS  |
| LINE Messaging API        | 推播 + Webhook                | 訊息計費 2024 起調漲，需控管成本  |
| LIFF SDK                  | 嵌入 LINE App 內              | 受限於 LINE 內建瀏覽器特性         |
| Rich Menu API             | 官方帳號下方選單              | 一個帳號限 1 個啟用                |
| RADIUS (FreeRADIUS)       | 標準化 Wi-Fi AAA              | 適用 enterprise CPE                |
| MikroTik Hotspot API      | 中小店家常用品牌              | 需 SSH 或 REST API 開通            |
| Ruijie / Reyee Cloud API  | 連鎖品牌常用                  | 需洽談 API 取用                    |
| OpenWrt + ChilliSpot      | 改機路由替代方案              | 適合預算極低店家                   |

### 6.4 關鍵技術決策與取捨

| 決策                        | 選擇                       | 替代方案 / 取捨                                 |
|-----------------------------|----------------------------|-------------------------------------------------|
| 多租戶隔離                  | 單庫 + RLS（共享 schema）  | 取捨：跨租戶查詢易；換 schema-per-tenant 隔離強但運維重 |
| 行為資料儲存                | PostgreSQL → 異步寫 ClickHouse | 取捨：MVP 量小可單庫；先建 pipeline 留擴展空間 |
| LIFF vs PWA                 | LIFF 為主                  | PWA 獨立網頁，但失去 LINE 內 Login 一鍵體驗     |
| 前端框架                    | Svelte                     | React 生態豐富但本團隊熟 Svelte                 |
| Wi-Fi 開通機制              | 雙模式：RADIUS + 廠牌 API  | 純 RADIUS 標準化；廠牌 API 涵蓋更多店家         |
| 推播任務模型                | Asynq + Redis              | 起步輕量；後續可升 Kafka                        |

### 6.5 技術限制

- LIFF 內 WebView 在 iOS / Android 行為不一致，需充分測試
- LINE Messaging API 訊息有頻次限制（每秒 100 則 / 每月免費額度有限）
- Captive Portal 在 iOS 上對 HTTP 跳轉行為已限制，需走 HTTPS + 標準 captive portal API（CAPPORT RFC 8908）
- 部分店家 CPE 不支援 RADIUS，需提供轉接方案
- 廣告影片受限於 LINE 內瀏覽器自動播放政策，需 muted + 觸發後播放

### 6.6 部署考量

- 環境：dev / staging / production，三環境一致；config 走 Helm values
- Secrets：GCP Secret Manager / Vault，禁止入庫
- DB 遷移：goose / golang-migrate，CI 自動執行
- Blue-Green / Canary：API 服務支援漸進式發布
- 備份：PostgreSQL PITR（保留 7 天）；行為庫每日 snapshot
- 災難復原：RPO 1 小時，RTO 4 小時（MVP）

---

## 7. 成功指標（KPI）與量測

| 類別     | 指標                          | 衡量工具                  |
|----------|-------------------------------|---------------------------|
| 取得     | LINE 加好友轉換率（連線→加好友）| 後端事件                  |
| 啟用     | 首次造訪 7 日內回訪率         | ClickHouse query          |
| 互動     | LIFF 廣告 CTR、影片完播率     | 前端事件 + 後端           |
| 留存     | 月活 / 週活店家比              | 後台 metrics              |
| 收入     | MRR、ARPU                     | 計費系統                  |
| 推播效益 | 開信率、點擊率、到店轉換率    | LINE Webhook + 後端       |
| 系統健康 | 5xx 錯誤率、API 延遲、可用性  | Prometheus                |

---

## 8. 建議任務分解（Engineering Backlog）

依優先順序由上而下，括號內為粗估人/週（一週 = 1 工程師 5 工作日）。

### 階段 0 — 立項與骨架（共約 4 人週）

- TASK-001 [Backend] 建立 monorepo 骨架（Go modules + 前端 workspace）（0.5）
- TASK-002 [DevOps] 建立 dev / staging GKE cluster + GitHub Actions CI（1.5）
- TASK-003 [Backend] 設計多租戶 PostgreSQL schema（含 RLS policy）+ migration 工具（1）
- TASK-004 [DevOps] 申請 LINE Channel（Login + Messaging）+ LIFF App，環境變數注入（0.5）
- TASK-005 [Frontend] LIFF 專案骨架（SvelteKit + LIFF SDK + i18n zh-TW）（0.5）

### 階段 1 — Wi-Fi 登入閉環 MVP（共約 7 人週）

- TASK-101 [Backend] LINE Login OIDC verify + 建立 user / device / session 記錄（1.5）
- TASK-102 [Backend] Wi-Fi Gateway Service：抽象介面 + RADIUS adapter + MikroTik adapter（2）
- TASK-103 [Frontend] LIFF Wi-Fi 登入頁（首屏 LCP ≤ 1.8s）（1）
- TASK-104 [Frontend] 連線成功頁 + 廣告輪播 + 倒數計時（1）
- TASK-105 [Backend] 行為事件 SDK：impression / click / connect 寫入 Redis Stream（1）
- TASK-106 [QA] LIFF 跨機型實機測試（iPhone 12+ / Android 11+）（0.5）

### 階段 2 — 店家後台 + 廣告管理（共約 6 人週）

- TASK-201 [Frontend] 店家後台骨架（SvelteKit + Auth + RBAC）（1.5）
- TASK-202 [Backend] Tenant / Site / User / Role API（CRUD + 邀請）（1.5）
- TASK-203 [Backend] Ads / Campaign / Creative API + 排程欄位（1）
- TASK-204 [Frontend] 廣告管理 UI（建立活動、上傳素材、排程、預覽）（1.5）
- TASK-205 [Backend] 受眾標籤引擎（規則式：訪問次數 / 最近造訪）（0.5）

### 階段 3 — 推播與分析（共約 7 人週）

- TASK-301 [Backend] LINE Messaging Service：訊息範本、額度扣抵、Webhook（1.5）
- TASK-302 [Backend] 推播 Job 排程（Asynq）+ 預估受眾數 API（1.5）
- TASK-303 [Frontend] 推播建立精靈（受眾 → 內容 → 排程 → 預覽 → 確認）（1.5）
- TASK-304 [Backend] ClickHouse 接入 + 每日彙總 ETL（1）
- TASK-305 [Frontend] 後台首頁儀表板（UV / 新客 / 回訪 / 推播效益）（1）
- TASK-306 [Backend] 跨店分析 API（雜湊 userId、共同客群）（0.5）

### 階段 4 — 叫號服務（共約 3 人週）

- TASK-401 [Backend] Queue / Ticket Domain + WebSocket / SSE 推送（1）
- TASK-402 [Frontend] LIFF 取號頁 + 號碼牌（QR）（0.5）
- TASK-403 [Frontend] 店員叫號平板介面（下一號 / 跳過 / 入座）（1）
- TASK-404 [Backend] 叫號通知整合 LINE Messaging（0.5）

### 階段 5 — 場域驗證與優化（共約 4 人週）

- TASK-501 [PM/Sales] 招募 3–5 家種子店家（餐飲 2 / 咖啡 2 / 零售 1）（並行）
- TASK-502 [DevOps] 上線 production 環境 + 監控告警（1）
- TASK-503 [QA] E2E 自動化（Playwright）+ 負載測試（k6）達標 200 RPS（1.5）
- TASK-504 [Sec] OWASP Top 10 掃描 + 第三方 pentest（1）
- TASK-505 [PM] 場域試運行 4 週、訪談 + 數據檢視，產出優化清單（0.5）

### 階段 6（Phase 2，僅列出，MVP 不做）

- 計費 / 訂閱（Stripe / 藍新）
- POS 整合（iCHEF / Square）
- A/B 測試平台
- 廣告自動最佳化（多臂老虎機）
- 連鎖總部進階分析

---

## 9. 里程碑與時程（粗估）

| 里程碑                    | 時程       | 驗收條件                                         |
|---------------------------|-----------|--------------------------------------------------|
| M1 — 骨架就緒             | T+4 週    | dev 環境 CI/CD 跑通、LINE Channel 完成           |
| M2 — Wi-Fi 登入閉環可用   | T+8 週    | 一家 demo 店家可走完 Journey 1                  |
| M3 — 店家後台 Beta        | T+12 週   | 店家可自助上廣告                                 |
| M4 — 推播 + 分析上線      | T+16 週   | 推播效益可量測                                   |
| M5 — 叫號服務上線         | T+19 週   | 至少 1 家餐飲使用                                |
| M6 — 場域驗證 + GA        | T+24 週   | 3–5 家店家 4 週連續使用、達成 1.3 目標           |

---

## 10. 開放議題（Open Questions）

1. 第一波鎖定餐飲為主？還是同時嘗試零售？（影響受眾標籤預設規則）
2. Wi-Fi 開通方案：是否先支援 MikroTik + Ruijie 兩家就好，OpenWrt 延後？
3. 計費模式：純訂閱、還是訂閱 + 訊息額度雙軌？
4. 連鎖總部後台是 MVP 內，還是 Phase 2？（目前判定 Phase 2，但若種子客戶有連鎖則可能調整）
5. 是否需要與 iCHEF / Square POS 在 MVP 即整合？

---

## 11. 附錄

### 11.1 名詞表

- **LIFF**：LINE Front-end Framework，可在 LINE App 內開啟並取得用戶身分的 Web 容器
- **Captive Portal**：使用者連 Wi-Fi 後，作業系統強制跳出的登入頁
- **RADIUS**：標準化 AAA（Authentication / Authorization / Accounting）通訊協定
- **CAPPORT**：RFC 8908，現代 Captive Portal 標準
- **RLS**：PostgreSQL Row-Level Security
- **Tenant**：租戶（在本系統中為店家集團或單店）
- **Site**：站點（一個實體店面，可掛多個 Wi-Fi SSID）

### 11.2 參考資料

- LINE Developers — Messaging API & LIFF（developers.line.biz）
- RFC 8908 — Captive Portal API
- WCAG 2.1 — Web Content Accessibility Guidelines
- 個人資料保護法（中華民國）

---

**文件結束**
