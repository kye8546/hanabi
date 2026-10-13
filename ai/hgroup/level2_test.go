package hgroup

import (
	"slices"
	"testing"

	"hanabi/game"
)

// Level 2（Basic Moves）的範例情境，取自 H-Group 原文 level-2.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy、3 = Donald；手牌字串由左（slot 1）到右（chop）。

func TestL2DoublePrompt(t *testing.T) {
	// r1 已打出，r3 棄掉一張（critical）。Alice 先提示 Bob 紅色（點到 slot 2 的 r2 與 chop 上的 r3，焦點在 chop，
	// 可能是救 r3 也可能是 r2，Bob 不打）。之後 Alice 提示 Cathy 紅色點到 r4：Bob 的兩張紅牌由左而右是 r2、r3。
	g, bots := scenarioAt(t, 2, "r1", "g4 y4 b4 w4 g3", "y4 r2 b4 g4 r3", "b3 r4 w3 g3 b4")
	g.Discards = append(g.Discards, card(t, "r3"))
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Red))
	force(t, g, game.Discard(0)) // Bob
	force(t, g, game.Discard(0)) // Cathy
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Double Prompt：Bob 應先打左邊的紅牌 r2（slot 2），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 r4 還要等 r3，不該打牌，卻做了 %+v", a)
	}
	force(t, g, game.Discard(0)) // Alice
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Double Prompt：Bob 接著應打出另一張紅牌 r3（slot 5），卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 3 {
		t.Fatalf("紅色應到 3，實際 %d", g.Fireworks[game.Red])
	}
}

func TestL2DoubleFinesse(t *testing.T) {
	// 4 人：r1 已打出。Alice 提示 Donald 紅色點到 r4 → Bob 盲打 r2、Cathy 盲打 r3、Donald 打出 r4
	g, bots := scenarioAt(t, 2, "r1", "g4 y4 b4 w4", "r2 y4 g4 b4", "r3 g4 y4 w4", "b3 r4 w3 g3")
	force(t, g, game.HintColorTo(3, game.Red))
	for _, who := range []string{"Bob", "Cathy"} {
		if a := step(t, g, bots); a != game.Play(0) {
			t.Fatalf("Double Finesse：%s 應從 finesse position 盲打，卻做了 %+v", who, a)
		}
	}
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Donald 應打出 r4（slot 2），卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 4 {
		t.Fatalf("紅色應到 4，實際 %d", g.Fireworks[game.Red])
	}
	if lv1, bots1 := scenarioAt(t, 1, "r1", "g4 y4 b4 w4", "r2 y4 g4 b4", "r3 g4 y4 w4", "b3 r4 w3 g3"); true {
		force(t, lv1, game.HintColorTo(3, game.Red))
		if a := step(t, lv1, bots1); a.Type == game.ActPlay {
			t.Fatalf("Level 1 沒有 Double Finesse，Bob 不該盲打，卻做了 %+v", a)
		}
	}
}

func TestL2DoubleFinesseSamePlayer(t *testing.T) {
	// 3 人、什麼都沒打出。Alice 提示 Cathy 紅色點到 r3 → Bob 盲打 r1；Cathy 看到提示當下 r1 旁邊有 r2，
	// 先等；Bob 再盲打 r2（現在在 slot 2）；Cathy 才打 r3。
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "r1 r2 g4 b4 w4", "r3 y4 g4 b4 w4")
	g.Hints = 5
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 應盲打 slot 1 的 r1，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 應等 Bob 有機會盲打 r2，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Bob 應接著盲打 slot 2 的 r2，卻做了 %+v", a)
	}
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 現在應打出 r3，卻做了 %+v（紅色 %d）", a, g.Fireworks[game.Red])
	}
}

func TestL2ReverseFinesse(t *testing.T) {
	// Alice 提示 Bob 紅色點到 r2；Cathy 的 finesse position 是 r1 → Bob 先等，Cathy 盲打 r1，Bob 再打 r2
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "r2 y4 g4 b4 w4", "r1 y3 r3 b3 r4")
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Reverse Finesse：Bob 應先等 Cathy 盲打，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Reverse Finesse：Cathy 應盲打 slot 1 的 r1，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 現在應打出 r2，卻做了 %+v（紅色 %d）", a, g.Fireworks[game.Red])
	}
}

func TestL2ReverseFinesseNotHappening(t *testing.T) {
	// 同上，但 Bob 被點到的是 r1：Bob 先等；Cathy 看得到 Bob 的是 r1，不盲打；Bob 於是知道自己的是 r1
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "r1 y4 g4 b4 w4", "r1 y4 g4 b4 w4")
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 應先等（可能是 Reverse Finesse），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("Cathy 看得到 Bob 的是 r1，不該盲打，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Red] != 1 || g.Strikes != 0 {
		t.Fatalf("Cathy 沒盲打，Bob 應打出自己的 r1，卻做了 %+v（紅色 %d）", a, g.Fireworks[game.Red])
	}
}

func TestL2SelfFinesse(t *testing.T) {
	// 1 都打出了。Alice 提示 Cathy 數字 3 點到 r3；誰的 finesse position 都沒有 2 → Cathy 盲打 slot 1 當成任何 2
	g, bots := scenarioAt(t, 2, "r1 y1 g1 b1 w1", "g4 y4 b4 w4 g5", "y4 g4 b4 w4 r4", "r2 r3 g4 b4 w4")
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 看得到 Cathy 的 finesse position 是 r2，不該盲打，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Self-Finesse：Cathy 應盲打 slot 1，卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 2 {
		t.Fatal("Cathy 的盲打應成功打出 r2")
	}
	bots[2].s.sync(g.View(2))
	if got := noteOfCard(t, bots[2], g, 2, "r3"); got != idents(t, "r3") {
		t.Errorf("Cathy 打出 r2 後，她的 3 應是 r3，筆記卻是 %v", got.list())
	}
}

func TestL2SelfFinesseAfterOtherReadingFails(t *testing.T) {
	// 同上，但 Bob 的 finesse position 是 b2：Cathy 先以為可能是 finesse Bob 的 b3。
	// Bob 沒盲打（他看得到 Cathy 有 r2）→ 其他解讀都不成立，Cathy 才盲打自己的 slot 1
	g, bots := scenarioAt(t, 2, "r1 y1 g1 b1 w1", "g4 y4 b4 w4 g5", "b2 g4 b4 w4 r4", "r2 r3 g4 b4 w4")
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 不該盲打，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 沒有盲打 b2，Cathy 應想到 Self-Finesse 而盲打 slot 1，卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 2 || g.Strikes != 0 {
		t.Fatal("Cathy 的盲打應成功打出 r2")
	}
}

func TestL2FiveStall(t *testing.T) {
	// Early Game、沒有 play / save clue 可給：Level 2 用 5 Stall 點 Bob 不在 chop 上的 5，而不是棄牌
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "y4 r5 b4 w4 g3", "y4 b3 w3 g3 b4")
	g.Hints = 7
	if a := step(t, g, bots); a != game.HintRankTo(1, 5) {
		t.Fatalf("Alice 應給 5 Stall，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("5 Stall 不是 play clue，Bob 不該打出，卻做了 %+v", a)
	}
	if got := noteOfCard(t, bots[1], g, 1, "r5"); got != idents(t, "r5 y5 g5 b5 w5") {
		t.Errorf("5 Stall 的筆記應為任何 5，得到 %v", got.list())
	}
	g, bots = scenarioAt(t, 1, "", "g4 y4 b4 w4 g3", "y4 r5 b4 w4 g3", "y4 b3 w3 g3 b4")
	g.Hints = 7
	if a := step(t, g, bots); a.Type != game.ActDiscard {
		t.Fatalf("Level 1 沒有 5 Stall，Alice 應棄牌，卻做了 %+v", a)
	}
}

func TestL2WrongPrompt(t *testing.T) {
	// r1 已打出。Alice 先用 2 Save 救 Bob chop 上的 y2；Bob 之後抽到 r2（slot 1）。
	// Alice 提示 Cathy 紅色點到 r3，想 finesse Bob 的 r2，卻忘了 Bob 有張已提示的 2：
	// Bob 依 Prompts > Finesses 打出那張 2（y2）而打錯，大家都看得出是 Wrong Prompt：
	// Bob 下一手盲打提示當下 finesse position 上的牌（現在在 slot 2），Cathy 不會跟著打錯。
	g, bots := scenarioAt(t, 2, "r1", "g4 y4 b4 w4 g3", "b4 g4 w4 b3 y2", "w4 r3 y4 g4 b4")
	r2 := card(t, "r2")
	r2.ID = 999
	g.Deck = append(g.Deck, r2)
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 2))
	force(t, g, game.Discard(0)) // Bob 抽到 r2
	force(t, g, game.Discard(0)) // Cathy
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應依 Prompts > Finesses 打出已提示的 2（slot 5），卻做了 %+v", a)
	}
	if g.Strikes != 1 {
		t.Fatal("這張 2 是 y2，應打錯")
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 應知道是 Wrong Prompt、等 Bob 盲打 r2，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Wrong Prompt：Bob 應盲打提示當下的 finesse position（現在的 slot 2），卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 2 || g.Strikes != 1 {
		t.Fatalf("Bob 應成功打出 r2（紅色 %d、閃電 %d）", g.Fireworks[game.Red], g.Strikes)
	}
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 現在應打出 r3，卻做了 %+v（紅色 %d）", a, g.Fireworks[game.Red])
	}
}

// noteOfCard 回傳 bot 對座位 p 手上（第一張）牌面為 c 的牌的筆記。
func noteOfCard(t *testing.T, b *Bot, g *game.Game, p int, c string) idSet {
	t.Helper()
	want := card(t, c)
	for _, hc := range g.Hands[p] {
		if hc.Card.Color == want.Color && hc.Card.Rank == want.Rank {
			return b.s.notes[hc.Card.ID].poss
		}
	}
	t.Fatalf("座位 %d 手上沒有 %s", p, c)
	return 0
}

func TestNoFinesseWhenBystanderLooksPrompted(t *testing.T) {
	// r1 已打出，Cathy 有一張被 2 Save 救下的 y2（她自己只知道是某張 2）。
	// Alice 若提示 Donald 紅色點到 r3 想 finesse Bob 的 r2，Cathy 會以為是 prompt 她的 2 而打錯：不該給
	for lv := 1; lv <= 2; lv++ {
		g, bots := scenarioAt(t, lv, "r1", "g4 y4 b4 w4", "r2 g4 b4 w4", "b4 g4 w4 y2", "y4 r3 g4 b4")
		g.Hints = 5
		force(t, g, game.HintRankTo(2, 2))
		force(t, g, game.HintRankTo(0, 4)) // Bob（不抽牌，r2 留在 finesse position）
		force(t, g, game.Discard(0))       // Cathy
		force(t, g, game.Discard(0))       // Donald
		if a := step(t, g, bots); a == game.HintColorTo(3, game.Red) {
			t.Fatalf("Lv%d：Cathy 會以為自己被 prompt，Alice 不該給這個 finesse", lv)
		}
	}
}

func TestGiverKnewOwnCardDespiteMyUnknownCard(t *testing.T) {
	// w1 已打出。Cathy 先提示 Alice「2」（w2）再提示白色（w3、w2）：Alice 知道 w3 是 delayed play clue。
	// Bob 看不到自己的 finesse position，從他的角度 Alice 的那張也可能是 w4（Reverse Finesse 他）。
	// Alice 接著提示 Bob 白色點到 w4。Alice 打出 w3 後 Bob 要推得出：他的白牌若是 w3，兩張 w3 都在
	// （Alice 的與他的），他的 finesse position 就不是 w3，Alice 當時就知道自己的是 w3，不會再提示一張 w3 → 是 w4
	g, bots := scenarioAt(t, 2, "w1", "y1 r1 w3 w2 r3", "y1 r4 r3 w4 b4", "w5 g4 y2 b1 r2")
	g.Hints = 3
	force(t, g, game.Discard(4))                 // Alice
	force(t, g, game.Discard(4))                 // Bob
	force(t, g, game.HintRankTo(0, 2))           // Cathy：w2
	force(t, g, game.Discard(0))                 // Alice
	force(t, g, game.Discard(0))                 // Bob（finesse position 是新抽的 g4）
	force(t, g, game.HintColorTo(0, game.White)) // Cathy：w3、w2
	force(t, g, game.HintColorTo(1, game.White)) // Alice：Bob 的 w4
	force(t, g, game.HintRankTo(2, 2))           // Bob
	force(t, g, game.Discard(4))                 // Cathy
	force(t, g, game.Play(4))                    // Alice 打出 w2
	force(t, g, game.HintRankTo(2, 2))           // Bob
	force(t, g, game.Discard(4))                 // Cathy
	force(t, g, game.Play(4))                    // Alice 打出 w3
	if g.Fireworks[game.White] != 3 {
		t.Fatalf("情境設定錯誤：白色應到 3，實際 %d", g.Fireworks[game.White])
	}
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應推得出自己的白牌是 w4 並打出（slot 5），卻做了 %+v", a)
	}
}

func TestDoNotSaveChopIfItExposesMoreImportantCard(t *testing.T) {
	// Cathy 的 chop 是 g2，左邊是 r5，她沒牌可打也沒提示可給（下一手會棄牌）。
	// 救 g2 只會讓她改棄 r5；Level 2 應改用 5 Stall 保住 r5、y5，讓她棄 g2
	g, bots := scenarioAt(t, 2, "", "w4 b2 y3 r2 y3", "g3 y2 b3 b4 w3", "y4 b4 y5 r5 g2")
	g.Current = 1
	if a := step(t, g, bots); a != game.HintRankTo(2, 5) {
		t.Fatalf("Bob 應給 Cathy 5 Stall 保住 r5，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestNoStompOnPossibleReverseFinesse(t *testing.T) {
	// r2、g2、w1 已打出。Alice 提示 Bob「3」點到 w3：Bob 看來可能是 r3、g3（直接打）或 w3（Reverse Finesse
	// Cathy finesse position 上的 w2）。Bob 該等 Cathy 盲打，而不是直接提示 Cathy 的 w2（Stomping on a Finesse）
	g, bots := scenarioAt(t, 2, "r2 g2 w1", "b3 b4 w4 w4 y4", "b1 w1 w3 r4 b5", "w2 w3 w1 w5 g4")
	g.Hints = 3
	force(t, g, game.HintRankTo(1, 3))
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("Bob 應先等（可能是 Reverse Finesse），卻做了 %+v", a)
	}
	if a.Type == game.ActHint && a.Target == 2 && a.Hint.Matches(g.Hands[2][0].Card) {
		t.Fatalf("Bob 不該直接提示 Cathy 可能要被盲打的 w2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 2 {
		t.Fatalf("Cathy 應盲打 w2，卻做了 %+v", a)
	}
}

func TestFinesseWhenGiversOwnConnectingCardIsUnknownToGiver(t *testing.T) {
	// b3、w2 已打出，w4 棄過一張（critical）。Bob 用「4」點 Alice chop 上的 b4：Alice 只知道是 b4（可打）或 w4（救）。
	// 之後 Alice 提示 Cathy 藍色點到 b5。Alice 不知道自己那張是 b4，不可能靠它：這是 finesse Bob 的 b4
	g, bots := scenarioAt(t, 2, "b3 w2", "r1 g1 y1 r2 b4", "b4 g3 r3 y3 g2", "y2 b5 r4 g4 w3")
	g.Discards = append(g.Discards, card(t, "w4"))
	g.Hints = 6
	g.Current = 1
	force(t, g, game.HintRankTo(0, 4)) // Bob
	force(t, g, game.Discard(4))       // Cathy
	force(t, g, game.HintColorTo(2, game.Blue))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 4 {
		t.Fatalf("Bob 應盲打 finesse position 的 b4，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestBlindPlayProvesPlayClueOverSave(t *testing.T) {
	// g1 已打出，g4 棄過一張（critical）。Alice 提示 Bob 綠色點到 chop 上的 g3 和另一張 g4：Bob 看來可能是
	// g2（直接打）、g3（Reverse Finesse Cathy 的 g2）或 g4（救 critical）。Cathy 盲打 g2 後，save clue 不會讓人盲打，
	// 所以是 play clue：Bob 的 chop 是 g3
	g, bots := scenarioAt(t, 2, "g1", "r4 y4 b3 y3 w3", "b4 g4 b1 r4 g3", "g2 g1 r3 w3 y1")
	g.Discards = append(g.Discards, card(t, "g4"))
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Green))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 應先等，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Green] != 2 {
		t.Fatalf("Cathy 應盲打 g2，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	a := step(t, g, bots)
	if a.Type != game.ActPlay || g.Fireworks[game.Green] != 3 {
		t.Fatalf("Bob 應打出 g3，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestGiverMayKnowViaMyCluedCard(t *testing.T) {
	// Bob 有一張被「1」點到的 r1（他不知道顏色）。Alice 提示 Cathy 紅色點到 r2：Cathy 看得到 Bob 的 r1，
	// 依 Good Touch 知道自己的是 r2。接著 Cathy 提示 Bob 紅色點到 r3（和那張 1）。Bob 看不到自己的 1，
	// 但「Cathy 那張是 r1」的可能，Cathy 看到 Bob 的 1 就能排除：Bob 不能斷定她不知道。
	// 所以這是靠 Cathy 的 r2 的 delayed play clue：Bob 的新紅牌是 r3，不是 r2
	g, bots := scenarioAt(t, 2, "", "y4 b4 g4 w4 y3", "w5 r3 b3 r1 g4", "g3 r2 b4 g4 y2")
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 1))         // Alice：Bob 的 r1
	force(t, g, game.Discard(0))               // Bob
	force(t, g, game.Discard(4))               // Cathy
	force(t, g, game.HintColorTo(2, game.Red)) // Alice：Cathy 的 r2
	force(t, g, game.Discard(4))               // Bob
	force(t, g, game.HintColorTo(1, game.Red)) // Cathy：Bob 的 r3 與 r1
	bots[1].s = newState(g.View(1), 2)
	bots[1].s.sync(g.View(1))
	if got := noteOfCard(t, bots[1], g, 1, "r3"); got != idents(t, "r3") {
		t.Fatalf("Bob 的新紅牌應是 r3（靠 Cathy 的 r2），筆記卻是 %v", got.list())
	}
}

func TestFinesseFallsToNextCandidateWhenFirstPasses(t *testing.T) {
	// 4 人：g1 已打出。Alice 提示 Cathy「3」點到 g3。Bob 和 Donald 的 finesse position 都是 g2。
	// Bob 看到 Donald 有 g2（可能是 Reverse Finesse）而不打；Donald 看到 Bob 沒打，知道 Bob 是看到了自己的 g2 → 盲打
	g, bots := scenarioAt(t, 2, "g1", "r4 y4 b4 w4", "g2 r3 y3 b3", "w1 g3 b4 r4", "g2 y4 w4 r4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 看到 Donald 也有 g2，應先不打，卻做了 %+v", a)
	}
	step(t, g, bots) // Cathy（可能打自己別的牌，但 g3 要等）
	if g.Fireworks[game.Green] != 1 || g.Strikes != 0 {
		t.Fatalf("Cathy 不該先打 g3")
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Green] != 2 {
		t.Fatalf("Bob 沒盲打，Donald 應知道是自己而盲打 g2，卻做了 %+v（%s）", a, bots[3].Why())
	}
}

func TestSelfFinesseWaitsWhenNotSurePlayable(t *testing.T) {
	// w1 已打出。Cathy 先提示 Alice 的 b1；Alice 再提示 Bob「3」點到 w3：可能是 b3（Alice 的 b1 接上後，
	// self-finesse b2）或 w3（self-finesse w2）。Bob 的 finesse position 可能是 b2 或 w2，b2 還不能打，所以先等——
	// 這不算沒接 finesse。Alice 打出 b1 後兩種都能打，Bob 盲打出 w2，他的 3 就確定是 w3
	g, bots := scenarioAt(t, 2, "w1", "b1 r4 y4 g4 r3", "w3 w2 r4 g4 y4", "g4 y3 r1 b4 b4")
	g.Current = 2
	force(t, g, game.HintColorTo(0, game.Blue)) // Cathy：Alice 的 b1
	force(t, g, game.HintRankTo(1, 3))          // Alice：Bob 的 w3
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 的 finesse position 可能是還不能打的 b2，應先等，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.Play(0))    // Alice 打出 b1
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.White] != 2 {
		t.Fatalf("Bob 應盲打 finesse position（w2），卻做了 %+v（%s）", a, bots[1].Why())
	}
	bots[1].s.sync(g.View(1))
	if got := noteOfCard(t, bots[1], g, 1, "w3"); got != idents(t, "w3") {
		t.Fatalf("Bob 盲打出 w2 後，他的 3 應是 w3，筆記卻是 %v", got.list())
	}
}

func TestTempoClueOnCluedFiveIsNotFiveStall(t *testing.T) {
	// Early Game。Bob 的 w5 已被 5 Stall；w4 在 Bob 自己手上已知（w2、w3 已打出）。Alice 再提示「5」只點到那張 5：
	// 這是 tempo clue（這張能打——接在 w4 之後），不是另一個 5 Stall
	g, bots := scenarioAt(t, 2, "w3", "r4 y4 b4 g4 r3", "y4 w5 r4 w4 g4", "g4 y3 r1 b4 b3")
	force(t, g, game.HintRankTo(1, 5))           // Alice：5 Stall
	force(t, g, game.HintColorTo(0, game.Blue))  // Bob（隨便）
	force(t, g, game.HintColorTo(1, game.White)) // Cathy：Bob 的 w5、w4（w4 能打）
	force(t, g, game.HintRankTo(1, 5))           // Alice：再點 5（tempo）
	bots[1].s = newState(g.View(1), 2)
	bots[1].s.sync(g.View(1))
	for _, r := range bots[1].s.recs {
		if r.giver == 0 && slices.ContainsFunc(r.cands, func(c cand) bool { return c.save }) && r.focus == g.Hands[1][1].Card.ID && len(bots[1].s.recs) > 0 && r == bots[1].s.recs[len(bots[1].s.recs)-1] {
			t.Fatalf("重新點到已提示的 5 不該被讀成 5 Stall：%+v", r.cands)
		}
	}
}

func TestRetryFinesseIgnoresCardsFromSameClue(t *testing.T) {
	// Alice 提示 Cathy 白色，點到 slot 1 的 w1（焦點）和 slot 3 的 w5。Cathy 看到 Bob 的 finesse position 是 w1，
	// 所以也可能是 w2（finesse Bob）。Bob 看得到焦點是 w1，不盲打。重新找連接牌時，同一個提示點到的 slot 3
	// 不能當成 w2 的連接牌（prompt），只剩 w1：Cathy 打出 w1。
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 r4 g3", "w1 y4 b4 r4 y3", "w1 g4 w5 y4 b4")
	g.Deck = append(g.Deck[:10:10], g.Deck...)
	force(t, g, game.HintColorTo(2, game.White))
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("Bob 看得到焦點是 w1，不該盲打")
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 1 {
		t.Fatalf("Cathy 應打出 w1（slot 1），卻做了 %+v", a)
	}
}

func TestStallClueMustNotLookLikeReverseFinesseToBystander(t *testing.T) {
	// 4 人局：Bob 的 y5 早被 5 Stall 救下（已知是 5）。Alice 若重新點它（黃色）拖延，提示本身沒有可行的計畫，
	// 但 Cathy 看不到自己的牌，會以為是 Reverse Finesse 她的 y1–y4 而盲打：這個提示會誤導，而且危險
	g, bots := scenarioAt(t, 2, "", "r3 g3 b3 w3", "r4 g4 y5 b4", "w4 r4 g4 b4", "w4 r3 g3 b3")
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 5))
	force(t, g, game.Discard(3)) // Bob
	force(t, g, game.Discard(3)) // Cathy
	force(t, g, game.Discard(3)) // Donald
	bots[0].Act(g.View(0))
	ev, ok := bots[0].s.evalClue(1, game.Hint{Kind: game.HintColor, Color: game.Yellow}, 1)
	if !ok || !ev.misleads || !ev.danger {
		t.Errorf("黃色重新點 Bob 的 y5 會讓 Cathy 盲打，應評為誤導且危險：misleads=%v danger=%v", ev.misleads, ev.danger)
	}
}

func TestGoodTouchIgnoresFinesseUnknownToGiver(t *testing.T) {
	// 4 人局：Alice 給 Bob 白色點到 w3（Reverse Finesse Cathy 的 w2）。Bob 看不到自己的牌，可能以為是直接的 w2，
	// 不知道 Cathy 的 w2 已被拿到。Bob 接著給 Donald「2」：Donald 不能依 Good Touch 排除自己那張是 w2
	// （Bob 不知道 Cathy 那張已被拿到，點到另一張 w2 不算違反 Good Touch）
	g, bots := scenarioAt(t, 2, "g1 w1", "r3 y3 b3 r4", "w3 y4 r4 b4", "w2 y3 g4 b4", "g2 r3 y4 b3")
	force(t, g, game.HintColorTo(1, game.White))
	force(t, g, game.HintRankTo(3, 2))
	bots[3].Act(g.View(3))
	if p := notesOf(bots[3], g, 3, 0); !p.has(identOf(game.White, 2)) || !p.has(identOf(game.Green, 2)) {
		t.Errorf("Donald 的 2 應可能是 w2 或 g2，筆記卻是 %v", p.list())
	}
}

func TestGiverTouchingDuplicateRulesOutOwnInterpretation(t *testing.T) {
	// Cathy 給 Alice 綠色點到 g3（和已救下的 g2）：Alice 看到 Bob 的 finesse position 有 g1，
	// 她的牌是 g1（直接）或 g3（Reverse Finesse Bob）還不確定。Alice 接著給 Bob 綠色點到 g1 與 chop 上的 g4：
	// 她不會點到可能和自己已提示的牌重複的牌，所以在她心中那張就是 g3，g4 是靠 Bob 的 g1、她的 g2、g3 的 delayed play clue。
	// Cathy 不能以為 g3 要由自己的 finesse position 盲打
	g, bots := scenarioAt(t, 2, "", "r4 b3 g3 y5 g2", "g1 w3 r2 w3 g4", "w1 y4 w4 b4 r3")
	g.Current = 1
	force(t, g, game.HintRankTo(0, 2))
	force(t, g, game.HintColorTo(0, game.Green))
	force(t, g, game.HintColorTo(1, game.Green))
	step(t, g, bots)
	step(t, g, bots)
	if n := bots[2].s.notes[g.Hands[2][0].Card.ID]; n.finessed {
		t.Errorf("Cathy 不該以為自己的 finesse position 被 finesse，筆記卻是 %v", n.poss.list())
	}
}

func TestOnlyMyFinesseBeforeMyPlay(t *testing.T) {
	// Bob 手上有確定能打的 b1。提示 Cathy 綠色點到 g2、Reverse Finesse Alice 的 g1，只有 Bob 給得出
	// （Alice、Cathy 都看不到自己的牌）：先給提示，b1 晚點打（原文 Level 20 The Occupied Play Clue 開頭的一般原則）
	// （用藍色讓 Bob 知道 b1：若是「1」，Alice 的 g1 可能和 Bob 那張 1 重複，就違反 Good Touch 了）
	g, bots := scenarioAt(t, 2, "", "g1 r2 y2 b3 w2", "r3 y3 b1 w4 y4", "r4 g2 y4 b4 w3")
	force(t, g, game.HintColorTo(1, game.Blue))
	if a := step(t, g, bots); a != game.HintColorTo(2, game.Green) {
		t.Fatalf("Bob 應先給 Cathy 綠色（只有他能給的 Reverse Finesse），卻做了 %+v", a)
	}
}
