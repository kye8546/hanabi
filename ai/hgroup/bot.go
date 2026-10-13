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
