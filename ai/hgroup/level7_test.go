package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 7（Emergency Discards）的範例情境，取自 H-Group 原文 level-7.mdx 與 Level 7 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。原文是 4 張手牌，這裡 3 人局是 5 張。

// known 讓每個 bot 把座位 p 第 slot 張牌記成之前就被提示過、提示本身就確定是 c（大家都知道的牌）。
func known(t *testing.T, g *game.Game, bots []*Bot, p, slot int, c string) {
	t.Helper()
	preClued(t, g, bots, p, slot, idents(t, c))
}

func TestL7AliceIsForgetful(t *testing.T) {
	// 挑戰題 Alice is Forgetful：所有 2 都已打出、提示 0 個，Alice 有大家都知道的 r3 卻棄掉 chop：Scream Discard。
	// Bob 把 chop（slot 5）記成 Chop Moved，這一手不能棄牌、必須給提示
	for _, lv := range []int{6, 7} {
		g, bots := scenarioAt(t, lv, "r2 y2 g2 b2 w2", "r3 g4 b4 w4 y4", "y4 g4 b4 w4 r5", "y1 b1 w1 r1 g1")
		g.Hints = 0
		known(t, g, bots, 0, 0, "r3")
		force(t, g, game.Discard(4))
		chop := g.Hands[1][4].Card.ID
		a := step(t, g, bots)
		if got := a.Type == game.ActHint; got != (lv >= 7) {
			t.Errorf("Lv%d：Bob 給提示 = %v（%+v）", lv, got, a)
		}
		bots[1].s.sync(g.View(1))
		bots[2].Act(g.View(2))
		for _, p := range []int{1, 2} {
			if got := bots[p].s.notes[chop].cm; got != (lv >= 7) {
				t.Errorf("Lv%d：座位 %d 把 Bob 的 chop 記成 Chop Moved = %v", lv, p, got)
			}
		}
	}
}

func TestL7AliceLovesGarbage(t *testing.T) {
	// 挑戰題 Alice Loves Garbage：提示 0 個，Alice 有大家都知道沒用的 y2，卻棄 chop：Scream Discard（With Known-Trash）
	g, bots := scenarioAt(t, 7, "r2 y2 g2 b2 w2", "y2 g4 b4 w4 y4", "y4 g4 b4 w4 r5", "y1 b1 w1 r1 g1")
	g.Hints = 0
	known(t, g, bots, 0, 0, "y2")
	force(t, g, game.Discard(4))
	chop := g.Hands[1][4].Card.ID
	if a := step(t, g, bots); a.Type != game.ActHint {
		t.Errorf("Bob 應給提示，卻做了 %+v", a)
	}
	bots[1].s.sync(g.View(1))
	if !bots[1].s.notes[chop].cm {
		t.Error("Bob 的 chop 應被 Chop Move")
	}
}

func TestL7AliceScreams(t *testing.T) {
	// Alice 這邊：提示 0 個，Bob 沒牌可打、chop 是 r5（critical）：Alice 不打 r3，改棄 chop（Scream Discard）
	for _, lv := range []int{6, 7} {
		g, bots := scenarioAt(t, lv, "r2 y2 g2 b2 w2", "r3 g4 b4 w4 y4", "y4 g4 b4 w4 r5", "y1 b1 w1 r1 g1")
		g.Hints = 0
		known(t, g, bots, 0, 0, "r3")
		a := step(t, g, bots)
		want := game.Play(0)
		if lv >= 7 {
			want = game.Discard(4)
		}
		if a != want {
			t.Errorf("Lv%d：Alice 應 %+v，卻做了 %+v", lv, want, a)
		}
	}
}

func TestL7AnotherScreamDiscard(t *testing.T) {
	// 挑戰題 Another Scream Discard：Bob 的 chop 是 g4（差一張就能打），不能用 Scream Discard 救，Alice 照常打出 r3
	g, bots := scenarioAt(t, 7, "r2 y2 g2 b2 w2", "r3 g4 b4 w4 y4", "y4 b4 w4 y3 g4", "y1 b1 w1 r1 g1")
	g.Hints = 0
	known(t, g, bots, 0, 0, "r3")
	if a := step(t, g, bots); a != game.Play(0) {
		t.Errorf("Alice 應打出 r3，卻做了 %+v", a)
	}
}

func TestL7ScreamingIntoTheVoid(t *testing.T) {
	// 挑戰題 Screaming into the Void：Bob 有大家都知道的 y3 可打，chop 上的 w5 這一手不會被棄，Alice 照常打出 g3
	g, bots := scenarioAt(t, 7, "r2 y2 g2 b2 w2", "g3 g4 b4 w4 y4", "y3 r2 b4 w4 w5", "y1 b1 w1 r1 g1")
	g.Hints = 0
	known(t, g, bots, 0, 0, "g3")
	known(t, g, bots, 1, 0, "y3")
	if a := step(t, g, bots); a != game.Play(0) {
		t.Errorf("Alice 應打出 g3，卻做了 %+v", a)
	}
}

func TestL7TheSilentScream(t *testing.T) {
	// 挑戰題 The Silent Scream：提示 2 個，Alice 有大家都知道的 g3 與沒用的 y1，她棄掉 y1：Shout Discard。
	// Bob 把 chop（b4）記成 Chop Moved，這一手必須給提示
	g, bots := scenarioAt(t, 7, "y1 g2 b2", "y1 g3 r4 w4 y4", "y4 g4 r3 w4 b4", "y1 b1 w1 r1 g1")
	g.Hints = 2
	known(t, g, bots, 0, 0, "y1")
	known(t, g, bots, 0, 1, "g3")
	force(t, g, game.Discard(0))
	chop := g.Hands[1][4].Card.ID
	if a := step(t, g, bots); a.Type != game.ActHint {
		t.Errorf("Bob 應給提示，卻做了 %+v", a)
	}
	bots[1].s.sync(g.View(1))
	if !bots[1].s.notes[chop].cm {
		t.Error("Bob 的 chop 應被 Chop Move")
	}
}

func TestL7TooFewClues(t *testing.T) {
	// 挑戰題 Too Few Clues：提示 0 個，Alice 有大家都知道的 b3，Cathy 的 chop 是 w5。Alice 打出 b3 的話 Bob 沒提示可給，
	// Cathy 可能棄掉 w5：Alice 棄牌（Generation Discard），Bob 用這個提示救 Cathy 的 w5，不 Chop Move 自己的 chop
	g, bots := scenarioAt(t, 7, "r2 y2 g2 b2 w2", "b3 g4 b4 w4 y4", "y4 g4 r4 w4 g1", "y1 b1 w1 r1 w5")
	g.Hints = 0
	known(t, g, bots, 0, 0, "b3")
	if a := step(t, g, bots); a.Type != game.ActDiscard {
		t.Fatalf("Alice 應棄牌（Generation Discard），卻做了 %+v", a)
	}
	chop := g.Hands[1][4].Card.ID
	a := step(t, g, bots)
	if a.Type != game.ActHint || a.Target != 2 {
		t.Fatalf("Bob 應提示 Cathy，卻做了 %+v", a)
	}
	if !g.Hands[2][4].Know.Clued {
		t.Error("Bob 的提示應點到 Cathy chop 上的 w5")
	}
	bots[1].s.sync(g.View(1))
	bots[2].Act(g.View(2))
	for _, p := range []int{0, 1, 2} {
		if bots[p].s.notes[chop].cm {
			t.Errorf("座位 %d 不應把 Bob 的 chop 記成 Chop Moved（Generation Discard）", p)
		}
	}
}

func TestL7BobIsForgetful(t *testing.T) {
	// 挑戰題 Bob is Forgetful：Alice 用最後一個提示給 Cathy「4」，看起來是 finesse Bob 的 r3。Bob 沒盲打、改棄 chop：
	// 他沒打的是 finesse 的盲打牌，Cathy 的 finesse position 也可能是 r3（Ambiguous Finesse），不是 Scream Discard。
	// Cathy 不 Chop Move，盲打自己的 finesse position
	g, bots := scenarioAt(t, 7, "r2 y2 g2 b2 w2", "y1 g4 b4 w4 y4", "r3 g4 b4 w4 g1", "r3 b1 w1 r4 g1")
	g.Hints = 1
	force(t, g, game.HintRankTo(2, 4))
	force(t, g, game.Discard(4))
	chop := g.Hands[2][4].Card.ID
	if a := step(t, g, bots); a != game.Play(0) {
		t.Errorf("Cathy 應盲打 finesse position 的 r3，卻做了 %+v", a)
	}
	if bots[2].s.notes[chop].cm {
		t.Error("不是 Scream Discard，Cathy 的 chop 不應被 Chop Move")
	}
}

func TestL7ShouldThisBlindPlay(t *testing.T) {
	// 挑戰題 Should this Blind-Play?：r3、y5、g3、b3、w5 已打出，Alice 有一張很久以前被 Chop Move、帶有「不是 5」資訊的牌：
	// 它有用的話只可能是 r4 / g4 / b4（都能打），也不可能是 critical。終局、提示不多、0 個閃電時賭它；2 個閃電時絕不
	for _, strikes := range []int{0, 2} {
		g, bots := scenarioAt(t, 7, "r3 y5 g3 b3 w5", "r1 y1 y2 g1 b1", "y1 y3 g1 b2 w1", "y4 w2 w3 b1 r2")
		g.Hints, g.Strikes = 1, strikes
		g.Deck = g.Deck[:5]
		id := g.Hands[0][2].Card.ID
		g.Hands[0][2].Know.Ranks &^= 1 << (game.MaxRank - 1)
		for i, b := range bots {
			b.s = newState(g.View(i), b.level)
			b.s.sync(g.View(i))
			n := b.s.notes[id]
			n.cm = true
			n.explicit &^= rankSet(5)
			n.poss &^= rankSet(5)
		}
		a := bots[0].Act(g.View(0))
		if got := a == game.Play(2); got != (strikes == 0) {
			t.Errorf("%d 個閃電：Alice 盲打 Chop Moved 的牌 = %v（%+v）", strikes, got, a)
		}
	}
}
