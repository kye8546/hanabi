# 花火 (Hanabi) — 單機 console 版

你與 1–4 個 AI 合作施放煙火。標準 5 色規則：8 個提示 token、3 道閃電。
也可以選 [Hanab Live](https://github.com/Hanabi-Live/hanabi-live/blob/main/docs/variants.md) 的變體：
- **6 Suits**：多一個青色（T，提示用 `t` / `青`），每色 10 張、共 60 張，滿分 30。
- **Black (6 Suits)**：多一個黑色（K，提示用 `k` / `黑`），每個數字只有 1 張（全都是 critical），共 55 張，滿分 30。
- **Rainbow (6 Suits)**：多一個彩虹色（M，洋紅色顯示），每種顏色提示都會點到它、但不能用彩虹色提示，共 60 張，滿分 30。
  隱藏反向資訊時，被兩種以上不同顏色點到的牌才顯示成彩虹色，只被一種顏色點到就顯示那種顏色。

## 執行

```bash
go run ./cmd/hanabi              # 開局時會要求選人數（2–5）、H-Group 慣例等級（目前可選 Lv1–7）、變體（Enter = 標準 5 色，2 = 6 Suits，3 = Black，4 = Rainbow）
go run ./cmd/hanabi -seed 42     # 固定牌序（不加 -seed 時開局會問種子，直接按 Enter = 隨機）
go run ./cmd/hanabi -nocolor     # 不用終端顏色（也支援 NO_COLOR 環境變數）
go run ./cmd/hanabi -showneg     # 開局就顯示反向提示資訊（預設只顯示被點到的顏色/數字；遊戲中可用 n 即時切換，連 AI 手牌一起）
```

編成 Windows 執行檔：`GOOS=windows GOARCH=amd64 go build -o hanabi.exe ./cmd/hanabi`。
遊戲結束（含輸入 `q` 離開）時會等你按 Enter 才關閉，直接點兩下 exe 執行時視窗不會一結束就消失。

AI 依 [H-Group 慣例](https://hanabi.github.io/) 行動，目前實作到 **Level 7**（更高的等級還不能選）：
- Lv1：整份 Beginner's Guide。
- Lv2：5 Stall、Double/Triple Prompt、Double Finesse、Prompt + Finesse、Reverse Finesse、Self-Finesse、Wrong Prompt。
- Lv3：Playing Multiple 1's（起手牌的 1 由舊到新、Fresh 1's、Chop-Focus 例外）、Fix Clue（含不給新資訊的 Stop Clue）、Sarcastic Discard、Information Lock。
- Lv4：Chop Moves——Trash Chop Move（提示大家都知道的垃圾 → 它右邊的牌受保護）、5's Chop Move（非拖延時「5」點到 chop 左邊一張的 5 → 保護 chop）、Order Chop Move（不照順序打 1，跳過幾張就保護後面第幾位的 chop）。
- Lv5：Layered / Clandestine / Hidden Finesse、Queued Finesse、Urgency Principle 的例外、Ambiguous Finesse，以及彩虹變體的 Prompts in Multi-Color Variants（prompt 正向資訊最多的已提示牌）。
- Lv6：Tempo Clues——Focus Shifting（tempo clue 的焦點已知能打時，移到下一張不知道能打的）、Valuable Tempo Clue、Tempo Clue Chop Move（不在拖延時給、沒有價值的 tempo clue 會順便保護接收者的 chop；同時像 Trash Chop Move 時以它為準），以及一般原則：差一張就能打的牌用 Chop Move 保護、Discard Modulation、Clarity Principle。
- Lv7：Emergency Discards——Scream Discard Chop Move（提示 0 個時不打大家都知道能打的牌、改棄 chop → 下一位 Chop Move 自己 chop 上必須保留的牌，這一手必須給提示；手上有大家都知道沒用的已提示牌卻棄 chop 也一樣）、Shout Discard Chop Move（任何提示數，不打能打的牌、改棄已知垃圾）、Generation Discard（棄牌換提示，讓下一位救再下一位的 chop，不 Chop Move），以及在終局、還能承受閃電時盲打很可能能打的 Chop Moved 牌。
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
| `u` | 回溯：回到你上一次行動之前（撤銷你那一手和之後 AI 的行動），可連續使用 |
| `x` | 偷看自己手牌的真實牌面（紀錄檔會註明在第幾回合偷看過） |
| `n` | 切換是否顯示反向提示資訊（例如「不是紅色」），連 AI 手牌一起 |
| `log` / `b` / `?` / `q` | 完整紀錄 / 重畫盤面 / 說明 / 離開 |

簡寫也可以：`p3`、`h1r`。

## 結構

```
game/      純規則引擎（無 I/O）：Action → Event，PlayerView 遮蔽自己的手牌
player/    Player 介面（真人與 AI 共用）＋ Run 主迴圈
ai/hgroup  H-Group 慣例 AI（目前 Level 1–7）
ai/        其他 AI；DiscardBot（能棄就棄最舊的牌，當基準）
console/   文字介面：Renderer 畫面、Human 讀指令
cmd/hanabi 組裝並啟動
```

- 新 AI：在 `ai/` 實作 `player.Player`，只依賴 `game.PlayerView`。
- 新 UI：取代 `console/`，`game/` 不需改動。
- 全 AI 對局可直接用 `player.Run`（見 `ai/hgroup/sim_test.go`）。

```bash
go test ./...
go test ./ai/hgroup -run 'TestSelfPlay$' -v   # 各等級 AI 自我對局的平均分數（HLEVEL=2 只跑 Level 2；HVARIANT=black6 / rainbow6 換變體；HGAMES=1000 改局數）
```
