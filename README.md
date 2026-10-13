# 花火 (Hanabi) — 單機 console 版

你與 1–4 個 AI 合作施放煙火。標準 5 色規則：8 個提示 token、3 道閃電。
也可以選 [Hanab Live](https://github.com/Hanabi-Live/hanabi-live/blob/main/docs/variants.md) 的變體：
- **6 Suits**：多一個青色（T，提示用 `t` / `青`），每色 10 張、共 60 張，滿分 30。
- **Black (6 Suits)**：多一個黑色（K，提示用 `k` / `黑`），每個數字只有 1 張（全都是 critical），共 55 張，滿分 30。

## 執行

```bash
go run ./cmd/hanabi              # 開局時會要求選人數（2–5）、H-Group 慣例等級（Lv1–25）、變體（Enter = 標準 5 色，2 = 6 Suits，3 = Black）
go run ./cmd/hanabi -seed 42     # 固定牌序（不加 -seed 時開局會問種子，直接按 Enter = 隨機）
go run ./cmd/hanabi -nocolor     # 不用終端顏色（也支援 NO_COLOR 環境變數）
go run ./cmd/hanabi -hideneg     # 不顯示反向提示資訊，只顯示被點到的顏色/數字（預設和 Hanab Live 一樣顯示）
```

編成 Windows 執行檔：`GOOS=windows GOARCH=amd64 go build -o hanabi.exe ./cmd/hanabi`。
遊戲結束（含輸入 `q` 離開）時會等你按 Enter 才關閉，直接點兩下 exe 執行時視窗不會一結束就消失。

AI 依 [H-Group 慣例](https://hanabi.github.io/) 行動，目前實作到 **Level 4**：
- Lv1：整份 Beginner's Guide。
- Lv2：5 Stall、Double/Triple Prompt、Double Finesse、Prompt + Finesse、Reverse Finesse、Self-Finesse、Wrong Prompt。
- Lv3：Playing Multiple 1's（起手牌的 1 由舊到新、Fresh 1's、Chop-Focus 例外）、Fix Clue（含不給新資訊的 Stop Clue）、Sarcastic Discard。
- Lv4：Chop Moves——Trash Chop Move（提示大家都知道的垃圾 → 它右邊的牌受保護）、5's Chop Move（非拖延時「5」點到 chop 左邊一張的 5 → 保護 chop）、Order Chop Move（不照順序打 1，跳過幾張就保護後面第幾位的 chop）。
- Black 變體另外照 [黑色的慣例](https://hanabi.github.io/variant-specific/black)（所有等級）：救 chop 上的黑 2、黑 5 用數字，黑 3、黑 4 用黑色；用另一種提示點到它們就是 play clue。例外：黑色點到兩張以上新的黑牌、或補上一張已提示牌的資訊時可以救黑 2 / 黑 5；「3」/「4」同時點到另一張可能是 critical 的新牌（Florrat Save）時可以救黑 3 / 黑 4。（「為了不違反 Good Touch」「給了重要的負向資訊」這兩個例外接收者無從確認，AI 不用也不這樣解讀。）

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
| `log` / `b` / `?` / `q` | 完整紀錄 / 重畫盤面 / 說明 / 離開 |

簡寫也可以：`p3`、`h1r`。

## 結構

```
game/      純規則引擎（無 I/O）：Action → Event，PlayerView 遮蔽自己的手牌
player/    Player 介面（真人與 AI 共用）＋ Run 主迴圈
ai/hgroup  H-Group 慣例 AI（目前 Level 1–3）
ai/        其他 AI；DiscardBot（能棄就棄最舊的牌，當基準）
console/   文字介面：Renderer 畫面、Human 讀指令
cmd/hanabi 組裝並啟動
```

- 新 AI：在 `ai/` 實作 `player.Player`，只依賴 `game.PlayerView`。
- 新 UI：取代 `console/`，`game/` 不需改動。
- 全 AI 對局可直接用 `player.Run`（見 `ai/hgroup/sim_test.go`）。

```bash
go test ./...
go test ./ai/hgroup -run TestSelfPlay -v   # 各等級 AI 自我對局的平均分數（HLEVEL=2 只跑 Level 2）
```
