package hgroup

import "hanabi/game"

// MaxLevel 是目前已實作的最高慣例等級。
const MaxLevel = 4

// Levels 是 H-Group 的 25 個慣例等級名稱（Levels[0] = Level 1）。
var Levels = [...]string{
	"基礎 Fundamentals",
	"基本技巧 Basic Moves",
	"基本策略 Basic Strategy",
	"Chop Moves",
	"中階 Finesse",
	"Tempo Clues",
	"最後手段 Last Resorts",
	"終局 End-Game",
	"拖延 Stalling",
	"特殊棄牌 Special Discards",
	"Bluffs",
	"情境 Context",
	"中階 Bluff",
	"垃圾牌技巧 Trash Moves",
	"Double Bluffs",
	"Ejections & Discharges",
	"重複牌技巧 Dupe Tech",
	"消去法 Elimination",
	"5 的技巧 5 Tech",
	"進階技巧 Advanced Moves",
	"Ignition",
	"緊急棄牌 Emergency Discards",
	"Charms & 其他",
	"非必要行動 Unnecessary Moves",
	"優先順序 Priority",
}

// 各技巧從哪個 H-Group 等級開始。低於該等級的 AI 不給、也不那樣解讀（s.knows）。
const (
	lvMultiConnect   = 2 // 一個提示多張連接牌：Double / Triple Prompt、Double Finesse、Prompt + Finesse
	lvReverseFinesse = 2 // Reverse Finesse：finesse 接收者之後的人
	lvSelfFinesse    = 2 // Self-Finesse：finesse 接收者自己（以及 finesse 對象讓給看得到的後一位）
	lvFiveStall      = 2 // 5 Stall
	lvWrongPrompt    = 2 // Wrong Prompt
	lvMultipleOnes   = 3 // Playing Multiple 1's（出牌順序、焦點、多出來的 1）
	lvFixClue        = 3 // Fix Clue（含 Stop Clue；Misplay Cost Principle 的修正）
	lvSarcastic      = 3 // Sarcastic Discard
	lvInfoLock       = 3 // Information Lock on In-Between Cards
	lvChopMove       = 4 // Chop Moves：Trash / 5's / Order Chop Move
	lvQueuedFinesse  = 5 // Queued Finesse（還沒實作；在那之前不 finesse 手上已有待打 finesse 牌的人）
)

// knows：這個 AI 的等級已經學到從 lv 開始的技巧。
func (s *state) knows(lv int) bool { return s.level >= lv }

// Bot 是依 H-Group 慣例行動的 AI。只透過 PlayerView 取得資訊。
type Bot struct {
	name  string
	level int
	s     *state
}

// New 建立一個使用指定慣例等級的 AI；超過已實作的等級時以 MaxLevel 行動。
func New(name string, level int) *Bot {
	return &Bot{name: name, level: min(max(level, 1), MaxLevel)}
}

func (b *Bot) Name() string { return b.name }

// Why 回傳最近一次行動的理由（寫進對局紀錄用）。
func (b *Bot) Why() string {
	if b.s == nil {
		return ""
	}
	return b.s.why
}

func (b *Bot) Act(v game.PlayerView) game.Action {
	if b.s == nil {
		b.s = newState(v, b.level)
	}
	b.s.sync(v)
	return b.s.decide(v)
}
