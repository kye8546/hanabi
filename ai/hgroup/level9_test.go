package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 9（Stalling）的範例情境，取自 H-Group 原文 level-9.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。

func TestL9DoubleDiscardAvoidance(t *testing.T) {
	// 原文 Double Discard Situation 的範例：Alice 沒事可做，棄掉 chop 上的 r4；Bob 也沒有 play / save clue 可給，
	// 他沒被提示的 chop 可能就是另一張 r4。Lv9 的 Bob 不棄牌，給 Cathy slot 1 的 r5 一個 5 Stall；Lv8 照樣棄 chop。
	// Cathy 知道 Bob 處在 Double Discard Situation，「5」只是 5 Stall，不是要她打 r5
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "y3 g3 b3 w3 r4", "b4 y4 g2 w2 r3", "r5 g3 b3 y3 w3")
		g.Hints = 5
		force(t, g, game.Discard(4))
		a := step(t, g, bots)
		want := game.Discard(4)
		if lv >= 9 {
			want = game.HintRankTo(2, 5)
		}
		if a != want {
			t.Errorf("Lv%d：Bob 應 %+v，卻做了 %+v", lv, want, a)
		}
		if lv >= 9 {
			if got := step(t, g, bots); got == game.Play(0) {
				t.Errorf("Lv%d：Cathy 不該打出 5 Stall 點到的 r5", lv)
			}
		}
	}
}

func TestL9NoDDAWhenOtherCopyVisible(t *testing.T) {
	// 同上，但另一張 r4 就在 Cathy 手上，Bob 看得到：他的 chop 不可能是 r4，照常棄牌
	g, bots := scenarioAt(t, 9, "", "y3 g3 b3 w3 r4", "b4 y4 g2 w2 r3", "r5 g3 b3 r4 w3")
	g.Hints = 5
	force(t, g, game.Discard(4))
	if a := step(t, g, bots); a != game.Discard(4) {
		t.Errorf("Bob 應棄 chop，卻做了 %+v", a)
	}
}
