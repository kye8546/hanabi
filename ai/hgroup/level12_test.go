package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 12（Context）的範例情境，取自 H-Group 原文 level-12.mdx。
// 座位 0 = Alice、1 = Bob、2 = Cathy、3 = Donald；手牌字串由左（slot 1）到右（chop）。

func TestL12FocusInversion(t *testing.T) {
	// 原文 Focus Inversion：Alice「1」點 Bob 的 r1，Bob 打出 r1、抽到 b1；Cathy 藍色點 Bob 的 slot 1（b1）與 chop（b3）。
	// chop 有「不是 1」的負向資訊，不可能是 b1，用顏色 Self-Finesse 自己沒有意義：焦點反轉到最左邊那張，Bob 打出 slot 1。
	// Lv11 照 chop 焦點讀，不會打 slot 1
	for _, lv := range []int{11, 12} {
		g, bots := scenario6(t, lv, "", "y4 g4 w4 y3 g3", "r1 y4 g4 w3 b3", "y3 g3 w4 r4 y4")
		g.Deck[len(g.Deck)-1] = game.Card{Color: game.Blue, Rank: 1, ID: g.Deck[len(g.Deck)-1].ID} // Bob 打出 r1 後抽到 b1
		force(t, g, game.HintRankTo(1, 1))
		force(t, g, game.Play(0))
		force(t, g, game.HintColorTo(1, game.Blue))
		force(t, g, game.Discard(4)) // Alice
		a := step(t, g, bots)
		if got := a == game.Play(0); got != (lv >= 12) {
			t.Fatalf("Lv%d：Bob 打出 slot 1 = %v（%+v，%s）", lv, got, a, bots[1].Why())
		}
		if lv < 12 {
			continue
		}
		if g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Bob 應打出 b1")
		}
		for _, p := range []int{0, 1, 2} {
			bots[p].Observe(g.View(p))
			if got := noteOfCard(t, bots[p], g, 1, "b3"); got.has(identOfCard(card(t, "b1"))) || got&idents(t, "b2 b3 b4 b5") == 0 {
				t.Errorf("座位 %d 應把 Bob 的 chop 記成 b2–b5 中有用的牌，筆記卻是 %v", p, got.list())
			}
		}
	}
}

func TestL12FocusInversionNotForRank(t *testing.T) {
	// 原文：用數字的 Self-Finesse 永遠優先於 Focus Inversion。r1 已打出；Alice「3」點 Bob 的 slot 1（y3）與 chop（r3），
	// 沒人看得到 r2：照 chop 焦點讀成 Self-Finesse，Bob 盲打 finesse position（r2），不把焦點移到 slot 1
	g, bots := scenario6(t, 12, "r1", "y4 g4 w4 y3 g3", "y3 r2 g4 w4 r3", "y4 g3 w4 b4 y4")
	force(t, g, game.HintRankTo(1, 3))
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 應 Self-Finesse 盲打出 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL12Stale1sClue(t *testing.T) {
	// 原文 The Stale 1's Clue：y1、g1、g2 已打出，還有 3 個提示。Alice 棄牌結束 Early Game（表示沒有該給的提示），
	// Bob 接著「1」點 Cathy 的兩張 1：兩張都是好的話 Alice 必須先給，所以照順序會先打的那張（最右邊）是壞的。
	// Cathy 跳過它、打出另一張，並把它當已知垃圾。Lv11 照順序先打最右邊那張（y1）而打錯
	for _, lv := range []int{11, 12} {
		g, bots := scenario6(t, lv, "y1 g2", "b4 w4 b3 r4", "y4 g4 w4 b4", "g4 r1 y1 w4", "y3 g3 b3 w3")
		g.Hints = 4
		force(t, g, game.Discard(3)) // Alice 棄牌，Early Game 結束
		force(t, g, game.HintRankTo(2, 1))
		a := step(t, g, bots)
		if got := a == game.Play(1); got != (lv >= 12) {
			t.Fatalf("Lv%d：Cathy 打出 slot 2 = %v（%+v，%s）", lv, got, a, bots[2].Why())
		}
		if lv < 12 {
			continue
		}
		if g.Fireworks[game.Red] != 1 {
			t.Fatalf("Cathy 應打出 r1")
		}
		for p := range 4 {
			bots[p].Observe(g.View(p))
			if !bots[p].s.notes[g.Hands[2][2].Card.ID].bad {
				t.Errorf("座位 %d 應把 Cathy 的 y1 記成已知垃圾", p)
			}
		}
	}
}

func TestL12NotStaleAfterOtherAction(t *testing.T) {
	// 「1」不是緊接在結束 Early Game 的棄牌之後（中間有別的行動），照一般的順序讀：先打最右邊那張
	g, bots := scenario6(t, 12, "y1 g2", "b4 w4 b3 r4", "y4 g4 w4 b4", "g4 r1 b1 w4", "y3 g3 b3 w3")
	g.Hints = 6
	force(t, g, game.Discard(3))                 // Alice：Early Game 結束
	force(t, g, game.HintColorTo(3, game.Green)) // Bob：Donald 的 g3
	force(t, g, game.Discard(3))                 // Cathy
	force(t, g, game.Play(1))                    // Donald 打出 g3
	force(t, g, game.Discard(3))                 // Alice
	force(t, g, game.HintRankTo(2, 1))           // Bob
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Cathy 應照一般順序先打最右邊的 1（slot 4），卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL12SelfishFinesse(t *testing.T) {
	// 原文 The Selfish Finesse：Alice 紅色點 Bob 的 r1（Cathy 沒有 r1，大家都知道是 r1）；Bob 紅色點 Cathy 的 r3。
	// Cathy 看到 Alice 的 finesse position 是 r2：Bob 留著 r1 不打，是要鎖住這個 finesse，Cathy 等著。
	// Alice 知道 Bob 確定自己有 r1，r2 就在自己的 finesse position，先等 Bob 打出 r1 再盲打
	g, bots := scenario6(t, 12, "", "r2 y4 g4 b4 w4", "r1 y3 g3 b3 w3", "r3 y4 g4 b4 w3")
	force(t, g, game.HintColorTo(1, game.Red))
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 r3 還不能打，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Alice 應先等 Bob 打出 r1，卻做了 %+v（%s）", a, bots[0].Why())
	}
	if got := noteOfCard(t, bots[0], g, 0, "r2"); got != idents(t, "r2") {
		t.Errorf("Alice 應把 finesse position 記成 r2，筆記卻是 %v", got.list())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 應打出 r1，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 r3 還不能打，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Alice 應盲打出 r2，卻做了 %+v（%s）", a, bots[0].Why())
	}
}

func TestL12SelfishClue(t *testing.T) {
	// 原文 The Selfish Clue：所有 1 都已打出，Alice 有大家都知道的 r2。Alice 紅色點 Bob 的 r3、r4、r5：焦點 r3 等 Alice 的 r2，
	// Bob 先不打；Alice 打出 r2 後 Bob 打出 r3
	g, bots := scenario6(t, 12, "r1 y1 g1 b1 w1", "y4 r2 g4 b4 w4", "b1 r3 r4 r5 b1", "y3 g3 b3 w3 y4")
	preClued(t, g, bots, 0, 1, idents(t, "r2"))
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot != 0 && a.Slot != 4 {
		t.Fatalf("Bob 的 r3 要等 Alice 的 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	step(t, g, bots) // Cathy
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Alice 應打出 r2，卻做了 %+v（%s）", a, bots[0].Why())
	}
	bots[1].Observe(g.View(1))
	if got := noteOfCard(t, bots[1], g, 1, "r3"); got != idents(t, "r3") {
		t.Errorf("Bob 應把焦點記成 r3，筆記卻是 %v", got.list())
	}
}

func TestL12DuplicationResponsibilityTie(t *testing.T) {
	// 原文 Duplication Responsibility Example 5（換成 Bob 先行動）：Alice 的 chop 是要救的 b2。Bob 有兩張不知道是哪張的 2
	// （r2 已打出，每張是 b2 的機率較高），Cathy 有兩張不知道是哪張的藍牌（每張是 b2 的機率較低）。張數一樣時，
	// 提示資訊比較不明確的 Cathy 負責：Bob 不救，交給 Cathy。Lv11 沒有這條平手規則，Bob 照樣救
	for _, lv := range []int{11, 12} {
		g, bots := scenario6(t, lv, "r2", "y4 g4 w4 y3 b2", "y2 g2 w3 y3 g3", "b3 b4 w4 g3 y4")
		g.Hints = 5
		preClued(t, g, bots, 1, 0, idents(t, "r2 y2 g2 b2 w2"))
		preClued(t, g, bots, 1, 1, idents(t, "r2 y2 g2 b2 w2"))
		preClued(t, g, bots, 2, 0, idents(t, "b1 b2 b3 b4 b5"))
		preClued(t, g, bots, 2, 1, idents(t, "b1 b2 b3 b4 b5"))
		force(t, g, game.Discard(3)) // Alice
		a := step(t, g, bots)
		saves := a.Type == game.ActHint && a.Target == 0 && a.Hint.Matches(game.Card{Color: game.Blue, Rank: 2})
		if saves != (lv < 12) {
			t.Fatalf("Lv%d：Bob 救 Alice 的 b2 = %v（%+v，%s）", lv, saves, a, bots[1].Why())
		}
		if lv < 12 {
			continue
		}
		a = step(t, g, bots)
		if a.Type != game.ActHint || a.Target != 0 || !a.Hint.Matches(game.Card{Color: game.Blue, Rank: 2}) {
			t.Fatalf("Cathy 應救 Alice 的 b2，卻做了 %+v（%s）", a, bots[2].Why())
		}
	}
}
