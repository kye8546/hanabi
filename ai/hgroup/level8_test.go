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

func TestL8PositionalDiscard(t *testing.T) {
	// 原文範例：只剩 r5、y5 還沒打出，牌堆空了、提示 0 個；Bob 有已提示的 y5，Cathy slot 2 是沒被提示的 r5。
	// Alice 看得到每一張還需要的牌，棄 slot 2（不是 chop）：Positional Discard。Bob 照常打 y5，Cathy 盲打 slot 2 的 r5
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 0, "r4 y4 g5 b5 w5", "g1 g1 b1 b1 w1", "g1 b1 w1 y5 g1", "g1 r5 b1 w1 g2")
		g.Hints = 0
		known(t, g, bots, 1, 3, "y5")
		a := step(t, g, bots)
		if want := (lv >= 8); (a == game.Discard(1)) != want {
			t.Fatalf("Lv%d：Alice 棄 slot 2 = %v，實際 %+v（%s）", lv, !want, a, bots[0].Why())
		}
		if lv < 8 {
			continue
		}
		if a := step(t, g, bots); a != game.Play(3) {
			t.Errorf("Bob 應打出 y5，卻做了 %+v（%s）", a, bots[1].Why())
		}
		if a := step(t, g, bots); a != game.Play(1) {
			t.Errorf("Cathy 應盲打 slot 2 的 r5，卻做了 %+v（%s）", a, bots[2].Why())
		}
	}
}

func TestL8PositionalMisplay(t *testing.T) {
	// 原文範例（4 人局）：所有 4 都已打出、牌堆 3 張、提示 0 個；Bob 有已提示的 g5、w5，Cathy 有已提示的 y5、b5，
	// Donald slot 4 是沒被提示的 r5。Alice 沒有已提示的牌，chop 就是 slot 4：棄它不會被當成 Positional Discard，
	// 改故意打錯它（Positional Misplay）。Bob、Cathy 照常打 5，Donald 盲打 slot 4 的 r5
	g, bots := scenarioEnd(t, 8, 3, "r4 y4 g4 b4 w4", "g1 g1 b1 b1", "g1 g5 w5 b1", "y5 g1 b5 b1", "g1 b1 w1 r5")
	g.Hints = 0
	for _, k := range []struct {
		p, slot int
		c       string
	}{{1, 1, "g5 y5 b5 r5 w5"}, {1, 2, "g5 y5 b5 r5 w5"}, {2, 0, "g5 y5 b5 r5 w5"}, {2, 2, "g5 y5 b5 r5 w5"}} {
		preClued(t, g, bots, k.p, k.slot, idents(t, k.c))
	}
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Alice 應打錯 chop（Positional Misplay），卻做了 %+v（%s）", a, bots[0].Why())
	}
	for p := 1; p <= 2; p++ {
		if a := step(t, g, bots); a.Type != game.ActPlay || !bots[p].s.notes[g.Log[len(g.Log)-2].Card.ID].clued {
			t.Errorf("座位 %d 應打出已提示的 5，卻做了 %+v（%s）", p, a, bots[p].Why())
		}
	}
	if a := step(t, g, bots); a != game.Play(3) {
		t.Errorf("Donald 應盲打 slot 4 的 r5，卻做了 %+v（%s）", a, bots[3].Why())
	}
}

func TestL8DoublePositionalMisplay(t *testing.T) {
	// Alice 打錯不是 chop 的 slot 2（Positional Discard 也行得通，卻多花一個閃電）：Double Positional Misplay，
	// slot 2 有能打的牌的最後兩位（Bob 的 r5、Cathy 的 y5）都盲打。
	// （打錯 finesse position（slot 1）不算：多半只是以為自己被 finesse）
	g, bots := scenarioEnd(t, 8, 0, "r4 y4 g5 b5 w5", "g1 g1 b1 b1 w1", "b1 r5 w1 g1 g1", "b1 y5 w1 g1 g2")
	g.Hints = 0
	force(t, g, game.Play(1))
	if a := step(t, g, bots); a != game.Play(1) {
		t.Errorf("Bob 應盲打 slot 2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a != game.Play(1) {
		t.Errorf("Cathy 應盲打 slot 2，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL8DistributionClue(t *testing.T) {
	// 終局（分數 21、牌堆 1 張：pace 0）：Bob 手上有大家都知道的 r3、r4、r5、g5，只剩兩回合打不完。
	// Cathy 有另一張 r3：Alice「3」只點到它（其他 3 都已打出，提示本身只可能是 r3，和 Bob 的 r3 重複）。
	// Lv7 照 Good Touch 不會是 r3，Cathy 不打；Lv8 是 Distribution Clue，Cathy 打出 r3 分擔 Bob 的牌
	for _, lv := range []int{7, 8} {
		g, bots := scenarioEnd(t, lv, 1, "r2 y5 g4 b5 w5", "y1 g1 b1 w1 y1", "r3 r4 r5 g5 y2", "y2 r3 b2 w2 g2")
		g.Hints = 2
		for slot, c := range []string{"r3", "r4", "r5", "g5"} {
			known(t, g, bots, 1, slot, c)
		}
		if lv >= 8 {
			if a := bots[0].Act(g.View(0)); a != game.HintRankTo(2, 3) {
				t.Errorf("Lv8：Alice 應給 Cathy「3」（Distribution Clue），卻做了 %+v（%s）", a, bots[0].Why())
			}
		}
		force(t, g, game.HintRankTo(2, 3))
		a := step(t, g, bots)
		if got := a == game.Play(0); got == (lv >= 8) {
			t.Errorf("Lv%d：Bob 打出 r3 = %v（%+v；Lv8 他的 r3 已分給 Cathy，應先打別張）", lv, got, a)
		}
		a = step(t, g, bots)
		if got := a == game.Play(1); got != (lv >= 8) {
			t.Errorf("Lv%d：Cathy 打出 r3 = %v（%+v，%s）", lv, got, a, bots[2].Why())
		}
	}
}

func TestL8ThreeCardsLeft(t *testing.T) {
	// 挑戰題 3 Cards Left：牌堆 3 張、提示 0 個，Alice 有已知的 r4，Bob 有已知的 r5、w5、b5，其他人的牌都沒用。
	// 打出 r4 不會換回提示，Bob 的三張 5 來不及打完；Alice 該棄牌（留著 r4），之後大家拖延，整隊拿到滿分
	g, bots := scenarioEnd(t, 8, 3, "r3 y5 g5 b4 w4", "g1 g1 y1 b1 r4", "g3 r5 w5 b5 y2", "r2 r1 y4 y3 g2")
	g.Hints = 0
	known(t, g, bots, 0, 4, "r4")
	for slot, c := range []string{"r5", "w5", "b5"} {
		known(t, g, bots, 1, slot+1, c)
	}
	if a := step(t, g, bots); a.Type != game.ActDiscard {
		t.Fatalf("Alice 應棄牌、留著 r4，卻做了 %+v（%s）", a, bots[0].Why())
	}
	for !g.Over {
		step(t, g, bots)
	}
	if g.Score() != 25 {
		t.Errorf("應拿到滿分，實際 %d 分", g.Score())
	}
}

func TestL8FiveCardsLeft(t *testing.T) {
	// 挑戰題 5 Cards Left：牌堆 5 張、提示 1 個，只有 Alice 的 y5、g5 被提示。給 Cathy 紅色（finesse Bob 的 r4）會浪費
	// Alice 的回合：她看不到的 b5、w5 可能被她抽到，最壞要一個人打 4 張 5。Alice 該打出 y5 或 g5，紅色交給 Bob、Cathy。
	// 牌堆放進 b5、w5，大家每回合都抽牌時正好被 Alice 抽到（從牌堆尾端抽）。
	// 提示 2 個時也一樣（終局不為了「只有我能給的 finesse」延後打牌）；Lv7 沒有終局的概念，提示 2 個時會先給 finesse
	for _, c := range []struct{ level, hints int }{{8, 1}, {8, 2}, {7, 2}} {
		g, bots := scenarioEnd(t, c.level, 5, "r3 y4 g4 b4 w4", "g1 g1 y5 g5 y1", "r4 b1 b2 w3 y1", "w2 w1 y4 r5 g1")
		g.Deck[1], g.Deck[4] = card(t, "w5"), card(t, "b5")
		g.Deck[1].ID, g.Deck[4].ID = 200, 201
		g.Hints = c.hints
		known(t, g, bots, 0, 2, "y5")
		known(t, g, bots, 0, 3, "g5")
		a := step(t, g, bots)
		if plays := a == game.Play(2) || a == game.Play(3); plays != (c.level >= 8) {
			t.Errorf("Lv%d 提示 %d 個：Alice 打出 y5 或 g5 = %v，做了 %+v（%s）", c.level, c.hints, plays, a, bots[0].Why())
		}
	}
}

func TestNoSaveTooLateInLastRound(t *testing.T) {
	// 自我對局 2 人局 seed 507 第 68 手：牌堆已空，Bob 已經沒有回合，Alice 以前還花提示救他 chop 上的 y4，
	// 自己確定能打的 y3 沒打到。最後一輪救不到的牌不必救（常識，所有等級）
	const codes = "h15 h05 d4 d5 h1Y d3 h12 d2 h15 d3 h12 d3 h1G d1 h11 p1 h11 p1 d4 h0R p1 d1 h11 p1 h1B d2 h11 p1 d4 d1 h1G h05 h12 p1 d3 h03 p4 p2 d2 h0G p1 h03 h1W d2 h1G p1 d4 p3 h1R p1 p4 d5 d2 h0W p1 d4 p3 d4 h11 p1 h13 p1 h1B p4 p5 h02 p3 d3"
	for _, lv := range []int{1, 8} {
		g, bots := replayCodes(t, 2, 507, lv, codes)
		if len(g.Deck) != 0 || g.Current != 0 {
			t.Fatalf("局面不對：牌堆 %d、輪到 P%d", len(g.Deck), g.Current)
		}
		if a := bots[0].Act(g.View(0)); a != game.Play(4) {
			t.Errorf("Lv%d：Alice 應該打出 y3（位置 5），做了 %+v（%s）", lv, a, bots[0].Why())
		}
	}
}

func TestGiverKnownByTrashElimination(t *testing.T) {
	// 使用者回報 20261028-145155 t50：AI-1 只被「3」點過的牌，Y3、B3 都已沒用，大家都知道是 W3。
	// pace 1、AI-2 手上 W2、W5、B5 三張要打，AI-1 一棄牌他就來不及打完，必須拖延；以前模擬「4」點使用者的 W4 時，
	// 以為使用者不知道 AI-1 那張是 W3、會 self-finesse 自己的牌，判成誤導，找不到拖延的提示而棄牌
	const codes = "h11 p5 h01 p2 p3 h15 h25 d5 d5 d5 d4 h0Y p1 h2G h02 p1 p1 h1R p1 p3 p1 p4 d4 h14 p4 p1 h13 h1G p2 h0R p1 p5 d3 d5 p3 h0G p1 d5 h1R d5 p1 h01 d3 d2 h0B p2 h0R d3 h2W"
	for _, lv := range []int{1, 8} {
		g, bots := replayCodes(t, 3, 1793170315843172000, lv, codes)
		if g.Current != 1 || len(g.Deck) != 4 {
			t.Fatalf("局面不對：牌堆 %d、輪到 P%d", len(g.Deck), g.Current)
		}
		if a := bots[1].Act(g.View(1)); a != game.HintRankTo(0, 4) {
			t.Errorf("Lv%d：AI-1 應該提示使用者「4」（W4 等 W2、W3 打出），做了 %+v（%s）", lv, a, bots[1].Why())
		}
	}
}

func TestBurnWhenLastDrawStrandsChain(t *testing.T) {
	// 使用者回報 20261028-164449 t53：牌堆剩 1 張、提示 5 個，只差 W4、W5。W5 在使用者手上，W4 只可能在 AI-1 手上或牌堆。
	// AI-1 棄掉已知垃圾就抽走最後一張，最後一輪使用者在 AI-1 之前，W4 來不及在 W5 之前打出；應該拖延
	const codes = "h11 p2 h12 h25 p4 h15 d5 d2 d3 h12 d2 p1 h2B p1 p5 h21 p3 p1 h2R p5 p4 h15 h23 h03 p2 h25 d2 p1 h04 p3 p2 h0B d3 p1 h2Y p2 d5 h0Y p4 p4 p4 d4 d3 p5 d4 h22 d5 p1 h1B p5 p5 p5"
	for _, lv := range []int{1, 8} {
		g, bots := replayCodes(t, 3, 1793177089331801000, lv, codes)
		if len(g.Deck) != 1 || g.Current != 1 {
			t.Fatalf("局面不對：牌堆 %d、輪到 P%d", len(g.Deck), g.Current)
		}
		if a := bots[1].Act(g.View(1)); a.Type != game.ActHint {
			t.Errorf("Lv%d：AI-1 應該給提示拖延，做了 %+v（%s）", lv, a, bots[1].Why())
		}
	}
}

func TestEndGameCountsOnlyTimelyPlays(t *testing.T) {
	// 使用者回報 20261029-144249 t74（Rainbow）：只剩 W3、W4、W5，牌堆 1 張、提示 1 個；W3、W4 都在使用者手上，
	// W5 在 AI-2 手上。白色點 W5（Reverse Finesse 使用者的 W3）和「3」直接點 W3 都只來得及打出 W3，
	// 直接的提示不必靠對方讀懂 finesse。以前把 W5 也算成會打出，選了白色
	codes := "h21 d5 p5 h11 p4 p2 h1B p3 d5 h22 p5 d5 h2G h21 p4 h24 d5 h0Y d4 h0G p2 p5 d5 p3 d5 d4 p4 d4 h04 h05 p1 h0G h0B p1 p5 h14 p3 h2R d4 p1 d5 p2 d3 h03 h01 p5 p2 d4 h24 p1 p5 d3 h02 d5 p1 h2B p1 h24 d4 p1 h14 p1 d2 h25 h05 h0Y p2 h2R h1G p2 h0R h05 p1"
	for _, lv := range []int{1, 8} {
		g, bots := replayCodesVariant(t, 3, game.RainbowSix, 1793256169623026000, lv, codes)
		if a := bots[1].Act(g.View(1)); a != game.HintRankTo(0, 3) {
			t.Errorf("Lv%d AI-1 應直接用「3」提示使用者的 W3，卻做了 %+v（%s）", lv, a, bots[1].Why())
		}
	}
}

func TestBurnWithTrashOnlyClue(t *testing.T) {
	// 使用者回報 20261030-095942 t58（2 人局）：pace 0、提示 6 個，使用者手上只剩「1」點過的已知垃圾 W1、R1、已知的 R5，
	// 沒被提示的 B4、W3（chop）。白色點到 W1 與 W3，全是大家都知道的垃圾，終局只是拖延；以前被當成 W1 的 Tempo Clue、
	// 沒人看得懂，找不到拖延提示而棄牌
	const codes = "h11 p1 h1G h0W p5 p5 d5 h01 p2 d4 h1Y p1 h1R p2 h12 d3 h11 p1 d4 h0B p3 d2 h1R p1 h15 d2 h1G p1 d4 p5 d4 p4 d4 h02 h1W d3 p5 p2 h1Y p1 d5 h03 p5 d2 d5 h05 d4 d2 d4 h01 h1W p3 d4 h0R p1 h05 p1"
	for _, lv := range []int{8, 9} {
		g, bots := replayCodes(t, 2, 1793325582766795000, lv, codes)
		if a := bots[1].Act(g.View(1)); a != game.HintColorTo(0, game.White) {
			t.Errorf("Lv%d：AI-1 應該用白色拖延，做了 %+v（%s）", lv, a, bots[1].Why())
		}
	}
}

func TestPositionalMisplayKnownTrash(t *testing.T) {
	// 使用者回報 20261102-094044 t61（Black 3 人局）：提示 0 個、牌堆 3 張，使用者 slot 4 的 G1、slot 5 的 R1 都是大家都知道
	// 沒用的已提示牌，該棄的是左邊那張 G1（已知垃圾由左而右棄），棄它只是一般的棄牌；故意打錯它就是 Positional Misplay，
	// AI-2 同一位置的 G5（他只知道是「G5、B5 或 W5」）要盲打
	const codes = "h1G h2R p1 h1Y h0B p2 p3 p1 p5 h22 p5 p1 h14 h0G p1 p2 p1 h02 d2 h0R d5 p4 d5 h1K h15 d1 d5 d4 d1 h11 " +
		"d3 p1 h0W d3 p1 h1B p5 p1 h02 h1B p1 h04 p4 p2 d5 p4 h01 d5 h22 p5 h0R p4 p4 d1 h2Y p5 p1 h24 h25 p1 p4"
	g, bots := replayCodesVariant(t, 3, game.BlackSix, 1793583644267462000, 9, codes)
	step(t, g, bots)
	if a := bots[2].Act(g.View(2)); a.Type != game.ActPlay || a.Slot != 3 {
		t.Errorf("AI-2 應該回應 Positional Misplay 盲打 slot 4 的 G5，卻做了 %+v（%s）", a, bots[2].Why())
	}
}
