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
func TestL9FiveStallClosestToChop(t *testing.T) {
	// 原文 5 Stalls Closest to Chop（位置對調）：Early Game，Alice 只剩 5 Stall 可給。Bob 的 g5 在 slot 3（離 chop 兩張），
	// Cathy 的 w5 在 slot 4（離 chop 一張）。Lv9 必須 5 Stall 最靠近 chop 的那張，提示 Cathy；Lv8 照分數提示比較近的 Bob
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "y4 g4 b4 w4 r4", "y3 b3 g5 y4 w3", "b3 y3 g3 w5 b4")
		a := step(t, g, bots)
		want := game.HintRankTo(1, 5)
		if lv >= 9 {
			want = game.HintRankTo(2, 5)
		}
		if a != want {
			t.Errorf("Lv%d：Alice 應 %+v，卻做了 %+v", lv, want, a)
		}
	}
}

func TestL9EarlyGameNextMustFiveStall(t *testing.T) {
	// Early Game：Bob 的 chop 是 y2，Bob 能給的提示只有 Cathy 的 r1 和 Cathy slot 3 的 w5（5 Stall）。
	// Alice 給 Cathy 紅色（r1）。Lv8 認為用掉 r1 之後 Bob 就沒有提示可給、會棄 chop，先救 y2；
	// Lv9 的 Early Game 全隊必須做一次 5 Stall，Bob 會 5 Stall w5、不會棄牌，Alice 照樣給 play clue
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "g4 y4 b4 w4 g3", "y3 b3 g3 w3 y2", "r1 b3 w5 g3 y4")
		a := step(t, g, bots)
		saved := a == game.HintRankTo(1, 2) || a == game.HintColorTo(1, game.Yellow)
		if saved != (lv < 9) {
			t.Errorf("Lv%d：Alice 救 Bob 的 y2 = %v（%+v）", lv, saved, a)
		}
		if lv >= 9 && a != game.HintColorTo(2, game.Red) && a != game.HintRankTo(2, 1) {
			t.Errorf("Lv%d：Alice 應提示 Cathy 的 r1，卻做了 %+v", lv, a)
		}
	}
}

func TestL9EightClueSave(t *testing.T) {
	// 原文 8 Clue Save：提示滿 8（不是第一回合）時，點到不在 slot 1 的新牌可以只是救牌。
	// Alice 提示、Bob 棄牌回到 8 個提示、Cathy 打出 y1；Alice 紅色點到 Cathy slot 4 的 r2，而 Bob 的 finesse position 是 r1。
	// Lv8 讀成 finesse Bob 的 r1，Bob 盲打；Lv9 是 8 Clue Save，Bob 不盲打
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "g4 y4 b4 w4 g3", "r1 b3 y3 w3 g3", "y1 b4 w4 r2 b3")
		force(t, g, game.HintRankTo(2, 4))
		force(t, g, game.Discard(4))
		force(t, g, game.Play(0))
		force(t, g, game.HintColorTo(2, game.Red))
		if a := step(t, g, bots); (a == game.Play(0)) != (lv < 9) {
			t.Errorf("Lv%d：Bob 盲打 = %v（%+v）", lv, a == game.Play(0), a)
		}
	}
}

// lockBob 讓 Bob 鎖手：slot 1 只知道是紅色，slot 2–3 是 5，slot 4–5 是 4。
func lockBob(t *testing.T, g *game.Game, bots []*Bot) {
	t.Helper()
	preClued(t, g, bots, 1, 0, idents(t, "r1 r2 r3 r4 r5"))
	preClued(t, g, bots, 1, 1, idents(t, "r5 y5 g5 b5 w5"))
	preClued(t, g, bots, 1, 2, idents(t, "r5 y5 g5 b5 w5"))
	preClued(t, g, bots, 1, 3, idents(t, "r4 y4 g4 b4 w4"))
	preClued(t, g, bots, 1, 4, idents(t, "r4 y4 g4 b4 w4"))
}

func TestL9AnxietyPlay(t *testing.T) {
	// 原文 The Anxiety Play：Bob 鎖手、提示 0 個——隊友刻意讓他停在 0 個提示，表示他有一張牌能打。
	// 最可能能打的是只知道是紅色的 slot 1（r1）。Lv9 打出它；Lv8 照舊棄牌
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "g4 y4 b4 w4 g3", "r1 g5 y5 b4 w4", "y3 b3 g3 w3 y4")
		g.Hints, g.Current = 0, 1
		lockBob(t, g, bots)
		a := step(t, g, bots)
		if got := a == game.Play(0); got != (lv >= 9) {
			t.Errorf("Lv%d：Bob 打出 slot 1 = %v（%+v）", lv, got, a)
		}
	}
}

func TestL9LockedHandSave(t *testing.T) {
	// 原文 Locked Hand Save：鎖手的 Bob 用紅色點到 Cathy chop 上的 r2（不是 critical），Alice 的 finesse position 是 r1。
	// Lv8 讀成 finesse Alice 的 r1；Lv9 只是救下 Cathy 的 chop，Alice 不盲打
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "r1 y4 b4 w4 g3", "g5 y5 b5 w5 r5", "y4 b4 w4 g4 r2")
		for slot := range 5 {
			known(t, g, bots, 1, slot, []string{"g5", "y5", "b5", "w5", "r5"}[slot])
		}
		g.Hints, g.Current = 5, 1
		force(t, g, game.HintColorTo(2, game.Red))
		force(t, g, game.HintRankTo(0, 4)) // Cathy 給個無關的提示
		if a := step(t, g, bots); (a == game.Play(0)) != (lv < 9) {
			t.Errorf("Lv%d：Alice 盲打 = %v（%+v）", lv, a == game.Play(0), a)
		}
	}
}

func TestL9FeedLockedPlayer(t *testing.T) {
	// 同上，但 Bob 的紅牌是 r3，Anxiety Play 會打錯。提示 0 個，Alice 有大家都知道的 y1：
	// Lv9 的 Alice 不能讓他停在 0 個提示，棄牌換一個提示給他；Lv8 照常打出 y1
	for _, lv := range []int{8, 9} {
		g, bots := scenarioAt(t, lv, "", "y1 y4 b4 w4 g3", "r3 g5 y5 b4 w4", "y3 b3 g3 w3 y4")
		g.Hints = 0
		lockBob(t, g, bots)
		known(t, g, bots, 0, 0, "y1")
		a := step(t, g, bots)
		if got := a.Type == game.ActDiscard; got != (lv >= 9) {
			t.Errorf("Lv%d：Alice 棄牌 = %v（%+v）", lv, got, a)
		}
	}
}

func TestL9SaveCriticalOverFeed(t *testing.T) {
	// 使用者回報 20261030-120740 t30（2 人局）：提示剩 1 個，使用者除了 chop 上剛抽到的 Y5 都已被提示、沒牌可打。
	// 用最後一個提示救 Y5 會讓他鎖手、停在 0 個提示（Anxiety Play 可能打錯）；改棄牌換提示的話，他沒事可做，
	// 下一手就棄掉 Y5（確定少分）。應該救 Y5
	const codes = "h11 p2 h1W p3 h12 p1 h11 p2 h1W p4 h15 p2 d4 h0Y p3 h01 p4 h05 d3 h02 p1 d5 h1B p3 d3 d5 h1R h0Y d1"
	g, bots := replayCodes(t, 2, 1793333260744621000, 9, codes)
	a := bots[1].Act(g.View(1))
	if a.Type != game.ActHint || a.Target != 0 || !a.Hint.Matches(g.Hands[0][0].Card) {
		t.Errorf("AI-1 應該提示使用者救下 Y5，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL9FillInNoPrompt(t *testing.T) {
	// 使用者回報 20261030-140936 t15（2 人局）：提示滿 8，使用者用「3」補上 AI-1 被黃色點過的 chop 牌的資訊（Fill-In Clue）。
	// AI-1 另一張黃牌是 Y4，Fill-In Clue 不是 play clue，不能讀成 prompt 那張黃牌當 Y2 而打出它
	const codes = "h11 p1 h15 h02 d3 p4 d3 h05 d3 h0Y d3 d3 h1Y d2 h13 h0G d2"
	g, bots := replayCodes(t, 2, 1793340576724361000, 9, codes)
	a := bots[1].Act(g.View(1))
	if a.Type == game.ActPlay && a.Slot == 1 {
		t.Errorf("AI-1 不應該把 Y4 當成被 prompt 的 Y2 打出（%s）", bots[1].Why())
	}
}

func TestL9PlayInsteadOfFeed(t *testing.T) {
	// 使用者回報 20261030-145128 t42（2 人局）：提示剩 1 個，用它救使用者 chop 上的 B2 會讓他鎖手、停在 0 個提示。
	// AI-1 有確定能打的 W5：打出它不用掉提示（還換回一個），不該棄掉已提示的 B3 換提示
	const codes = "h1R h0B p3 h01 p1 h0W p5 h05 h15 p3 d4 h01 p1 h04 d2 d5 d2 h05 h13 d2 h1B d1 h1Y p1 h1G d1 h15 d1 h11 d1 h15 d1 h11 d1 h1B d3 h1W p1 p4 d1 h1W"
	g, bots := replayCodes(t, 2, 1793343088261705000, 9, codes)
	if a := bots[1].Act(g.View(1)); a.Type != game.ActPlay || a.Slot != 0 {
		t.Errorf("AI-1 應該打出 W5，卻做了 %+v（%s）", a, bots[1].Why())
	}
}
