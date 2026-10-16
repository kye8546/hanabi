// Rainbow 變體的慣例（https://hanabi.github.io/variant-specific/rainbow ；變體慣例不屬於任何等級，所有等級都用）。
// 彩虹色的牌被每種顏色提示點到（hintSet）。Level 5 的 Prompts in Multi-Color Variants 在 connect.go 的 promptOrder。

package hgroup

import (
	"slices"

	"hanabi/game"
)

// multiColor：這局有被多種顏色提示點到的花色（彩虹色）。
func (s *state) multiColor() bool { return slices.Contains(s.colors, game.Rainbow) }

// freeChoice：顏色提示只點到彩虹色的牌，而給提示者有好幾種顏色可選、點到的牌都完全一樣（Free Choice）。
// colors 是這些可選的顏色（位元），chosen 是他選的；沒有 Free Choice 時 colors 是 0。
// receiver 看不到自己的牌，不知道點到的是不是彩虹牌，只有看得到的人能這樣推論。
type freeChoice struct {
	colors   uint8
	chosen   game.Color
	receiver int
}

// freeChoiceOf 算出提示 h（點到 R 的 touched）的 Free Choice。我是接收者時看不到，算不出來。
func (s *state) freeChoiceOf(R int, h game.Hint, touched []int) freeChoice {
	if h.Kind != game.HintColor || R == s.me || !s.multiColor() {
		return freeChoice{}
	}
	for _, id := range touched {
		if c, ok := s.card(id); !ok || c.Color != game.Rainbow {
			return freeChoice{}
		}
	}
	var mask uint8
	for _, c := range s.colors {
		if c == game.Rainbow {
			continue
		}
		same := true
		for _, id := range s.hands[R] {
			k, ok := s.card(id)
			if !ok || (game.Hint{Kind: game.HintColor, Color: c}).Matches(k) != slices.Contains(touched, id) {
				same = false
				break
			}
		}
		if same {
			mask |= 1 << c
		}
	}
	if mask&(mask-1) == 0 {
		return freeChoice{} // 只有一種顏色可選，不是 Free Choice
	}
	return freeChoice{colors: mask, chosen: h.Color, receiver: R}
}

// freeChoiceFinesse（The Free Choice Finesse）：要 prompt 彩虹牌 m 時，給提示者有 Free Choice、其中有和已提示牌 id
// 被點到的顏色相同的顏色，他卻選了不同的顏色——那就不是 prompt 這張，而是 finesse（例如 Bob 有一張紅色的牌，
// Alice 用藍色而不是紅色點到 Cathy 的 m2，Bob 就盲打 finesse position 的 m1）。接收者自己看不到，不這樣推論。
func (s *state) freeChoiceFinesse(obs, id int, m ident) bool {
	f := s.free
	if f.colors == 0 || obs == f.receiver || m.color() != game.Rainbow {
		return false
	}
	pos := uint8(s.notes[id].hinted) // 點到它的顏色（hinted 的低 8 位元）
	return pos&f.colors != 0 && pos&(1<<f.chosen) == 0
}
