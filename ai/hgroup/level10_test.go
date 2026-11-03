package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 10（Special Discards）的範例情境，取自 H-Group 原文 level-10.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。原文是 4 張手牌，這裡 3 人局是 5 張。

func TestL10GentlemansDiscard(t *testing.T) {
	// 原文 The Gentleman's Discard：第一回合，Alice「1」點到 Cathy 的 r1，Bob 紅色補上它、又點到新的 r2。
	// Cathy 棄掉確定的 r1：別人手上沒有已被拿到的 r1（不是 Sarcastic Discard），Alice 看不到別的 r1，
	// 就是她 finesse position 的那張，盲打出 r1。Lv9 沒有 Gentleman's Discard，Alice 不盲打
	for _, lv := range []int{9, 10} {
		g, bots := scenario6(t, lv, "", "r1 g4 b4 w4 y3", "y4 g3 b3 w3 g4", "r2 g3 r1 b4 w4")
		g.Hints = 5
		force(t, g, game.HintRankTo(2, 1))
		force(t, g, game.HintColorTo(2, game.Red))
		force(t, g, game.Discard(2))
		a := step(t, g, bots)
		if got := a == game.Play(0); got != (lv >= 10) {
			t.Fatalf("Lv%d：Alice 盲打 finesse position = %v（%+v，%s）", lv, got, a, bots[0].Why())
		}
		if lv >= 10 && g.Fireworks[game.Red] != 1 {
			t.Fatalf("Alice 應盲打出 r1")
		}
	}
}

func TestL10LayeredGentlemansDiscard(t *testing.T) {
	// 原文 The Layered Gentleman's Discard：同上，但 Alice 的 finesse position 是 g1、slot 2 才是 r1。
	// 她盲打出 g1 後知道 r1 還沒來，下一手繼續盲打 slot 2。Bob 看得到，也把 Alice 的牌記成要盲打的 r1
	g, bots := scenario6(t, 10, "", "g1 r1 b4 w4 y3", "y4 g3 b3 w3 g4", "r2 g3 r1 b4 w4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 1))
	force(t, g, game.HintColorTo(2, game.Red))
	force(t, g, game.Discard(2))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Green] != 1 {
		t.Fatalf("Alice 應從 finesse position 盲打（打出 g1），卻做了 %+v", a)
	}
	bots[1].Act(g.View(1))
	if got := noteOfCard(t, bots[1], g, 0, "r1"); got != idents(t, "r1") {
		t.Errorf("Bob 應把 Alice 的 r1 記成要盲打的 r1，筆記卻是 %v", got.list())
	}
	force(t, g, game.Discard(4)) // Bob
	force(t, g, game.Discard(4)) // Cathy
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Layered Gentleman's Discard：Alice 應繼續盲打 slot 2 的 r1，卻做了 %+v（%s）", a, bots[0].Why())
	}
}

func TestL10BatonDiscard(t *testing.T) {
	// 原文 The Baton Discard：r1 已打出。Alice 棄掉確定的 r3；Bob 沒有已提示的牌，在 Cathy 手上看不到 r3，
	// 就是他 finesse position 的那張：記成 r3（視同被提示），沒別的事可做就棄 chop。Cathy 看得到，也這樣記
	for _, lv := range []int{9, 10} {
		g, bots := scenario6(t, lv, "r1", "r3 g4 b4 w4 y3", "r3 y4 g3 b3 w3", "y4 g4 b4 w4 g2")
		g.Hints = 5
		known(t, g, bots, 0, 0, "r3")
		force(t, g, game.Discard(0))
		r3 := g.Hands[1][0].Card.ID
		a := step(t, g, bots)
		bots[2].Act(g.View(2))
		for _, p := range []int{1, 2} {
			n := bots[p].s.notes[r3]
			if got := n.finessed && n.poss == idents(t, "r3"); got != (lv >= 10) {
				t.Errorf("Lv%d：座位 %d 把 Bob 的 finesse position 記成被傳到的 r3 = %v（%v）", lv, p, got, n.poss.list())
			}
		}
		if a.Type == game.ActPlay {
			t.Errorf("Lv%d：r3 還不能打，Bob 不該打牌（%+v）", lv, a)
		}
	}
}

func TestL10SarcasticFinesse(t *testing.T) {
	// 原文 The Sarcastic Finesse：什麼都沒打出，Alice 有一張不知道顏色的 2（實際是 r2）。Alice 紅色點 Cathy 的 r2，
	// finesse Bob 的 r1；Bob 盲打出 r1。Cathy 的 r2 確定了，看到 Alice 那張就是 r2：Sarcastic Discard 傳給 Alice
	g, bots := scenario6(t, 10, "", "y4 r2 b4 w4 y3", "r1 y4 g3 b3 w3", "r2 g4 b4 w4 g3")
	g.Hints = 5
	preClued(t, g, bots, 0, 1, idents(t, "r2 y2 g2 b2 w2"))
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 應盲打出 r1，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Discard(0) {
		t.Fatalf("Cathy 應 Sarcastic Discard 她的 r2，卻做了 %+v（%s）", a, bots[2].Why())
	}
	bots[0].Act(g.View(0))
	if got := noteOfCard(t, bots[0], g, 0, "r2"); got != idents(t, "r2") {
		t.Errorf("Alice 應知道自己的 2 是 r2，筆記卻是 %v", got.list())
	}
}

func TestL10GiveSarcasticFinesse(t *testing.T) {
	// 給 Sarcastic Finesse：同上的手牌，Alice 不知道自己的 2 是什麼。Lv9 認為紅色點 Cathy 的 r2 可能和自己的 2 重複
	// （違反 Good Touch）而不給；Lv10 照給——最差也是 Bob 打出 r1、Cathy 告訴 Alice 那張是 r2
	for _, lv := range []int{9, 10} {
		g, bots := scenario6(t, lv, "", "y4 r2 b4 w4 y3", "r1 y4 g3 b3 w3", "r2 g4 b4 w4 g3")
		g.Hints = 5
		preClued(t, g, bots, 0, 1, idents(t, "r2 y2 g2 b2 w2"))
		a := step(t, g, bots)
		if got := a == game.HintColorTo(2, game.Red); got != (lv >= 10) {
			t.Errorf("Lv%d：Alice 給紅色 = %v（%+v，%s）", lv, got, a, bots[0].Why())
		}
	}
}

// certainFinesse 是原文 The Certain Finesse 的情境：r2 已打出，Alice 有一張紅色（負向的 4，依 Good Touch 是 r3 或 r5），
// 她紅色點 Bob 的 r4，要 Reverse Finesse Cathy 的 r3；Bob 先等。alice 是她那張紅牌的實際牌面。
func certainFinesse(t *testing.T, alice string) (*game.Game, []*Bot) {
	g, bots := scenario6(t, 10, "r2", "y4 "+alice+" b4 w4 y3", "r4 y4 g3 b3 w3", "r3 g4 b4 w4 g3")
	g.Hints = 5
	preClued(t, g, bots, 0, 1, idents(t, "r3 r5"))
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 要等 Cathy 的 Reverse Finesse，卻做了 %+v", a)
	}
	return g, bots
}

func TestL10CertainDiscard(t *testing.T) {
	// Alice 的紅牌真的是 r3：Cathy 棄掉 finesse position 的 r3（Certain Discard），傳回給 Alice；
	// Alice 知道自己那張是 r3、打出，Bob 接著打 r4
	g, bots := certainFinesse(t, "r3")
	if a := step(t, g, bots); a != game.Discard(0) {
		t.Fatalf("Cathy 應 Certain Discard finesse position 的 r3，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Alice 應打出被傳回的 r3，卻做了 %+v（%s）", a, bots[0].Why())
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 4 {
		t.Fatalf("Bob 應打出 r4，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL10CertainFinesseNoDuplicate(t *testing.T) {
	// Alice 的紅牌是 r5：Cathy 照常盲打 r3
	g, bots := certainFinesse(t, "r5")
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 應盲打出 r3，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL10CompositionFinesse(t *testing.T) {
	// 原文 The Composition Finesse：所有 2 都已打出，Alice 的紅牌依 Good Touch 是 r3、r4 或 r5（實際是 r4）。
	// Alice 紅色點 Bob 的 r4；Bob 先等。Cathy 的 finesse position 是 r3，Alice 那張不是 r3，盲打出 r3。
	// Bob 的紅牌確定是 r4，看到 Alice 那張就是 r4：Sarcastic Discard 傳給 Alice
	g, bots := scenario6(t, 10, "r2 y2 g2 b2 w2", "y4 r4 b4 w4 y3", "r4 y4 g3 b3 w3", "r3 g4 b4 w4 g3")
	g.Hints = 5
	preClued(t, g, bots, 0, 1, idents(t, "r3 r4 r5"))
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 要等 Cathy 的 Reverse Finesse，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 應盲打出 r3，卻做了 %+v（%s）", a, bots[2].Why())
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Discard(0) {
		t.Fatalf("Bob 應 Sarcastic Discard 他的 r4，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL10GiveCertainFinesse(t *testing.T) {
	// 給 Certain Finesse：同 certainFinesse 的手牌，Alice 自己不知道紅牌是 r3 還是 r5。Lv9 認為 finesse Cathy 的 r3
	// 可能和自己的紅牌重複（違反 Good Touch）而不提示 Bob 的 r4；Lv10 照給——Cathy 看到重複會 Certain Discard 傳回來
	for _, lv := range []int{9, 10} {
		g, bots := scenario6(t, lv, "r2", "y4 r3 b4 w4 y3", "r4 y4 g3 b3 w3", "r3 g4 b4 w4 g3")
		g.Hints = 5
		preClued(t, g, bots, 0, 1, idents(t, "r3 r5"))
		a := step(t, g, bots)
		if got := a.Type == game.ActHint && a.Target == 1; got != (lv >= 10) {
			t.Errorf("Lv%d：Alice 提示 Bob 的 r4 = %v（%+v，%s）", lv, got, a, bots[0].Why())
		}
	}
}
