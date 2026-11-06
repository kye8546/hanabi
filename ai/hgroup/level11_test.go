package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 11（Bluffs）的範例情境，取自 H-Group 原文 level-11.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy、3 = Donald；手牌字串由左（slot 1）到右（chop）。原文是 4 張手牌，3 人局這裡是 5 張。

// negInfo 讓每個 bot 把座位 p 第 slot 張牌記成之前被別的提示排除了 excl 這些牌面（負向資訊）。必須在任何人行動之前呼叫。
func negInfo(t *testing.T, g *game.Game, bots []*Bot, p, slot int, excl idSet) {
	t.Helper()
	id := g.Hands[p][slot].Card.ID
	for i, b := range bots {
		if b.s == nil {
			b.s = newState(g.View(i), b.level)
			b.s.sync(g.View(i))
		}
		n := b.s.notes[id]
		n.explicit &^= excl
		n.poss &^= excl
	}
}

func TestL11Bluff(t *testing.T) {
	// 原文 The Bluff：所有 2 都已打出。Alice 紅色點 Cathy 的 r4，Bob 看不到 r3，以為自己被 finesse，盲打出 b3。
	// 藍色接不上紅色，是 Bluff：Cathy 知道自己的紅牌是差一張就能打的 r4，不打；Bob 也不再往下盲打。
	// Lv10 沒有 Bluff：Cathy 把紅牌當直接的 r3 打出
	for _, lv := range []int{10, 11} {
		g, bots := scenario6(t, lv, "r2 y2 g2 b2 w2", "y4 g4 w4 y1 g1", "b3 y4 g4 w4 y1", "w1 r4 g1 b1 b4")
		g.Hints = 5
		force(t, g, game.HintColorTo(2, game.Red))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 3 {
			t.Fatalf("Lv%d：Bob 應盲打出 b3，卻做了 %+v", lv, a)
		}
		a := step(t, g, bots)
		if got := a == game.Play(1); got != (lv < 11) {
			t.Fatalf("Lv%d：Cathy 打出紅牌 = %v（%+v，%s）", lv, got, a, bots[2].Why())
		}
		if lv < 11 {
			continue
		}
		if got := noteOfCard(t, bots[2], g, 2, "r4"); got != idents(t, "r4") {
			t.Errorf("Cathy 應把紅牌記成 r4，筆記卻是 %v", got.list())
		}
		bots[1].Observe(g.View(1))
		if got := noteOfCard(t, bots[1], g, 2, "r4"); got != idents(t, "r4") {
			t.Errorf("Bob 應把 Cathy 的紅牌記成 r4，筆記卻是 %v", got.list())
		}
		force(t, g, game.Discard(4)) // Alice
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Errorf("Bluff 已經結束，Bob 不該再盲打（%+v，%s）", a, bots[1].Why())
		}
	}
}

func TestL11SelfBluff(t *testing.T) {
	// 原文 The Self-Bluff：r2 已打出。Alice「4」點 Bob 的 r4，Bob 看不到別人有能打的牌，盲打 finesse position，是 b1。
	// 1 接不上 4：Self-Bluff，Bob 的 4 就是差一張就能打的 r4，不再往下盲打（以為是 b4、繼續盲打 b2、b3 就打錯了）
	g, bots := scenario6(t, 11, "r2", "g2 b2 w4 y4 g4", "b1 y3 r4 g3 w3", "y3 g3 b3 w2 y2")
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Bob 應盲打出 b1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	bots[1].Act(g.View(1))
	if got := noteOfCard(t, bots[1], g, 1, "r4"); got != idents(t, "r4") {
		t.Errorf("Bob 應知道自己的 4 是 r4，筆記卻是 %v", got.list())
	}
	bots[2].Act(g.View(2))
	if got := noteOfCard(t, bots[2], g, 1, "r4"); got != idents(t, "r4") {
		t.Errorf("Cathy 應把 Bob 的 4 記成 r4，筆記卻是 %v", got.list())
	}
	step(t, g, bots) // Cathy
	step(t, g, bots) // Alice
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Errorf("Self-Bluff 已經結束，Bob 不該再打牌（%+v，%s）", a, bots[1].Why())
	}
}

func TestL11BluffThroughClued(t *testing.T) {
	// 原文 Bluffs Through Already-Clued Cards：所有 1 都已打出，Cathy 有一張只被「3」點過的 r3。Alice 紅色點 Donald 的 r4，
	// Bob 以為要 finesse 自己的 r2、prompt Cathy 的 3 當 r3，盲打出 b2：Bluff。r4 差兩張，不能當 Bluff 的目標，
	// 所以 Cathy 的 3 就是 r3（r4 經過它只差一張）。Cathy 不盲打（Lv10 會以為 r2 在自己的 finesse position）
	for _, lv := range []int{10, 11} {
		g, bots := scenario6(t, lv, "r1 y1 g1 b1 w1", "y4 g4 b4 w4", "b2 y4 g3 w3", "y1 g1 r3 b1", "w1 r4 y1 g1")
		g.Hints = 5
		preClued(t, g, bots, 2, 2, idents(t, "r3 y3 g3 b3 w3"))
		force(t, g, game.HintColorTo(3, game.Red))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 2 {
			t.Fatalf("Lv%d：Bob 應盲打出 b2，卻做了 %+v", lv, a)
		}
		a := step(t, g, bots)
		if got := a == game.Play(0); got != (lv < 11) {
			t.Fatalf("Lv%d：Cathy 盲打 finesse position = %v（%+v，%s）", lv, got, a, bots[2].Why())
		}
		if lv < 11 {
			continue
		}
		for _, p := range []int{1, 3} {
			bots[p].Observe(g.View(p))
		}
		for _, p := range []int{1, 2, 3} {
			if got := noteOfCard(t, bots[p], g, 2, "r3"); got != idents(t, "r3") {
				t.Errorf("座位 %d 應把 Cathy 的 3 記成 r3，筆記卻是 %v", p, got.list())
			}
		}
		for _, p := range []int{1, 2, 3} {
			if got := noteOfCard(t, bots[p], g, 3, "r4"); got != idents(t, "r4") {
				t.Errorf("座位 %d 應把 Donald 的紅牌記成 r4，筆記卻是 %v", p, got.list())
			}
		}
	}
}

func TestL11RankConnectFinesseImpossible(t *testing.T) {
	// 原文 Example 6：r2、b2 已打出。Cathy 的 4 之前被排除了藍色。Alice「4」點它，Bob 盲打出 b3：3 接得上 4，
	// 但她的 4 不可能是 b4，所以是 Bluff，那張是 r4
	g, bots := scenario6(t, 11, "r2 b2", "y4 g4 w4 y3 g3", "b3 y4 g4 w4 y1", "y1 r4 g1 w1 y2")
	g.Hints = 5
	negInfo(t, g, bots, 2, 1, idents(t, "b1 b2 b3 b4 b5"))
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 3 {
		t.Fatalf("Bob 應盲打出 b3，卻做了 %+v（%s）", a, bots[1].Why())
	}
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Errorf("Cathy 的 4 是 r4，不該打牌（%+v，%s）", a, bots[2].Why())
	}
	if got := noteOfCard(t, bots[2], g, 2, "r4"); got != idents(t, "r4") {
		t.Errorf("Cathy 應把 4 記成 r4，筆記卻是 %v", got.list())
	}
}

func TestL11RankConnectIsFinesse(t *testing.T) {
	// 原文 Example 3：r2 已打出。Alice「4」點 Cathy 的 r4，Bob 盲打出 r3：3 接得上 4，是 finesse，Cathy 的 4 是 r4、能打
	g, bots := scenario6(t, 11, "r2", "y4 g4 w4 y3 g3", "r3 y4 g4 w4 y1", "y1 r4 g1 w1 y2")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Bob 應盲打出 r3，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 4 {
		t.Errorf("Cathy 應打出 r4，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL11LayeredFromBluffSeatIsBluff(t *testing.T) {
	// 原文 Mistaking a Layered Finesse for a Bluff：Bluff Seat 上的 Bluff 優先於 Layered Finesse。r1 已打出，
	// Alice 紅色點 Cathy 的 r3，Bob 的 finesse position 是 b1、後面才是 r2。Bob 盲打出 b1：接不上紅色、r3 是差一張的牌，
	// 是 Bluff——Bob 不再往下盲打（Lv10 會照 Layered Finesse 把 slot 2 記成要盲打的 r2），Cathy 知道是 r3
	for _, lv := range []int{10, 11} {
		g, bots := scenario6(t, lv, "r1", "y4 g4 b4 w4 y3", "b1 r2 g4 w4 y4", "y2 r3 g3 w3 b3")
		g.Hints = 5
		force(t, g, game.HintColorTo(2, game.Red))
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Lv%d：Bob 應盲打出 b1，卻做了 %+v", lv, a)
		}
		if a := step(t, g, bots); a.Type == game.ActPlay {
			t.Fatalf("Lv%d：Cathy 的 r3 還不能打，卻做了 %+v（%s）", lv, a, bots[2].Why())
		}
		bots[1].Observe(g.View(1))
		r2 := g.Hands[1][1].Card.ID
		if got := bots[1].s.notes[r2].finessed; got != (lv < 11) {
			t.Errorf("Lv%d：Bob 把 slot 2 記成要繼續盲打 = %v", lv, got)
		}
		if lv >= 11 {
			if got := noteOfCard(t, bots[2], g, 2, "r3"); got != idents(t, "r3") {
				t.Errorf("Cathy 應把紅牌記成 r3，筆記卻是 %v", got.list())
			}
		}
	}
}

func TestL11GuidePrinciple(t *testing.T) {
	// 原文 Guide Principle：什麼都沒打出。Alice 紅色點 Donald 的 r2；Cathy 的 chop 是 g5。Bob 看不到 r1，
	// 不論是 finesse 還是 Bluff 都要現在盲打——Alice 在引導他，Cathy 的 chop 由 Alice 負責（她看到 Cathy 有提示可給）。
	// Lv10 的 Urgency Principle 例外會讓 Bob 先救 g5
	g, bots := scenario6(t, 11, "", "y3 g3 b3 w3", "r1 y4 g4 b1", "y4 g4 b4 g5", "y2 b2 r2 w4")
	g.Hints = 5
	force(t, g, game.HintColorTo(3, game.Red))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
		t.Errorf("Bob 應盲打出 r1，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL11TruthPrinciple(t *testing.T) {
	// 原文 Bob's Truth Principle：所有 1 都已打出，Cathy 的 finesse position 是 b2。Alice 藍色點 Donald 的 b3：
	// 可能是 finesse Cathy，Bob 就當成 finesse（真話），不盲打；Cathy 盲打出 b2
	g, bots := scenario6(t, 11, "r1 y1 g1 b1 w1", "y4 g4 w4 y3", "r4 y4 g4 w3", "b2 g3 r3 w4", "y2 b3 r2 g2")
	g.Hints = 5
	force(t, g, game.HintColorTo(3, game.Blue))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 應當成 finesse Cathy，不盲打，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 2 {
		t.Errorf("Cathy 應盲打出 b2，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL11GiveBluff(t *testing.T) {
	// 給 Bluff（原文 The Bluff 的情境，Alice 是 AI）：所有 2 都已打出，Bob 的 finesse position 是 b3，Cathy 有 r4，沒人有 r3。
	// Lv11 的 Alice 用紅色點 Cathy 的 r4：Bob 盲打出 b3，Cathy 知道 r4 差一張就能打，一個提示換到一張打出的牌與一張被拿到的 r4
	// （直接提示 Bob 的 b3 只換到一張）。Lv10 沒有 Bluff，不會這樣給
	for _, lv := range []int{10, 11} {
		g, bots := scenario6(t, lv, "r2 y2 g2 b2 w2", "y4 g4 w4 y1 g1", "b3 y4 g4 w4 y1", "w1 r4 g1 b1 y1")
		g.Hints = 5
		a := step(t, g, bots)
		bluff := a == game.HintColorTo(2, game.Red)
		if bluff != (lv >= 11) {
			t.Fatalf("Lv%d：Alice 用紅色 Bluff Cathy = %v（%+v，%s）", lv, bluff, a, bots[0].Why())
		}
		if !bluff {
			continue
		}
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 3 {
			t.Fatalf("Bob 應盲打出 b3，卻做了 %+v（%s）", a, bots[1].Why())
		}
		if a := step(t, g, bots); a == game.Play(1) {
			t.Fatalf("Cathy 不該打出 r4（%s）", bots[2].Why())
		}
		if got := noteOfCard(t, bots[2], g, 2, "r4"); got != idents(t, "r4") {
			t.Errorf("Cathy 應把紅牌記成 r4，筆記卻是 %v", got.list())
		}
	}
}

func TestL11GiveSelfBluff(t *testing.T) {
	// 給 Self-Bluff（原文 The Self-Bluff 的情境，Alice 是 AI）：r2 已打出，Bob 的 finesse position 是 b1，r4 差一張就能打、沒人有 r3，
	// Cathy 沒有值得提示的牌。Lv11 的 Alice 用「4」點 Bob 的 r4：Bob 盲打出 b1，知道自己的 4 是 r4。Lv10 直接用藍色點 b1
	for _, lv := range []int{10, 11} {
		g, bots := scenario6(t, lv, "r2", "g2 b2 w4 y4 g4", "b1 y3 r4 g3 w3", "r1 r1 y4 g4 b4")
		g.Hints = 5
		a := step(t, g, bots)
		bluff := a == game.HintRankTo(1, 4)
		if bluff != (lv >= 11) {
			t.Fatalf("Lv%d：Alice 用「4」Self-Bluff Bob = %v（%+v，%s）", lv, bluff, a, bots[0].Why())
		}
		if !bluff {
			continue
		}
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Bob 應盲打出 b1，卻做了 %+v（%s）", a, bots[1].Why())
		}
		bots[1].Observe(g.View(1))
		if got := noteOfCard(t, bots[1], g, 1, "r4"); got != idents(t, "r4") {
			t.Errorf("Bob 應知道自己的 4 是 r4，筆記卻是 %v", got.list())
		}
	}
}
