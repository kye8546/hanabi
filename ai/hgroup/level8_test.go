package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 8（End-Game）的範例情境，取自 H-Group 原文 level-8.mdx 與 Level 8 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。原文 3 人局是 4 張手牌，這裡是 5 張。
// pace = 分數 + 牌堆張數 + 人數 − 還拿得到的最高分，小於人數就是終局。

// scenarioEnd 同 scenarioAt，但牌堆只剩 deck 張（終局用）。
func scenarioEnd(t *testing.T, level, deck int, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	g, bots := scenarioAt(t, level, stacks, hands...)
	g.Deck = g.Deck[:deck]
	return g, bots
}

// actAll 讓每個 bot 照目前畫面更新筆記（不執行它們的行動）。
func actAll(g *game.Game, bots []*Bot) {
	for p, b := range bots {
		b.Act(g.View(p))
	}
}

func TestL8NoTrashChopMoveInEndGame(t *testing.T) {
	// 終局（分數 15、牌堆 8 張：pace 1 < 3 人）：Alice「1」只點到 Bob 大家都知道沒用的 r1。
	// Lv7 是 Trash Chop Move（它右邊的牌受保護）；Lv8 終局不 Chop Move，只是拖延：r1 照樣標成沒用
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 8, "r3 y3 g3 b3 w3", "y4 g4 b4 w4 y4", "y4 r1 g4 b4 w4", "r4 b4 w4 g4 y1")
		force(t, g, game.HintRankTo(1, 1))
		r1, chop := g.Hands[1][1].Card.ID, g.Hands[1][4].Card.ID
		actAll(g, bots)
		for p, b := range bots {
			if got := b.s.notes[chop].cm; got != (lv < 8) {
				t.Errorf("Lv%d：座位 %d 把 Bob 的 chop 記成 Chop Moved = %v", lv, p, got)
			}
			if !b.s.notes[r1].bad {
				t.Errorf("Lv%d：座位 %d 沒把 r1 記成沒用", lv, p)
			}
		}
	}
}

func TestL8FiveChopMoveIsStallInEndGame(t *testing.T) {
	// 終局（Alice 棄牌後分數 15、牌堆 8 張：pace 1）：Cathy「5」點到 Bob chop 左邊一張的 r5。
	// Lv7 是 5's Chop Move（chop 受保護）；Lv8 終局改當 5 Stall，只救下 r5
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 9, "r3 y3 g3 b3 w3", "y4 g4 b4 w4 y1", "y4 g4 b4 r5 w4", "r4 b4 w4 g4 y1")
		g.Hints = 5
		force(t, g, game.Discard(4)) // 結束 Early Game（否則「5」是 5 Stall）
		force(t, g, game.HintColorTo(0, game.Yellow))
		force(t, g, game.HintRankTo(1, 5))
		r5, chop := g.Hands[1][3].Card.ID, g.Hands[1][4].Card.ID
		actAll(g, bots)
		for p, b := range bots {
			if got := b.s.notes[chop].cm; got != (lv < 8) {
				t.Errorf("Lv%d：座位 %d 把 Bob 的 chop 記成 Chop Moved = %v", lv, p, got)
			}
		}
		if p := bots[1].s.myPoss(r5); p&^rankSet(5) != 0 {
			t.Errorf("Lv%d：Bob 應把被點到的牌記成某張 5，卻是 %v", lv, p)
		}
		if a := step(t, g, bots); a == game.Play(3) {
			t.Errorf("Lv%d：Bob 不該打出 r5", lv)
		}
	}
}

func TestL8NoTempoClueChopMoveInEndGame(t *testing.T) {
	// 原文 Level 6 的 Tempo Clue Chop Move 範例搬到終局（分數 13、牌堆 11 張：pace 2，不是 Level 6 的拖延情況）：
	// Alice 紅色點到 Bob 的 r1、r2，Bob 打出 r1，Cathy「2」只補上 r2 的資訊。
	// Lv7 照樣 Chop Move Bob 的 chop；Lv8 終局不 Chop Move，Bob 照樣打出 r2
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 12, "y3 g3 b3 w3", "y4 g4 b4 w4 g4", "r1 r2 y4 w4 b4", "y4 b4 w4 g4 b4")
		force(t, g, game.HintColorTo(1, game.Red))
		force(t, g, game.Play(0))
		force(t, g, game.HintRankTo(1, 2))
		chop := g.Hands[1][4].Card.ID
		bots[0].Act(g.View(0))
		if got := bots[0].s.notes[chop].cm; got != (lv < 8) {
			t.Errorf("Lv%d：Alice 把 Bob 的 chop 記成 Chop Moved = %v", lv, got)
		}
		g.Current = 1
		if a := step(t, g, bots); a != game.Play(1) {
			t.Errorf("Lv%d：Bob 應打出 r2（slot 2），卻做了 %+v", lv, a)
		}
	}
}

func TestL8NoScreamInEndGame(t *testing.T) {
	// 挑戰題 3 Cards Left 的局面（分數 21、牌堆 3 張：pace 2）：提示 0 個，Alice 有大家都知道的 r4 卻棄 chop。
	// Lv7 是 Scream Discard（Bob 必須給提示）；Lv8 終局沒有 Scream Discard，Bob 照常打出能打的 5
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 3, "r3 y5 g5 b4 w4", "g1 g1 y1 b1 r4", "g3 r5 w5 b5 y2", "r2 r1 y4 y3 g2")
		g.Hints = 0
		known(t, g, bots, 0, 4, "r4")
		for slot, c := range []string{"r5", "w5", "b5"} {
			known(t, g, bots, 1, slot+1, c)
		}
		force(t, g, game.Discard(3))
		chop := g.Hands[1][4].Card.ID
		a := step(t, g, bots)
		if got := a.Type == game.ActHint; got != (lv < 8) {
			t.Errorf("Lv%d：Bob 給提示 = %v（%+v）", lv, got, a)
		}
		if lv >= 8 && a != game.Play(2) && a != game.Play(3) {
			t.Errorf("Lv%d：Bob 應打出 w5 或 b5，卻做了 %+v", lv, a)
		}
		bots[2].Act(g.View(2))
		if got := bots[2].s.notes[chop].cm; got != (lv < 8) {
			t.Errorf("Lv%d：Cathy 把 Bob 的 chop 記成 Chop Moved = %v", lv, got)
		}
	}
}
