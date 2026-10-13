// Package ai 放各種 AI 玩家。每個 AI 只依賴 game.PlayerView 做決策。
package ai

import "hanabi/game"

// DiscardBot 是最陽春的 AI：能棄牌就棄最舊（最右邊）的牌；
// 提示 token 已滿不能棄牌時，給第一個合法提示。
type DiscardBot struct {
	name string
}

func NewDiscardBot(name string) *DiscardBot { return &DiscardBot{name: name} }

func (b *DiscardBot) Name() string { return b.name }

func (b *DiscardBot) Act(v game.PlayerView) game.Action {
	hand := v.MyHand()
	if v.CanDiscard() {
		return game.Discard(len(hand) - 1)
	}
	if hints := v.LegalHints(); len(hints) > 0 {
		return hints[0]
	}
	// 理論上不會發生（提示已滿時一定有合法提示），保底打出最舊的牌
	return game.Play(len(hand) - 1)
}
