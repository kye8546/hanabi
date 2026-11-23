package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 13（Intermediate Bluffs）的範例情境，取自 H-Group 原文 level-13.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy、3 = Donald；手牌字串由左（slot 1）到右（chop）。3 人局是 5 張手牌。

func TestL13ColorDisconnect3Bluff(t *testing.T) {
	// 原文 The 3 Bluff Example 1（Color Disconnect）：第一回合 Alice 紅色點 Cathy 的 r3，Bob 盲打出 b1。
	// Cathy 知道是 r2（一般的 Bluff）或 r3（3 Bluff），兩種都記下；Bob 也不再往下盲打。
	// Lv12 沒有 3 Bluff：r3 差兩張不是合法的 Bluff 目標，Cathy 只記成 r2，看得到 r3 的 Bob 照 Layered Finesse 繼續盲打
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "", "y4 g4 w4 y3 g3", "b1 y4 g4 w4 b4", "g4 r3 w4 y4 b4")
		g.Hints = 5
		force(t, g, game.HintColorTo(2, game.Red))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Lv%d：Bob 應盲打出 b1，卻做了 %+v（%s）", lv, a, bots[1].Why())
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Lv%d：Cathy 的紅牌還不能打，卻做了 %+v（%s）", lv, a, bots[2].Why())
		}
		want := idents(t, "r2")
		if lv >= 13 {
			want = idents(t, "r2 r3")
		}
		if got := noteOfCard(t, bots[2], g, 2, "r3"); got != want {
			t.Errorf("Lv%d：Cathy 應把紅牌記成 %v，筆記卻是 %v", lv, want.list(), got.list())
		}
		force(t, g, game.Discard(4)) // Alice
		if a := step(t, g, bots); (a.Type == game.ActPlay) != (lv < 13) {
			t.Errorf("Lv%d：Bob 繼續盲打 = %v（%+v，%s）", lv, a.Type == game.ActPlay, a, bots[1].Why())
		}
	}
}

func TestL13Number3Bluff(t *testing.T) {
	// 原文 The 3 Bluff Example 2（Number 3 with Suit Disconnect）：第一回合 Alice「3」點 Cathy 的 r3，Bob 盲打出 b1。
	// 1 接不上 3：3 Bluff，Cathy 只知道那是某張 3
	g, bots := scenario6(t, 13, "", "y4 g4 w4 y4 g4", "b1 y4 g4 w4 b4", "g4 r3 w4 y4 b4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Bob 應盲打出 b1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 3 還不能打，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if got, want := noteOfCard(t, bots[2], g, 2, "r3"), idents(t, "r3 y3 g3 b3 w3"); got != want {
		t.Errorf("Cathy 應只知道是某張 3（%v），筆記卻是 %v", want.list(), got.list())
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Errorf("3 Bluff 已經結束，Bob 不該再盲打（%+v，%s）", a, bots[1].Why())
	}
}

func TestL13Hard3Bluff(t *testing.T) {
	// 原文 The 3 Bluff Example 5（Number 3 With Suit Connect）/ The Hard 3 Bluff：第一回合 Alice「3」點 Cathy 的 r3，
	// Bob 盲打出 r1。1 接不上 3，是 3 Bluff：Bob 沒有被保證有 r2，不再往下盲打，Cathy 只知道是某張 3。
	// Lv12 沒有 3 Bluff：當成 Double Finesse，Bob 接著盲打 r2
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "", "y4 g4 w4 y4 g4", "r1 r2 g4 w4 b4", "g4 r3 w4 y4 b4")
		g.Hints = 5
		force(t, g, game.HintRankTo(2, 3))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
			t.Fatalf("Lv%d：Bob 應盲打出 r1，卻做了 %+v（%s）", lv, a, bots[1].Why())
		}
		bots[1].Observe(g.View(1))
		r2 := g.Hands[1][1].Card.ID
		if got := bots[1].s.notes[r2].finessed; got != (lv < 13) {
			t.Fatalf("Lv%d：Bob 把 slot 2 記成要繼續盲打 = %v", lv, got)
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Lv%d：Cathy 的 3 還不能打，卻做了 %+v（%s）", lv, a, bots[2].Why())
		}
		if lv >= 13 {
			if got, want := noteOfCard(t, bots[2], g, 2, "r3"), idents(t, "r3 y3 g3 b3 w3"); got != want {
				t.Errorf("Cathy 應只知道是某張 3（%v），筆記卻是 %v", want.list(), got.list())
			}
		}
	}
}

func TestL13CriticalColorBluff(t *testing.T) {
	// 原文 The Critical Color Bluff：r1 已打出、一張 r4 已棄掉。Alice 紅色點 Cathy 的 r4（play clue），Bob 盲打出 b1。
	// Cathy 記成差一張就能打的 r3，或是 critical 的 r4（Critical Color Bluff）。Lv12 只記成 r3
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "r1", "y4 g4 w4 y3 g3", "b1 y4 g4 w4 b4", "g4 r4 w4 y4 b4")
		g.Discards = append(g.Discards, game.Card{ID: 999, Color: game.Red, Rank: 4})
		g.Hints = 5
		force(t, g, game.HintColorTo(2, game.Red))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Lv%d：Bob 應盲打出 b1，卻做了 %+v（%s）", lv, a, bots[1].Why())
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Lv%d：Cathy 的紅牌還不能打，卻做了 %+v（%s）", lv, a, bots[2].Why())
		}
		want := idents(t, "r3")
		if lv >= 13 {
			want = idents(t, "r3 r4")
		}
		if got := noteOfCard(t, bots[2], g, 2, "r4"); got != want {
			t.Errorf("Lv%d：Cathy 應把紅牌記成 %v，筆記卻是 %v", lv, want.list(), got.list())
		}
		force(t, g, game.Discard(4)) // Alice
		if a := step(t, g, bots); (a.Type == game.ActPlay) != (lv < 13) {
			t.Errorf("Lv%d：Bob 繼續盲打 = %v（%+v，%s）", lv, a.Type == game.ActPlay, a, bots[1].Why())
		}
	}
}

func TestL13NoCriticalColorBluffWithNumber(t *testing.T) {
	// Critical Color Bluff 只能用顏色：r1 已打出、一張 r4 已棄掉，Alice「4」點 Cathy 的 r4，Bob 盲打出 b1。
	// 1 接不上 4，但 r4 差兩張、又不是顏色提示，不是合法的 Bluff 目標：照 Layered Finesse 讀，Bob 繼續盲打
	g, bots := scenario6(t, 13, "r1", "y4 g4 w4 y3 g3", "b1 r2 r3 w4 b4", "g3 r4 w3 y3 b2")
	g.Discards = append(g.Discards, game.Card{ID: 999, Color: game.Red, Rank: 4})
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Bob 應盲打出 b1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	step(t, g, bots)             // Cathy
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 2 {
		t.Errorf("Bob 應照 Layered Finesse 繼續盲打 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL13HardBluff(t *testing.T) {
	// 原文 The Hard Bluff：r1、b2 已打出，Cathy 有一張只被「3」點過的 r3。Alice「4」點 Cathy 新的 r4，Bob 盲打出 r2。
	// 可能是 Bluff（4 是差一張的 b4），也可能是 finesse + prompt（4 是 r4、3 是 r3）：Cathy 兩種都記下，現在什麼都不打。
	// Lv12 只記成 b4
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "r1 b2", "y4 g4 w4 y4 g4", "r2 y4 g4 w4 y1", "g2 r4 r3 y2 w2")
		g.Hints = 5
		preClued(t, g, bots, 2, 2, idents(t, "r3 y3 g3 b3 w3"))
		force(t, g, game.HintRankTo(2, 4))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 2 {
			t.Fatalf("Lv%d：Bob 應盲打出 r2，卻做了 %+v（%s）", lv, a, bots[1].Why())
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Lv%d：Cathy 現在不該打牌，卻做了 %+v（%s）", lv, a, bots[2].Why())
		}
		want := idents(t, "b4")
		if lv >= 13 {
			want = idents(t, "r4 b4")
		}
		if got := noteOfCard(t, bots[2], g, 2, "r4"); got != want {
			t.Errorf("Lv%d：Cathy 應把 4 記成 %v，筆記卻是 %v", lv, want.list(), got.list())
		}
	}
}

func TestL13NotHardBluff(t *testing.T) {
	// 原文 The Hard Bluff 的前一個例子：只有 r1 已打出，Cathy 有一張只被「3」點過的 r3。Alice「4」點 Cathy 新的 r4，
	// Bob 盲打出 r2。沒有差一張就能打的 4，不是 Bluff：finesse + prompt，Cathy 把 3 當 r3 打出
	g, bots := scenario6(t, 13, "r1", "y4 g4 w4 y4 g4", "r2 y4 g4 w4 y1", "g2 r4 r3 y2 w2")
	g.Hints = 5
	preClued(t, g, bots, 2, 2, idents(t, "r3 y3 g3 b3 w3"))
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 應盲打出 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a != game.Play(2) || g.Fireworks[game.Red] != 3 {
		t.Errorf("Cathy 應把 3 當 r3 打出，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL13KnownBluff(t *testing.T) {
	// 原文 The Known Bluff：r1 已打出，Bob 每張牌都被排除了紅色。Alice 紅色點 Cathy 的 r3，Bob 知道自己沒有 r2，
	// 但看不出別的解釋：Cathy 照一般解讀會把它當 r2 打錯，所以 Bob 照樣盲打 finesse position（b1）。Cathy 知道是 r3。
	// Lv12 的 Bob 不盲打
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "r1", "y4 g4 w4 y3 g3", "b1 y4 g4 w4 b4", "g4 r3 w4 y4 b4")
		g.Hints = 5
		for slot := range 5 {
			negInfo(t, g, bots, 1, slot, idents(t, "r1 r2 r3 r4 r5"))
		}
		force(t, g, game.HintColorTo(2, game.Red))
		a := step(t, g, bots)
		if got := a == game.Play(0); got != (lv >= 13) {
			t.Fatalf("Lv%d：Bob 盲打 finesse position = %v（%+v，%s）", lv, got, a, bots[1].Why())
		}
		if lv < 13 {
			continue
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Cathy 的 r3 還不能打，卻做了 %+v（%s）", a, bots[2].Why())
		}
		if got := noteOfCard(t, bots[2], g, 2, "r3"); got != idents(t, "r3") {
			t.Errorf("Cathy 應把紅牌記成 r3，筆記卻是 %v", got.list())
		}
	}
}

func TestL13GoodTouchBluff(t *testing.T) {
	// 原文 The Good Touch Bluff：4 人局，r2、b2 已打出，Donald 有一張只被藍色點過的 b4。Alice「4」點 Cathy 的 r4，
	// Bob 盲打出 b3。3 接得上 4，但 b4 已在 Donald 手上被提示（Good Touch），所以是 Bluff：Cathy 的 4 是 r4，不打。
	// Donald 看到 Cathy 照 finesse 讀應該會把 r4 當 b4 打錯，Alice 卻給了這個提示：他自己的藍牌就是 b4
	g, bots := scenario6(t, 13, "r2 b2", "y4 g4 w4 y3", "b3 y4 g4 w4", "g3 r4 w3 y3", "b4 y1 g1 w1")
	g.Hints = 5
	preClued(t, g, bots, 3, 0, idents(t, "b1 b2 b3 b4 b5"))
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 3 {
		t.Fatalf("Bob 應盲打出 b3，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 4 是 r4，不該打牌，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if got := noteOfCard(t, bots[2], g, 2, "r4"); got != idents(t, "r4") {
		t.Errorf("Cathy 應把 4 記成 r4，筆記卻是 %v", got.list())
	}
	bots[3].Observe(g.View(3))
	if got := noteOfCard(t, bots[3], g, 3, "b4"); got != idents(t, "b4") {
		t.Errorf("Donald 應知道自己的藍牌是 b4，筆記卻是 %v", got.list())
	}
	if got := noteOfCard(t, bots[3], g, 2, "r4"); got != idents(t, "r4") {
		t.Errorf("Donald 應把 Cathy 的 4 記成 r4，筆記卻是 %v", got.list())
	}
}

func TestL13Give3Bluff(t *testing.T) {
	// 給 3 Bluff（原文：「3 Bluffs are mostly used to get 1's played at the beginning of the game」）：什麼都沒打出，
	// Bob 的 finesse position 是 b1，Cathy 有 r3，別處沒有能打的牌。Lv13 的 Alice 提示 Cathy 的 r3：Bob 盲打出 b1，
	// r3 也被拿到。Lv12 的 r3 差兩張不是合法的 Bluff 目標，不會這樣給
	for _, lv := range []int{12, 13} {
		g, bots := scenario6(t, lv, "", "y4 g4 w4 y4 g4", "b1 y4 g4 w4 b4", "g4 r3 w4 y4 b4")
		g.Hints = 5
		a := step(t, g, bots)
		bluff := a.Type == game.ActHint && a.Target == 2 && g.Hands[2][1].Know.Clued
		if bluff != (lv >= 13) {
			t.Fatalf("Lv%d：Alice 提示 Cathy 的 r3（3 Bluff）= %v（%+v，%s）", lv, bluff, a, bots[0].Why())
		}
		if !bluff {
			continue
		}
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Bob 應盲打出 b1，卻做了 %+v（%s）", a, bots[1].Why())
		}
		if a := step(t, g, bots); a == game.Play(1) {
			t.Fatalf("Cathy 不該打出 r3（%s）", bots[2].Why())
		}
	}
}
