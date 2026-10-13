// Package hgroup 實作依照 H-Group 慣例（https://hanabi.github.io/）行動的 AI。
//
// 目前實作 Level 1–4：Level 1 涵蓋整份 Beginner's Guide（chop、play/save clue、焦點判定、
// Good Touch / Save / Minimum Clue Value 三原則、Early Game、delayed play clue、prompt、finesse、失誤後重置推論）；
// Level 2 加上多重 prompt / finesse、Reverse Finesse、Self-Finesse、5 Stall、Wrong Prompt；
// Level 3 加上 Playing Multiple 1's、Fix Clue、Sarcastic Discard；
// Level 4 加上 Chop Moves（Trash Chop Move、5's Chop Move、Order Chop Move）。
// 變體：Black (6 Suits) 另外照黑色的 save clue 慣例（https://hanabi.github.io/variant-specific/black，所有等級）。
package hgroup

import (
	"math/bits"
	"strconv"

	"hanabi/game"
)

// ident 是一種牌面（顏色 + 數字），編號 0..34（青色是 25..29、黑色是 30..34）。
type ident int8

const numIdents = game.NumColors * int(game.MaxRank)

func identOf(c game.Color, r game.Rank) ident { return ident(int(c)*int(game.MaxRank) + int(r) - 1) }
func identOfCard(c game.Card) ident           { return identOf(c.Color, c.Rank) }

func (i ident) color() game.Color { return game.Color(int(i) / int(game.MaxRank)) }
func (i ident) rank() game.Rank   { return game.Rank(int(i)%int(game.MaxRank) + 1) }
func (i ident) String() string    { return i.color().Letter() + strconv.Itoa(int(i.rank())) }
func (i ident) copies() int       { return game.Copies(i.color(), i.rank()) } // 這個牌面共幾張

// idSet 是牌面的集合（bitmask），用來記錄一張牌「還可能是」哪些牌面。
type idSet uint64

// suitIdents 回傳變體 va 所有可能的牌面。
func suitIdents(va game.Variant) idSet {
	var s idSet
	for _, c := range va.Colors() {
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			s |= 1 << identOf(c, r)
		}
	}
	return s
}

func setOf(ids ...ident) idSet {
	var s idSet
	for _, i := range ids {
		s |= 1 << i
	}
	return s
}

func (s idSet) has(i ident) bool { return s&(1<<i) != 0 }
func (s idSet) count() int       { return bits.OnesCount64(uint64(s)) }

func (s idSet) list() []ident {
	var out []ident
	for i := ident(0); int(i) < numIdents; i++ {
		if s.has(i) {
			out = append(out, i)
		}
	}
	return out
}

// single 在集合只剩一種牌面時回傳它。
func (s idSet) single() (ident, bool) {
	if s.count() != 1 {
		return 0, false
	}
	return ident(bits.TrailingZeros64(uint64(s))), true
}

// hintSet 回傳提示 h 會點到的所有牌面。
func hintSet(h game.Hint) idSet {
	var s idSet
	for i := ident(0); int(i) < numIdents; i++ {
		if (h.Kind == game.HintColor && i.color() == h.Color) || (h.Kind == game.HintRank && i.rank() == h.Rank) {
			s |= 1 << i
		}
	}
	return s
}
