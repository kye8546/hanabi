package hgroup

import (
	"fmt"
	"slices"
	"strings"
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

func TestSelfFinesseNoteAfterFinesseTargetPasses(t *testing.T) {
	// Level 5 挑戰題 Bob is Lazy 的局面（Level 2 就適用）：Alice 提示 Cathy「2」點到 slot 1 與 slot 4，
	// Cathy 看到 Bob 的 finesse position 是 r1，先記成 r2。Bob 沒盲打（給了別的提示）→ 改成 Self-Finesse，
	// 她的 2 要改記成 y2/g2/b2/w2；盲打出 y1 後就是 y2（以前筆記一直停在 r2）
	g, bots := scenarioAt(t, 2, "", "g1 y4 b4 w4 g3", "r1 y4 g4 b4 w4", "y2 y1 g4 b2 w4")
	force(t, g, game.HintRankTo(2, 2))
	force(t, g, game.HintRankTo(0, 1)) // Bob：Alice 的 g1
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Yellow] != 1 {
		t.Fatalf("Cathy 應盲打 finesse position（slot 2 的 y1），卻做了 %+v（%s）", a, bots[2].Why())
	}
	bots[2].s.sync(g.View(2))
	if got := noteOfCard(t, bots[2], g, 2, "y2"); got != idents(t, "y2") {
		t.Fatalf("Cathy 盲打出 y1 後，她的 2 應是 y2，筆記卻是 %v", got.list())
	}
}

func TestWaitingReceiverDoesNotClueFinessedCardsCopy(t *testing.T) {
	// r2、b1 已打出。Alice 提示 Bob「3」點到 b3：Bob 的牌可能是 r3（直接），也可能是 b3（Reverse Finesse Cathy 的 b2），
	// 他要等 Cathy。等的時候只能做無關的動作：提示 Alice 的 b2，Cathy 一盲打 b2 它就是重複的牌（違反 Good Touch）
	g, bots := scenarioAt(t, 2, "r2 b1", "b2 g4 y4 w4 r4", "b3 r4 g4 y4 w4", "b2 r4 g4 y4 w4")
	force(t, g, game.HintRankTo(1, 3))
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("Bob 應先等 Cathy 盲打，卻做了 %+v", a)
	}
	if a.Type == game.ActHint && a.Target == 0 && a.Hint.Matches(g.Hands[0][0].Card) {
		t.Fatalf("Bob 不該提示 Alice 的 b2（Cathy 的 b2 可能正被 finesse），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestReceiverCountsCopiesToKnowFinesse(t *testing.T) {
	// 4 人。Cathy 提示 Alice 黃色點到 y2：Alice 的牌可能是 y1，也可能是 Reverse Finesse Bob 的 y1 → y2。
	// Donald 接著抽到第三張 y1：三張 y1 都看得到，Alice 的黃牌只能是 y2——Bob 的 y1 確定被 finesse
	// （看得到 Alice 那張的人本來就知道），Alice 要記下，也就不能提示 Donald 的 y1（重複）
	g, bots := scenarioAt(t, 2, "", "y2 r4 g4 b4", "y1 r4 g3 b3", "g4 b4 w4 r3", "y1 w3 g3 w4")
	g.Hints = 5
	g.Current = 2
	force(t, g, game.HintColorTo(0, game.Yellow)) // Cathy
	topDeck(t, g, "y1")
	force(t, g, game.Discard(3)) // Donald 抽到 y1
	a := step(t, g, bots)
	if n := bots[0].s.notes[g.Hands[1][0].Card.ID]; !n.finessed || n.poss != idents(t, "y1") {
		t.Fatalf("Alice 推得出自己是 y2，Bob 的 y1 應記成被 finesse，筆記卻是 %v（finessed=%v）", n.poss.list(), n.finessed)
	}
	if a.Type == game.ActHint && a.Target == 3 && (a.Hint.Matches(g.Hands[3][0].Card) || a.Hint.Matches(g.Hands[3][1].Card)) {
		t.Fatalf("Alice 不該提示 Donald 的 y1，卻做了 %+v（%s）", a, bots[0].Why())
	}
}

func TestReplanWhenSomeoneElsePlaysMyPromptedCard(t *testing.T) {
	// 使用者回報 20261015-160004 t31（照紀錄重現）：Cathy（座位 2）被救的黃牌是 y3（她不知道，記成 y2 或 y3）。
	// t29 Alice 提示 Donald「4」點到 y4：Cathy 看不到自己的牌，先以為自己的黃牌是 y2（prompt）、finesse position 是 y3。
	// t30 Bob 盲打出 y2——他看得到 Cathy 的黃牌是 y3，只有「finesse Bob 的 y2、prompt Cathy 的 y3」時才會盲打。
	// Cathy 要照現在的煙火重算：她的黃牌是 y3 的 prompt，finesse position 不再被 finesse（那張是 b3）
	g, err := game.NewVariant(4, game.NoVariant, 1792051204766149000)
	if err != nil {
		t.Fatal(err)
	}
	bots := make([]*Bot, 4)
	for i := range bots {
		bots[i] = New(fmt.Sprint(i), 5)
	}
	for _, code := range strings.Fields("h3G h21 p3 p4 h3W h2B p2 p2 h1W d2 p2 p2 d4 p4 h1R h24 d4 h2Y d1 h0Y p1 p1 h0B d2 p2 p1 p2 h15 h34 p1") {
		if p := g.Current; p != 0 {
			bots[p].Act(g.View(p)) // AI 每回合都行動過，狀態和實際對局一致
		}
		a, err := parseCode(code)
		if err != nil {
			t.Fatal(err)
		}
		force(t, g, a)
	}
	yellow := g.Hands[2][3].Card.ID
	a := step(t, g, bots)
	if a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("Cathy 不該盲打 finesse position（y2 已由 Bob 打出，她的黃牌是 y3 的 prompt），卻做了 %+v（%s）", a, bots[2].Why())
	}
	if n := bots[2].s.notes[yellow]; n.poss != idents(t, "y3") {
		t.Fatalf("Cathy 的黃牌應記成 y3，筆記卻是 %v", n.poss.list())
	}
}

func TestFinesseChainAgainstSeatOrder(t *testing.T) {
	// 使用者回報 20261015-161229 t45：w1 已打出。Alice 提示 Bob 白色點到 w5：Cathy 的 finesse position 是 w3、w4，
	// Donald 的是 w2。w2 要先打，但 Donald 在 Cathy 之後——Cathy 和 Reverse Finesse 的接收者一樣先等 Donald 盲打 w2，
	// 再依序盲打 w3、w4，Bob 最後打 w5（原文沒有「連接牌要照座位順序」的限制）
	g, bots := scenarioAt(t, 2, "w1", "r4 g4 b4 y4", "w5 r3 g3 b3", "w3 w4 r1 y3", "w2 g1 b2 y2")
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.White))
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("Bob 的白牌要等 w2、w3、w4，不該打，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay && (a.Slot == 0 || a.Slot == 1) {
		t.Fatalf("Cathy 的 w3 要等 Donald 的 w2，不該先盲打，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 2 {
		t.Fatalf("Donald 應盲打 finesse position 的 w2，卻做了 %+v（%s）", a, bots[3].Why())
	}
	force(t, g, game.Discard(3)) // Alice
	step(t, g, bots)             // Bob
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.White] != 3 {
		t.Fatalf("Cathy 現在應盲打 w3，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestNoChainWaitingOnReceiversOwnCard(t *testing.T) {
	// 重播紀錄 20261006-143640（Lv2）到 t20：Alice 的「4」點到 Cathy 的 b4。Bob 不能讀成「Cathy 先 self-prompt 出 b2、
	// 我再盲打 b3」——排在接收者前面的人不會等接收者自己先打連接牌。以前放寬座位順序時 Bob 這樣讀，b2 由 Cathy 的
	// 另一張打出後，他照計畫盲打 finesse position，實際是 b2 而打錯
	g, err := game.NewVariant(3, game.NoVariant, 1791268600458482000)
	if err != nil {
		t.Fatal(err)
	}
	bots := make([]*Bot, 3)
	for i := range bots {
		bots[i] = New(fmt.Sprint(i), 2)
	}
	for _, code := range strings.Fields("h11 p2 h01 h22 p3 h15 p3 h0B p1 h24 h2B h05 p1 d4 h0G d3 d3 p1 h1R") {
		if p := g.Current; p != 0 {
			bots[p].Act(g.View(p))
		}
		a, err := parseCode(code)
		if err != nil {
			t.Fatal(err)
		}
		force(t, g, a)
	}
	if a := step(t, g, bots); g.Strikes != 0 {
		t.Fatalf("Bob 不該照「等 Cathy 先打 b2」的計畫盲打，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestFinesseCardLaterCluedOtherwise(t *testing.T) {
	// 使用者紀錄 20261014-095355（Black、Lv4）：t39 黃色點到使用者的 y3，從接收者的角度也可能是 y4（finesse AI-1 的 y3）；
	// t40 使用者用藍色點到 AI-1 那張牌——它不可能是 y3，y4 的解讀就不成立。AI-1 在 t44 不能還以為使用者會把 y3 當 y4 打而給 Fix Clue
	g, bots := replayCodesVariant(t, 3, game.BlackSix, 1791942835598568000, 4,
		"h11 p3 h12 h21 p3 p4 d5 d4 d5 h2R d2 h03 h23 p1 p4 h2K p3 p1 p1 h2W d5 d5 h04 d4 p3 h25 d2 d5 h0G d2 p1 h2R p1 h2G p5 p2 h1G p4 h0Y h1B p5 p1 h1K")
	if a := step(t, g, bots); a.Type != game.ActPlay {
		t.Fatalf("AI-1 應該打出確定的 b4，卻 %+v（%s）", a, bots[1].s.why)
	}
}

func TestPromptMayWaitForReceiversSelfFinesse(t *testing.T) {
	// 使用者回報 20261019-143628（Rainbow、Lv5）：t10 使用者「4」點到 AI-2 的 w4——AI-1 的 w1（已提示）、AI-2 自己
	// finesse position 的 w2（Self-Finesse）、AI-1 的 w3（prompt）。排在接收者前面的 AI-1 要等 AI-2 先打出 w2 才打 w3，
	// 這只是 prompt（他打的是已提示的牌），不是要他盲打，可以等；以前一律不允許，AI-2 只剩「y4，自己的 y2、y3」可讀，
	// t12 在 w1 打出前就把 w2 當 y2 盲打而打錯。現在 w4 也是可能，w2 還不能打，AI-2 先等
	g, bots := replayCodesVariant(t, 3, game.RainbowSix, 1792391788272327000, 5, "h2B h0G p5 p5 h21 p1 h1B d5 h1W h24 p5 ")
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("AI-2 的 finesse position 可能是還不能打的 w2，不該盲打：%+v（%s）", a, bots[2].s.why)
	}
}
