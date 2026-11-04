# 花火 (Hanabi) — 單機 console 版

你與 1–4 個 AI 合作施放煙火。標準 5 色規則：8 個提示 token、3 道閃電。
也可以選 [Hanab Live](https://github.com/Hanabi-Live/hanabi-live/blob/main/docs/variants.md) 的變體：
- **6 Suits**：多一個青色（T，提示用 `t` / `青`），每色 10 張、共 60 張，滿分 30。
- **Black (6 Suits)**：多一個黑色（K，提示用 `k` / `黑`），每個數字只有 1 張（全都是 critical），共 55 張，滿分 30。
- **Rainbow (6 Suits)**：多一個彩虹色（M，洋紅色顯示），每種顏色提示都會點到它、但不能用彩虹色提示，共 60 張，滿分 30。
  隱藏反向資訊時，被兩種以上不同顏色點到的牌才顯示成彩虹色，只被一種顏色點到就顯示那種顏色。

## 執行

```bash
go run ./cmd/hanabi              # 開局時會要求選人數（2–5）、H-Group 慣例等級（目前可選 Lv1–10）、變體（Enter = 標準 5 色，2 = 6 Suits，3 = Black，4 = Rainbow）
go run ./cmd/hanabi -seed 42     # 固定牌序（不加 -seed 時開局會問種子，直接按 Enter = 隨機）
go run ./cmd/hanabi -nocolor     # 不用終端顏色（也支援 NO_COLOR 環境變數）
go run ./cmd/hanabi -showneg     # 開局就顯示反向提示資訊（預設只顯示被點到的顏色/數字；遊戲中可用 n 即時切換，連 AI 手牌一起）
```

### 網頁版

```bash
go run ./cmd/hanabiweb                    # 開 http://localhost:8080 用瀏覽器玩
go run ./cmd/hanabiweb -addr :9000        # 換位址（沒給 -addr 時用環境變數 PORT，雲端主機多半這樣設定）
go run ./cmd/hanabiweb -aidelay 300ms     # AI 每一手的間隔（預設 700ms，讓人看得出發生了什麼）
```

功能和 console 版相同：點自己的牌打出 / 棄掉，點隊友的牌給提示，cm 標記、AI 代打、回溯、偷看、反向資訊切換都有，
結束時可以下載對局紀錄（格式和 console 版相同）。遊戲在伺服器上跑，瀏覽器只收到自己能看的畫面，看不到自己的手牌。
對局紀錄存到 `weblogs/`（`-logs` 可改），和 console 版的 `logs/` 分開，不影響 AI 的重播基準；要分析就把檔案複製到 `logs/`。
前端檔案用 `go:embed` 包進執行檔，部署只需要一個執行檔。重新啟動伺服器時進行中的局會消失。
畫面用 SSE 推送；代理伺服器把串流緩衝住時（例如 Cloudflare 的免帳號通道）網頁會自動改用長輪詢。

讓朋友連到你電腦上的伺服器（免費、不用註冊；Mac 睡眠時連不到，`caffeinate` 讓它開著時不睡眠，`-s` 只在接電源時有效；闔上蓋子仍會睡）：

```bash
brew install cloudflared
caffeinate -is go run ./cmd/hanabiweb
cloudflared tunnel --url http://localhost:8080   # 另一個視窗；印出的 https://….trycloudflare.com 給朋友（每次啟動網址都不同）
```

編成 Windows 執行檔：`GOOS=windows GOARCH=amd64 go build -o hanabi.exe ./cmd/hanabi`。
遊戲結束（含輸入 `q` 離開）時會等你按 Enter 才關閉，直接點兩下 exe 執行時視窗不會一結束就消失。

AI 依 [H-Group 慣例](https://hanabi.github.io/) 行動，目前實作到 **Level 10**（更高的等級還不能選）：
- Lv1：整份 Beginner's Guide。
- Lv2：5 Stall、Double/Triple Prompt、Double Finesse、Prompt + Finesse、Reverse Finesse、Self-Finesse、Wrong Prompt。
- Lv3：Playing Multiple 1's（起手牌的 1 由舊到新、Fresh 1's、Chop-Focus 例外）、Fix Clue（含不給新資訊的 Stop Clue）、Sarcastic Discard、Information Lock。
- Lv4：Chop Moves——Trash Chop Move（提示大家都知道的垃圾 → 它右邊的牌受保護）、5's Chop Move（非拖延時「5」點到 chop 左邊一張的 5 → 保護 chop）、Order Chop Move（不照順序打 1，跳過幾張就保護後面第幾位的 chop）。
- Lv5：Layered / Clandestine / Hidden Finesse、Queued Finesse、Urgency Principle 的例外、Ambiguous Finesse，以及彩虹變體的 Prompts in Multi-Color Variants（prompt 正向資訊最多的已提示牌）。
- Lv6：Tempo Clues——Focus Shifting（tempo clue 的焦點已知能打時，移到下一張不知道能打的）、Valuable Tempo Clue、Tempo Clue Chop Move（不在拖延時給、沒有價值的 tempo clue 會順便保護接收者的 chop；同時像 Trash Chop Move 時以它為準），以及一般原則：差一張就能打的牌用 Chop Move 保護、Discard Modulation、Clarity Principle。
- Lv7：Emergency Discards——Scream Discard Chop Move（提示 0 個時不打大家都知道能打的牌、改棄 chop → 下一位 Chop Move 自己 chop 上必須保留的牌，這一手必須給提示；手上有大家都知道沒用的已提示牌卻棄 chop 也一樣）、Shout Discard Chop Move（任何提示數，不打能打的牌、改棄已知垃圾）、Generation Discard（棄牌換提示，讓下一位救再下一位的 chop，不 Chop Move），以及在終局、還能承受閃電時盲打很可能能打的 Chop Moved 牌。
- Lv8：End-Game——終局（pace < 人數）不 Chop Move（像 Chop Move 的提示改當拖延，也沒有 Scream / Shout Discard）、Positional Discard / Positional Misplay / Double Positional Misplay（看得到每一張還需要的牌時，棄或打錯某個位置，叫那個位置上有能打的牌的最後一位盲打）、Distribution Clue（終局刻意點到別人已提示的牌的重複，分擔他來不及打的牌），以及推演起來先拖延比較好時，自己的牌晚點打。
- Lv9：Stalling——Double Discard Avoidance（上一位棄掉的牌只剩一張、可能就在我的 chop 上時，不棄 chop，改給提示，拖延提示也可以；別人在這種情況給的提示當拖延解讀）；Early Game 全隊必須做一次 5 Stall（不必提示上一位玩家：Permission to Discard），5 Stall 要給最靠近 chop 的 5；提示滿 8、鎖手、Double Discard 時給的提示當拖延解讀（只補資訊的提示不是 finesse；Locked Hand Save：鎖手的人點到別人 chop 上的牌只是救它；8 Clue Save：提示滿 8 時點到不在 slot 1 的新牌只是救它）；Anxiety Play（鎖手的人提示 0 個時打出最可能能打的牌，所以他那張不能打時，他的上一位必須棄牌換一個提示給他）。
- Lv10：Special Discards——Gentleman's Discard（棄掉確定能打的已提示牌，傳給 finesse position 上有同一張的人盲打；可以藏在能打的牌後面：Layered）、Baton Discard（還不能打的確定的牌，正好在別人的 finesse position 時傳給他，那張視同被提示）——這兩種 AI 只解讀、不主動給（實測主動給分數略低）；Sarcastic Finesse（焦點可能和給提示者不確定的已提示牌重複的 finesse，接收者看到重複就 Sarcastic Discard 傳回）、Certain Finesse（被 finesse 的牌可能重複時，被 finesse 的人看到給提示者那張就是它，就把自己的 finesse position 棄掉傳回：Certain Discard）。
- 所有等級都會在終局被迫拖延（取自 Level 8：棄牌會讓最高分拿不到時改給提示）。
- Black 變體另外照 [黑色的慣例](https://hanabi.github.io/variant-specific/black)（所有等級）：救 chop 上的黑 2、黑 5 用數字，黑 3、黑 4 用黑色；用另一種提示點到它們就是 play clue。例外：黑色點到兩張以上新的黑牌、或補上一張已提示牌的資訊時可以救黑 2 / 黑 5；「3」/「4」同時點到另一張可能是 critical 的新牌（Florrat Save）時可以救黑 3 / 黑 4。（「為了不違反 Good Touch」「給了重要的負向資訊」這兩個例外接收者無從確認，AI 不用也不這樣解讀。）
- Rainbow 變體另外照 [彩虹的慣例](https://hanabi.github.io/variant-specific/rainbow)（所有等級）：彩虹 2 照一般的 2 Save，critical 的彩虹牌可以用任何顏色或數字救；Free Choice Finesse（顏色提示只點到彩虹牌時，捨棄和已提示牌相同的顏色、改用別的顏色 = finesse）。

你給 AI 的提示也會照同一個等級解讀，所以請照該等級的規範玩。

每局都會自動存一份紀錄到 `logs/日期-時間.log`（設定、每一步行動、AI 的決策理由），
覺得 AI 哪一步怪怪的，把檔案和回合數（例如 `t23`）給 Claude 就能完整重現。

## 指令

| 指令 | 說明 |
|---|---|
| `p <位置>` | 打出手牌（位置從左 1 開始；新牌放最左） |
| `d <位置>` | 棄牌（提示滿 8 個時不能棄） |
| `h <玩家> <顏色\|數字>` | 提示，例如 `h 1 r`、`h 2 3`；玩家可用編號或名字 |
| `m [玩家] <位置>` | 標記 / 取消標記一張牌被 Chop Move（位置列顯示 `cm`），例如 `m 1 4`、`m 5`（自己的牌）；只是你自己的筆記，跟著那張牌移動，AI 的 Chop Move 不會自動顯示 |
| `a` | AI 代打一手：照開局選的慣例等級替你決定這一手並直接執行（紀錄檔那一行會寫「AI 代打」和 AI 的理由） |
| `u` | 回溯：回到你上一次行動之前（撤銷你那一手和之後 AI 的行動），可連續使用 |
| `x` | 偷看自己手牌的真實牌面（紀錄檔會註明在第幾回合偷看過） |
| `n` | 切換是否顯示反向提示資訊（例如「不是紅色」），連 AI 手牌一起 |
| `log` / `b` / `?` / `q` | 完整紀錄 / 重畫盤面 / 說明 / 離開 |

簡寫也可以：`p3`、`h1r`、`m14`（1 號的位置 4）、`m5`（自己的位置 5）。

盤面上的 **Pace**（和 Hanab Live 相同）= 目前分數 + 牌堆張數 + 人數 − 還拿得到的最高分，
也就是從現在起最多還能棄幾張牌（打錯也算）而仍有機會拿到最高分；每棄一張減 1，打牌、提示不變，牌堆空了顯示 `-`。
小於人數就是 H-Group 的終局（End-Game，黃色），0 以下是紅色（0 = 不能再棄牌，負的 = 已經拿不到最高分）。
有牌面的所有張數都被棄掉時，分數後面會多顯示「最多 N」。說明頁（`?`）有完整的算法。

## 結構

```
game/      純規則引擎（無 I/O）：Action → Event，PlayerView 遮蔽自己的手牌
player/    Player 介面（真人與 AI 共用）＋ Run 主迴圈
ai/hgroup  H-Group 慣例 AI（目前 Level 1–10）
ai/        其他 AI；DiscardBot（能棄就棄最舊的牌，當基準）
console/   文字介面：Renderer 畫面、Human 讀指令
session/   console 與網頁共用：對局紀錄、AI 代打的座位、回溯
web/       網頁介面：HTTP 伺服器（房間、指令、SSE 推送畫面，備用長輪詢）＋ 內嵌的前端（web/static，純 HTML/CSS/JS）
cmd/hanabi 組裝並啟動 console 版
cmd/hanabiweb 啟動網頁版
```

- 新 AI：在 `ai/` 實作 `player.Player`，只依賴 `game.PlayerView`。
- 新 UI：取代 `console/`，`game/` 不需改動。
- 全 AI 對局可直接用 `player.Run`（見 `ai/hgroup/sim_test.go`）。

```bash
go test ./...
go test ./ai/hgroup -run 'TestSelfPlay$' -v   # 各等級 AI 自我對局的平均分數（HLEVEL=2 只跑 Level 2；HVARIANT=black6 / rainbow6 換變體；HGAMES=1000 改局數）
```
