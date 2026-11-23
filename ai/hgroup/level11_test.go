package hgroup

import (
	"slices"
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

func TestBluffNotBlockedByReceiversWrongReading(t *testing.T) {
	// 使用者回報 20261112-142830：「3」點 AI-3 的 Y3，AI-3 以為可能是 R3、要 AI-1 盲打 R2；t9 使用者用藍色 Bluff AI-1
	// （AI-2 的 B3），t10 AI-1 盲打出 R2。AI-1、AI-2 和使用者都看得到 AI-3 那張是 Y3，那些解讀只是 AI-3 的猜測，
	// 這次盲打就是 Bluff。以前 AI-1 當成 Layered Finesse，t14 再把 slot 2 的 R2 當 B2 盲打錯；
	// 使用者的紅色也不能給（AI-3 那張被記成 R3，看得到 Y3 的使用者會把焦點當 R3 打出 R4）
	g, bots := replayCodes(t, 4, 1794464910083634000, 11, "h21 h3B p4 p3 h33 h35 p4 h1Y h2B p1 p4 p1 h14")
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("AI-1 不該再盲打（藍色是 Bluff），卻打了 slot %d（%s）", a.Slot+1, bots[1].Why())
	}
	if a.Type == game.ActHint && a.Target == 0 && a.Hint == (game.Hint{Kind: game.HintColor, Color: game.Red}) {
		t.Fatalf("AI-1 不該用紅色點使用者的 R4、R3（焦點 R4 會被當成 R3 打出）")
	}
}

func TestBlindPlayDuplicatingCluedCardIsNotALayer(t *testing.T) {
	// 同一局：AI-3 以為藍色是 Reverse Finesse 自己的 B2，t12 盲打出 Y2；AI-1 的 Y2 在藍色之前就被提示了，
	// 給提示者不會拿它當 layer，所以不是 Layered Finesse，AI-3 不該再把 slot 3 的 G4 當 B2 盲打
	g, bots := replayCodes(t, 4, 1794464910083634000, 11, "h21 h3B p4 p3 h33 h35 p4 h1Y h2B p1 p4 p1 h14 p2 h1W")
	if a := step(t, g, bots); a == game.Play(2) {
		t.Fatalf("AI-3 不該把 slot 3 當 B2 盲打（%s）", bots[3].Why())
	}
}

func TestFiveStallFallbackDoesNotBlockBluff(t *testing.T) {
	// 使用者回報 20261112-165325：t6 AI-2 的「5」是給 AI-1 的 5 Stall（fallback 留著「R5，self-finesse R1–R4」），
	// t7 使用者白色點 AI-2 的 W3、Bluff AI-1，t8 AI-1 盲打出 R1。以前 AI-1 因為那個 fallback 不當成 Bluff，
	// 一直把 AI-2 的 W3 記成「W2 或 W3」，t41 誤以為要 Double Finesse 自己而 Certain Discard
	g, bots := replayCodes(t, 3, 1794473605627249000, 11, "h2G p1 h0W p4 h25 h15 h2W p1")
	w3 := g.Hands[2][3].Card.ID
	step(t, g, bots)
	bots[1].Act(g.View(1))
	if got := bots[1].s.notes[w3].poss; got != idents(t, "w3") {
		t.Errorf("AI-1 應把 AI-2 被白色點到的牌記成 W3（Bluff），卻是 %v", got.list())
	}
}

func TestBluffDeniesDuplicateOfVisibleCluedCard(t *testing.T) {
	// 使用者回報 20261113-165056：t2 AI-1 用「1」點到 AI-3 的 K1、W1，t5 使用者用藍色 Bluff、AI-1 盲打出 R1。
	// 使用者看得到 AI-3 的 1，是 R1 的話不會讓 AI-1 盲打出重複的 R1，所以那張是 K1，AI-3 該打出它
	codes := "h21 h31 p2 p3 h3B p1 h0R"
	g, bots := replayCodesVariant(t, 4, game.BlackSix, 1794559856632818000, 11, codes)
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Black] != 1 {
		t.Fatalf("AI-3 該打出 K1，卻做了 %+v（%s）", a, bots[3].Why())
	}
}

func TestBluffNotBlockedByClueThatNeverUsesBluffee(t *testing.T) {
	// 使用者回報 20261117-102820 t30（Rainbow，4 人局）：t27 AI-2 的紅色點 AI-3 的 M1，AI-3 看不到，解讀是 M1 或
	// 「R4，Reverse Finesse 使用者的 R2」——都用不到 AI-1。t29 使用者用藍色點 AI-2 chop 上的 B4（B3 誰都沒有），
	// t30 AI-1 盲打出 R2：藍色接不上，是 Bluff。AI-3 以前以為 AI-1 可能在接紅色而不當成 Bluff，
	// 照「Reverse Finesse 自己的 B3」在 t32 盲打錯 W5
	const codes = "h3Y p1 h31 p3 h3Y p2 h1B p2 h1R p1 h35 p3 d4 p4 d4 h02 d4 h03 p1 h0G p2 h3G d4 d1 p4 d3 h3R p4 h2B p1 h04"
	g, bots := replayCodesVariant(t, 4, game.RainbowSix, 1794882500904103000, 11, codes)
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("t29 的藍色是 Bluff，AI-3 不該盲打 finesse position（%s）", bots[3].Why())
	}
	for p := 1; p < 4; p++ {
		bots[p].Observe(g.View(p))
		for _, r := range bots[p].s.recs {
			if r.giver == 0 && r.target == 2 && !r.bluff {
				t.Errorf("AI-%d 應把 t29 的藍色讀成 Bluff", p)
			}
		}
	}
}

func TestLayeredFinesseNotIllegalBluff(t *testing.T) {
	// 使用者回報 20261118-121252：t1 使用者紅色點 AI-3 的 R3，要 AI-1 盲打 W1（layer）、R1、R2。AI-1 坐在 Bluff Seat，
	// 打出接不上紅色的 W1，但 R3 差兩張、不是合法的 Bluff 目標（原文：「if Alice gives a clue to Bob which is not a legal Bluff
	// target, the clue should always be interpreted as a Layered Finesse」）——看得到 R3 的 AI-1 繼續盲打 R1、R2。
	// 看不到的 AI-3 先記成「R2 或 R3」，AI-1 繼續盲打後就知道是 R3。以前大家都讀成 Bluff（R2），AI-1 打完 W1 就停
	g, bots := replayCodes(t, 4, 1794975172511946000, 12, "h3R p1")
	r3 := g.Hands[3][1].Card.ID
	step(t, g, bots) // AI-2
	step(t, g, bots) // AI-3
	if g.Strikes > 0 || g.Fireworks[game.Red] != 0 {
		t.Fatalf("AI-3 的紅牌還不能打（%s）", bots[3].Why())
	}
	if got := bots[3].s.notes[r3].poss; !got.has(identOfCard(card(t, "r3"))) {
		t.Fatalf("AI-3 應把紅牌記成可能是 R3（Layered Finesse），筆記卻是 %v", got.list())
	}
	force(t, g, game.Discard(3)) // 使用者
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 1 {
		t.Fatalf("AI-1 應繼續盲打出 R1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	for p := 1; p < 4; p++ {
		bots[p].Observe(g.View(p))
		if got := bots[p].s.notes[r3].poss; got != idents(t, "r3") {
			t.Errorf("AI-%d 應把 AI-3 的紅牌記成 R3，筆記卻是 %v", p, got.list())
		}
	}
}

func TestBluffConfirmedWhenBluffeeStops(t *testing.T) {
	// 看不到焦點的接收者分不出 Bluff 與 Layered Finesse 時，盲打的人下一手不再盲打就是 Bluff。
	// Alice 紅色點 Donald 的 r2（差一張），Bob 盲打出 w1（接不上紅色）；Donald 看得到 Bob 的 w1 後面是 r1、r2，
	// 以為也可能是 r3（Layered Finesse）。Bob 看得到 r2，知道是 Bluff，下一手不盲打——Donald 就記成 r2
	g, bots := scenario6(t, 12, "", "y4 g4 b4 w4", "w1 r1 r2 b3", "y4 g3 b4 w3", "g4 b4 r2 y4")
	r2 := g.Hands[3][2].Card.ID
	force(t, g, game.HintColorTo(3, game.Red))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 1 {
		t.Fatalf("Bob 應盲打出 w1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	step(t, g, bots) // Cathy
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Donald 的紅牌還不能打（%s）", bots[3].Why())
	}
	if got := bots[3].s.notes[r2].poss; !got.has(identOfCard(card(t, "r2"))) || !got.has(identOfCard(card(t, "r3"))) {
		t.Errorf("Donald 還分不出是 Bluff 還是 Layered Finesse，應記成 r2 或 r3，筆記卻是 %v", got.list())
	}
	force(t, g, game.Discard(3)) // Alice
	hand := slices.Clone(g.Hands[1])
	if a := step(t, g, bots); a.Type == game.ActPlay && !hand[a.Slot].Know.Clued {
		t.Fatalf("Bob 看得到 r2，知道是 Bluff，不該再盲打，卻做了 %+v（%s）", a, bots[1].Why())
	}
	for p := 1; p < 4; p++ {
		bots[p].Observe(g.View(p))
		if got := bots[p].s.notes[r2].poss; got != idents(t, "r2") {
			t.Errorf("座位 %d 應把 Donald 的紅牌記成 r2（Bluff），筆記卻是 %v", p, got.list())
		}
	}
}

func TestBluffUndoesPromptNoteFromSeveralReadings(t *testing.T) {
	// r1、b2 已打出，Cathy 有一張只被「3」點過的 3（實際是 r3）。Alice「4」點 Cathy 新的 r4：提示當下 Cathy 的 3 被兩種解讀
	// 寫成「r3 或 b3」（r4：finesse Bob 的 r2、prompt 3 當 r3；b4：prompt 3 當 b3）。Bob 盲打出 r2，2 接不上 4，是 Bluff（b4）。
	// 撤掉連接筆記時，以前只撤「正好等於某一種解讀」的筆記，「r3 或 b3」留著；r2 打出後 r3、b3 都能打，Cathy 就把 3 打出
	g, bots := scenario6(t, 11, "r1 b2", "y4 g4 w4 y4 g4", "r2 y4 g4 w4 y1", "g2 r4 r3 y2 w2")
	g.Hints = 5
	preClued(t, g, bots, 2, 2, idents(t, "r3 y3 g3 b3 w3"))
	force(t, g, game.HintRankTo(2, 4))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 應盲打出 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Errorf("Cathy 不知道 3 是哪張，不該打牌，卻做了 %+v（%s）", a, bots[2].Why())
	}
}
