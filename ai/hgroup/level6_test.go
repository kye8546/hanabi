package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 6（Tempo Clues）的範例情境，取自 H-Group 原文 level-6.mdx 與 Level 6 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。原文是 4 張手牌，這裡 3 人局是 5 張。

// scenario6 同 scenarioAt，但牌堆補到正常長度（scenarioAt 的牌堆只有 20 張，pace 太低，
// Level 6 會以為是終局、所有 Tempo Clue 都只是拖延）。
func scenario6(t *testing.T, level int, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	g, bots := scenarioAt(t, level, stacks, hands...)
	for i := range 20 {
		c := card(t, "g4")
		c.ID = 1000 + i
		g.Deck = append(g.Deck, c)
	}
	return g, bots
}

// preClued 讓每個 bot 把座位 p 第 slot 張牌記成之前就被提示過（提示本身的資訊是 explicit），
// 用來布置「已被提示、但還不知道確切牌面」的牌。必須在任何人行動之前呼叫。
func preClued(t *testing.T, g *game.Game, bots []*Bot, p, slot int, explicit idSet) {
	t.Helper()
	id := g.Hands[p][slot].Card.ID
	for i, b := range bots {
		if b.s == nil {
			b.s = newState(g.View(i), b.level)
			b.s.sync(g.View(i))
		}
		n := b.s.notes[id]
		n.clued, n.touches, n.giver = true, 1, (p+1)%len(bots)
		n.explicit = explicit & b.s.all
		n.poss = n.explicit & b.s.useful()
	}
}

func TestL6TempoClueChopMove(t *testing.T) {
	// 原文範例：Alice 紅色點到 Bob 的 slot 1、2，Bob 打出 r1；Cathy「2」只補上 slot 2（r2）的資訊、沒點到新牌：
	// 只讓一張牌能打，沒有價值 → Tempo Clue Chop Move，Bob 的 chop（slot 5）被 Chop Move，他打出 r2
	for _, lv := range []int{5, 6} {
		g, bots := scenario6(t, lv, "", "y4 g4 b4 w4 g3", "r1 r2 y3 w3 b3", "y3 b4 w4 g3 b4")
		force(t, g, game.HintColorTo(1, game.Red))
		force(t, g, game.Play(0))
		force(t, g, game.HintRankTo(1, 2))
		chop := g.Hands[1][4].Card.ID
		bots[0].Act(g.View(0))
		if got := bots[0].s.notes[chop].cm; got != (lv >= 6) {
			t.Errorf("Lv%d：Alice 把 Bob 的 chop 記成 Chop Moved = %v", lv, got)
		}
		g.Current = 1
		if a := step(t, g, bots); a != game.Play(1) {
			t.Errorf("Lv%d：Bob 應打出 r2（slot 2），卻做了 %+v", lv, a)
		}
		if got := bots[1].s.notes[chop].cm; got != (lv >= 6) {
			t.Errorf("Lv%d：Bob 把自己的 chop 記成 Chop Moved = %v", lv, got)
		}
	}
}

func TestL6FillingInII(t *testing.T) {
	// 挑戰題 Filling in, II：Bob 只有一張已提示的綠牌（g3），Alice「3」補上它的資訊：只讓一張牌能打，
	// 沒有價值 → Bob 的 chop 被 Chop Move，他打出 g3
	g, bots := scenario6(t, 6, "r4 y2 g2 b2 w5", "y4 g4 b4 w4 r1", "y4 b4 w4 y5 g3", "y1 b1 w1 r1 g1")
	g.Hints = 5
	preClued(t, g, bots, 1, 4, colorSet(game.Green))
	force(t, g, game.HintRankTo(1, 3))
	chop := g.Hands[1][3].Card.ID
	if a := step(t, g, bots); a != game.Play(4) {
		t.Errorf("Bob 應打出 g3（slot 5），卻做了 %+v", a)
	}
	if !bots[1].s.notes[chop].cm {
		t.Error("Bob 的 chop（slot 4）應被 Chop Move")
	}
}

func TestL6TempoClueStallIsNotChopMove(t *testing.T) {
	// Tempo Clue Stall：Alice 提示滿 8（不能棄牌），沒有價值的 Tempo Clue 只是拖延，不 Chop Move
	g, bots := scenario6(t, 6, "r4 y2 g2 b2 w5", "y4 g4 b4 w4 r1", "y4 b4 w4 y5 g3", "y1 b1 w1 r1 g1")
	g.Hints = game.MaxHints
	preClued(t, g, bots, 1, 4, colorSet(game.Green))
	force(t, g, game.HintRankTo(1, 3))
	chop := g.Hands[1][3].Card.ID
	bots[2].Act(g.View(2))
	if bots[2].s.notes[chop].cm {
		t.Error("提示滿 8 時的 Tempo Clue 是 Tempo Clue Stall，不應 Chop Move")
	}
}

func TestL6FillingInI(t *testing.T) {
	// 挑戰題 Filling in, I：Bob 有已提示的藍牌（slot 4）與綠牌（slot 5），Alice「3」把兩張補成 b3、g3：
	// 讓兩張牌能打，是 Valuable Tempo Clue，焦點是最左邊的 b3，不 Chop Move
	g, bots := scenario6(t, 6, "r4 y2 g2 b2 w5", "y4 g4 b4 w4 r1", "y4 b4 w4 b3 g3", "y1 b1 w1 r1 g1")
	g.Hints = 5
	preClued(t, g, bots, 1, 3, colorSet(game.Blue))
	preClued(t, g, bots, 1, 4, colorSet(game.Green))
	force(t, g, game.HintRankTo(1, 3))
	bots[2].Act(g.View(2))
	if bots[2].s.notes[g.Hands[1][2].Card.ID].cm {
		t.Error("Valuable Tempo Clue 不應 Chop Move")
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || a.Slot != 3 && a.Slot != 4 {
		t.Errorf("Bob 應打出 b3 或 g3，卻做了 %+v", a)
	}
}

func TestL6TempoClueCannotBePrompted(t *testing.T) {
	// Valuable Tempo Clue 的第二種：Bob 有兩張已提示的紅牌，「2」補上右邊那張是 r2——現在 prompt r2 會 prompt 到
	// 左邊那張，沒辦法用 prompt 讓它打出，所以有價值，不 Chop Move
	g, bots := scenario6(t, 6, "r1", "y4 g4 b4 w4 g3", "y4 r4 b4 r2 w4", "y3 b4 w4 g3 b4")
	g.Hints = 5
	preClued(t, g, bots, 1, 1, colorSet(game.Red))
	preClued(t, g, bots, 1, 3, colorSet(game.Red))
	force(t, g, game.HintRankTo(1, 2))
	bots[2].Act(g.View(2))
	if bots[2].s.notes[g.Hands[1][4].Card.ID].cm {
		t.Error("沒辦法用 prompt 讓它打出的 Tempo Clue 有價值，不應 Chop Move")
	}
	if a := step(t, g, bots); a != game.Play(3) {
		t.Errorf("Bob 應打出 r2（slot 4），卻做了 %+v", a)
	}
}

func TestL6TempoOrTrash(t *testing.T) {
	// 挑戰題 Tempo or Trash?：r2 已打出，Alice 的 r4、Cathy 的 r3 都已被提示，Bob 的 slot 5 是被救下的 5（r5）。
	// Alice 紅色點到 Bob 新的 slot 1（垃圾 r1）與 slot 5：Tempo Clue 優先於 Trash Chop Move，
	// 只 Chop Move slot 4（TCCM），slot 2、3 不動，slot 1 是已知垃圾
	g, bots := scenario6(t, 6, "r2", "y4 g4 r4 w4 g3", "r1 y4 b4 g4 r5", "y3 r3 w3 g3 b4")
	g.Hints = 5
	preClued(t, g, bots, 0, 2, idents(t, "r4"))
	preClued(t, g, bots, 2, 1, idents(t, "r3"))
	preClued(t, g, bots, 1, 4, rankSet(5))
	force(t, g, game.HintColorTo(1, game.Red))
	bots[1].Act(g.View(1))
	for slot, want := range []bool{false, false, false, true} {
		if got := cmOf(bots[1], g, 1, slot); got != want {
			t.Errorf("Bob 的 slot %d 被 Chop Move = %v，應為 %v", slot+1, got, want)
		}
	}
	if !bots[1].s.notes[g.Hands[1][0].Card.ID].bad {
		t.Error("Bob 的 slot 1 應記成已知垃圾")
	}
}

func TestL6FocusShifting(t *testing.T) {
	// Focus Shifting：Bob 的 slot 1 是已知的 r1、slot 2 是已提示的 3。紅色點到兩張：最左邊那張已經知道能打，
	// 焦點移到 slot 2（r3），要 finesse Cathy 的 r2；Lv5 焦點是 r1，slot 2 只是「紅 3」，Cathy 不盲打
	for _, lv := range []int{5, 6} {
		g, bots := scenario6(t, lv, "y1 g1 b1 w1", "y4 g4 b4 w4 g3", "r1 r3 y4 b4 w4", "r2 b4 w4 g3 y3")
		g.Hints = 5
		preClued(t, g, bots, 1, 0, rankSet(1))
		preClued(t, g, bots, 1, 1, rankSet(3))
		force(t, g, game.HintColorTo(1, game.Red))
		if a := step(t, g, bots); a != game.Play(0) {
			t.Fatalf("Lv%d：Bob 應打出 r1，卻做了 %+v", lv, a)
		}
		a := step(t, g, bots)
		if blind := a == game.Play(0); blind != (lv >= 6) {
			t.Errorf("Lv%d：Cathy 盲打 r2 = %v（行動 %+v）", lv, blind, a)
		}
	}
}

func TestL6DiscardModulation(t *testing.T) {
	// 原文 Discard Modulation：1 全打出、Mid-Game，Bob 的 chop 是 r3（差一張就能打，不能直接救），Cathy 有能打的 b2。
	// Alice 不自己提示 b2，改棄牌，讓 Bob 去提示（他就不會棄掉 r3）；Lv5 直接提示 Cathy
	for _, lv := range []int{5, 6} {
		g, bots := scenario6(t, lv, allOnes, "y4 g4 b4 w4 y3", "y4 w4 b4 r3 w3", "b4 b2 y3 w3 y4")
		midGame(t, g)
		a := step(t, g, bots)
		if discard := a.Type == game.ActDiscard; discard != (lv >= 6) {
			t.Errorf("Lv%d：Alice 棄牌 = %v（行動 %+v）", lv, discard, a)
		}
	}
}

func TestL6OneAwayChopMove(t *testing.T) {
	// The Value of One-Away-From-Playable Cards：Bob 的 chop 是 r3（差一張就能打），不能直接救，但能用 Trash Chop Move 保護
	// （「1」點到 slot 4 的垃圾 y1）。沒有 play clue 可給時 Alice 這樣做；Lv5 不會
	for _, lv := range []int{5, 6} {
		g, bots := scenario6(t, lv, allOnes, "y4 g4 b4 w4 y3", "g4 y4 w4 y1 r3", "y4 b4 w4 g3 b4")
		g.Hints = 5 // 提示滿 8 時是拖延，Lv5 也可能選它
		a := step(t, g, bots)
		if tcm := a == game.HintRankTo(1, 1); tcm != (lv >= 6) {
			t.Errorf("Lv%d：Alice 給 Trash Chop Move = %v（行動 %+v）", lv, tcm, a)
		}
	}
}

func TestL6FocusShiftOverStopClue(t *testing.T) {
	// 使用者的例子：第一回合 Alice 紅色點到 Cathy 的 slot 1、2，slot 1 大家都記成 r1（play clue 的焦點）。
	// 接著 Bob 再紅色點到同樣兩張：沒給新資訊，但 slot 1 已經知道能打，Focus Shifting 讓焦點移到 slot 2 → r2，
	// 是沒有價值的 Tempo Clue，同時 Chop Move Cathy 的 chop。不是叫她別打 r1 的 Stop Clue（Level 3 第 119 行）。
	// prompt 落點雖然只看提示資訊（slot 1 只被紅色點過），但 r1 本來就會被打出去，不擋 r2 的 prompt（使用者的判斷，playsAnyway）
	for _, lv := range []int{5, 6} {
		g, bots := scenario6(t, lv, "", "y4 g4 b4 w4 g3", "y3 b4 w4 g3 b4", "r1 r2 y3 w3 b3")
		force(t, g, game.HintColorTo(2, game.Red))
		force(t, g, game.HintColorTo(2, game.Red))
		r1, r2, chop := g.Hands[2][0].Card.ID, g.Hands[2][1].Card.ID, g.Hands[2][4].Card.ID
		bots[0].Act(g.View(0))
		s := bots[0].s
		if lv >= 6 {
			if s.notes[r1].bad {
				t.Error("Lv6：slot 1 的 r1 不該被當成 Stop Clue 記成壞牌")
			}
			if !s.notes[chop].cm {
				t.Error("Lv6：Cathy 的 chop 應被 Tempo Clue Chop Move")
			}
		} else if s.notes[chop].cm {
			t.Error("Lv5 沒有 Tempo Clue Chop Move")
		}
		g.Current = 2
		if a := step(t, g, bots); lv >= 6 && a != game.Play(0) {
			t.Errorf("Lv%d：Cathy 應先打出 r1，卻做了 %+v", lv, a)
		}
		if p := bots[2].s.notes[r2].poss; lv >= 6 && p != idents(t, "r2") {
			t.Errorf("Lv6：Cathy 應把 slot 2 記成 r2，卻是 %v", p.list())
		}
		if lv >= 6 && !bots[2].s.notes[chop].cm {
			t.Error("Lv6：Cathy 自己也應把 chop 記成 Chop Moved")
		}
	}
}

func TestL6TempoOnSecondOfSameCluedCardsIsValuable(t *testing.T) {
	// 使用者回報 20261020-120718 t24：使用者的 slot 4、5 都只被「3」點過（slot 4 由 t20 的「4」可知是 r3 / b3 / w3、已知能打）。
	// AI-2 白色點 slot 5：要 prompt w3 時會先落在 slot 4（它也可能是 w3），slot 5 輪不到——是有價值的 Tempo Clue，
	// 不 Chop Move 使用者的 chop（以前從使用者的角度跳過已知能打的 slot 4，讀成 TCCM，和兩個 AI 看法不一）
	codes := "h1B h01 p1 p5 p4 h12 p5 p1 d5 h2G h2Y p2 d5 d5 p2 h1R p4 h03 d4 h24 p1 d4 p5 h0W"
	g, bots := replayCodes(t, 3, 1792469238872685000, 6, codes)
	chop := g.Hands[0][2].Card.ID
	me := New("me", 6)
	me.Act(g.View(0))
	bots[1].Act(g.View(1))
	for name, b := range map[string]*Bot{"使用者": me, "AI-1": bots[1]} {
		if b.s.notes[chop].cm {
			t.Errorf("%s 不該把 slot 3 記成被 Chop Move", name)
		}
	}
}
