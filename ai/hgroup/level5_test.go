package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 5（Intermediate Finesses）的範例情境，取自 H-Group 原文 level-5.mdx 與 Level 5 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy、3 = Donald、4 = Emily；手牌字串由左（slot 1）到右（chop）。

// topDeck 讓下一張被抽到的牌是 c（牌堆從尾端抽）。
func topDeck(t *testing.T, g *game.Game, c string) {
	t.Helper()
	k := card(t, c)
	k.ID = g.Deck[len(g.Deck)-1].ID
	g.Deck[len(g.Deck)-1] = k
}

func TestL5LayeredFinesse(t *testing.T) {
	// 原文 The Layered Finesse：什麼都沒打出。Alice 提示 Bob 紅色點到 r2；Cathy 的 finesse position 是 b1、後面是 r1。
	// Bob 可能是 r1，也可能是 Layered Finesse Cathy 的 r1，先等。Cathy 盲打出 b1（不是 r1）→ 繼續盲打 slot 2 的 r1；
	// Bob 看到 Cathy 盲打，知道自己是 r2
	g, bots := scenarioAt(t, 5, "", "g4 y4 b4 w4 g3", "r2 y4 g4 b4 w4", "b1 r1 y3 g3 w4")
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 可能被 Layered Finesse，應先等，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Cathy 應從 finesse position 盲打（打出 b1），卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("r1 還沒打出，Bob 的 r2 要等，卻做了 %+v", a)
	}
	if got := noteOfCard(t, bots[1], g, 1, "r2"); got != idents(t, "r2") {
		t.Fatalf("Cathy 盲打出別張牌，Bob 應知道自己是 r2，筆記卻是 %v", got.list())
	}
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Layered Finesse：Cathy 應繼續盲打 slot 2 的 r1，卻做了 %+v（%s）", a, bots[2].Why())
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 現在應打出 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if lv4, bots4 := scenarioAt(t, 4, "", "g4 y4 b4 w4 g3", "r2 y4 g4 b4 w4", "b1 r1 y3 g3 w4"); true {
		lv4.Hints = 5
		force(t, lv4, game.HintColorTo(1, game.Red))
		force(t, lv4, game.Discard(4)) // Bob
		force(t, lv4, game.Play(0))    // Cathy 盲打出 b1
		force(t, lv4, game.Discard(4)) // Alice
		force(t, lv4, game.Discard(4)) // Bob
		if a := step(t, lv4, bots4); a == game.Play(1) {
			t.Fatalf("Level 4 沒有 Layered Finesse，Cathy 不該繼續盲打")
		}
	}
}

func TestL5ClandestineFinesse(t *testing.T) {
	// 原文 The Clandestine Finesse：r1、b1 已打出。Alice 提示 Cathy「3」點到 slot 2 的 r3。Bob 盲打 finesse position
	// 當成 r2，結果是 b2。Cathy 的 3 可能是 b3（一般的 finesse），也可能是 r3（Bob 的 slot 2 是 r2，Clandestine Finesse），
	// 要再給 Bob 一回合；Bob 知道是 layer，接著盲打 slot 2 的 r2，Cathy 才打 r3
	g, bots := scenarioAt(t, 5, "r1 b1", "r1 b1 r1 b1 r1", "b2 r2 g4 y4 w4", "y4 r3 w4 g4 b4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 2 {
		t.Fatalf("Bob 應盲打 finesse position（打出 b2），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 1 {
		t.Fatalf("Cathy 的 3 可能是 r3（Clandestine Finesse），應給 Bob 一回合，卻打了")
	}
	if got := noteOfCard(t, bots[2], g, 2, "r3"); got != idents(t, "r3 b3") {
		t.Fatalf("Cathy 的 3 應記成 r3 或 b3，筆記卻是 %v", got.list())
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 盲打出的是 layer，應繼續盲打 slot 2 的 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 現在應打出 r3，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL5ClandestineFinesseNotHappening(t *testing.T) {
	// 同上，但 Cathy 的 3 其實是 b3：Bob 打出 b2 後不再盲打，Cathy 才知道是 b3
	g, bots := scenarioAt(t, 5, "r1 b1", "r1 b1 r1 b1 r1", "b2 r2 g4 y4 w4", "y4 b3 w4 g4 y4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 2 {
		t.Fatalf("Bob 應盲打 finesse position（打出 b2），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 1 {
		t.Fatalf("Cathy 應給 Bob 一回合，卻打了她的 3")
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Bob 打出的 b2 就是要的牌，不該再盲打，卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Blue] != 3 {
		t.Fatalf("Bob 沒再盲打，Cathy 應知道是 b3 而打出，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL5HiddenFinesse(t *testing.T) {
	// 挑戰題 What's Hidden?：r1、b1 已打出。Bob 的 chop 上有被「2」救下的 r2（他不知道是哪張 2），slot 1 是沒被提示的 b2。
	// Alice 提示 Cathy「3」點到 b3：Bob 以為是 prompt，打出 r2——Cathy 記成「r3，或 b3（Hidden Finesse b2）」，先不打，
	// 也不提示 b2（會 stomp）。Bob 發現打出的不是 b2，知道 b2 在 finesse position，盲打它；Cathy 才打 b3
	g, bots := scenarioAt(t, 5, "r1 b1", "r1 b1 r1 b1 r1", "y4 g4 w4 y4 r2", "y4 g4 b3 w4 g4")
	g.Hints = 5
	g.Current = 2
	force(t, g, game.HintRankTo(1, 2)) // Cathy：2 Save Bob 的 r2
	force(t, g, game.Discard(4))       // Alice
	topDeck(t, g, "b2")
	force(t, g, game.Discard(3)) // Bob 棄 chop，抽到 b2
	force(t, g, game.Discard(0)) // Cathy
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a != game.Play(4) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Bob 應把被提示的 2 當成 prompt 打出（是 r2），卻做了 %+v（%s）", a, bots[1].Why())
	}
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("Cathy 的 3 可能是 b3（Hidden Finesse），不該打，卻做了 %+v", a)
	}
	if a.Type == game.ActHint && a.Target == 1 && (a.Hint.Kind == game.HintRank && a.Hint.Rank == 2 || a.Hint.Kind == game.HintColor && a.Hint.Color == game.Blue) {
		t.Fatalf("Cathy 不該提示 Bob 的 b2（會 stomp Hidden Finesse），卻做了 %+v", a)
	}
	if got := noteOfCard(t, bots[2], g, 2, "b3"); !got.has(identOf(game.Blue, 3)) || !got.has(identOf(game.Red, 3)) {
		t.Fatalf("Cathy 的 3 應記成 r3 或 b3，筆記卻是 %v", got.list())
	}
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Blue] != 2 {
		t.Fatalf("Hidden Finesse：Bob 應盲打提示當下 finesse position 的 b2（slot 2），卻做了 %+v（%s）", a, bots[1].Why())
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Blue] != 3 {
		t.Fatalf("Cathy 現在應打出 b3，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

// queuedFinesse 照挑戰題 Two Finesses in a Row 進行到 Bob 第二次行動前（g3 y3 w3 已打出，其他牌是垃圾，不必救）：
// Alice 提示 Donald 藍色點到 b3，Bob 盲打 slot 1，Cathy 再提示 Donald 紅色點到 r2。
func queuedFinesse(t *testing.T, level int, bob string) (*game.Game, []*Bot) {
	t.Helper()
	g, bots := scenarioAt(t, level, "g3 y3 w3", "g1 y1 g2 w1", bob, "g1 y1 w1 g2", "w1 b3 r2 y1")
	g.Hints = 5
	force(t, g, game.HintColorTo(3, game.Blue))
	topDeck(t, g, "r1")
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Bob 應盲打 b1，卻做了 %+v", a)
	}
	force(t, g, game.HintColorTo(3, game.Red)) // Cathy
	force(t, g, game.Discard(3))               // Donald
	force(t, g, game.Discard(3))               // Alice
	return g, bots
}

func TestL5QueuedFinesse(t *testing.T) {
	// 挑戰題 Two Finesses in a Row：Alice 提示 Donald 藍色點到 b3（Double Finesse Bob 的 b1、b2），Bob 盲打出 b1。
	// Cathy 再提示 Donald 紅色點到 r2：Bob 看不到 r1，是他的——Queued Finesse，r1 在他現在的 finesse position（slot 1）。
	// 照提示先後，Bob 先盲打 slot 2 的 b2，再盲打 r1
	g, bots := queuedFinesse(t, 5, "b1 b2 g1 y2")
	r1 := g.Hands[1][0].Card.ID
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Blue] != 2 {
		t.Fatalf("Queued Finesse：Bob 應先盲打較早的 finesse（slot 2 的 b2），卻做了 %+v（%s）", a, bots[1].Why())
	}
	if n := bots[1].s.notes[r1]; !n.finessed || n.poss != idents(t, "r1") {
		t.Fatalf("Bob 的 slot 1 應記成被 finesse 的 r1，筆記卻是 %v", n.poss.list())
	}
	force(t, g, game.Discard(3)) // Cathy
	force(t, g, game.Discard(1)) // Donald
	force(t, g, game.Discard(3)) // Alice
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Queued Finesse：Bob 接著應盲打 r1（現在的 slot 2），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL5QueuedFinesseMovesRight(t *testing.T) {
	// 原文 The Queued Finesse：被 finesse 過的人，finesse position 往右移一張。Donald 提示 Cathy 藍色點到 b2（finesse Bob 的 b1），
	// Bob 還沒行動，Alice 又提示 Cathy 紅色點到 r2：Bob 的 finesse position 現在是 slot 2，那裡的 r1 排在 b1 之後盲打
	for _, level := range []int{4, 5} {
		g, bots := scenarioAt(t, level, "g3 y3 w3", "g1 y1 g2 w1", "b1 r1 g1 y2", "b2 r2 w1 g1", "y1 w1 g2 g1")
		g.Hints = 5
		g.Current = 3
		force(t, g, game.HintColorTo(2, game.Blue)) // Donald
		force(t, g, game.HintColorTo(2, game.Red))  // Alice
		r1 := g.Hands[1][1].Card.ID
		if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
			t.Fatalf("Lv%d：Bob 應先盲打較早的 finesse（slot 1 的 b1），卻做了 %+v（%s）", level, a, bots[1].Why())
		}
		if n := bots[1].s.notes[r1]; n.finessed != (level >= 5) {
			t.Fatalf("Lv%d：Bob 的 r1 被 finesse = %v，應為 %v（Queued Finesse 是 Level 5）", level, n.finessed, level >= 5)
		}
		if level < 5 {
			continue
		}
		step(t, g, bots)             // Cathy
		force(t, g, game.Discard(3)) // Donald
		force(t, g, game.Discard(3)) // Alice
		if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
			t.Fatalf("Queued Finesse：Bob 接著應盲打 r1（slot 2），卻做了 %+v（%s）", a, bots[1].Why())
		}
	}
}

func TestL5QueuedFinesseTurnsOutLayered(t *testing.T) {
	// 同上，但 Bob 的 slot 2 其實是 r1、b2 在 slot 3（Alice 的藍色是 Layered Finesse）。Bob 先盲打 slot 2 打出 r1：
	// Cathy 的紅色已經接上，slot 1 不再被 finesse；藍色的 finesse 往右移到 slot 3 的 b2
	g, bots := queuedFinesse(t, 5, "b1 r1 b2 y2")
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 應先盲打 slot 2（打出 r1），卻做了 %+v（%s）", a, bots[1].Why())
	}
	force(t, g, game.Discard(3)) // Cathy
	force(t, g, game.Discard(1)) // Donald
	force(t, g, game.Discard(3)) // Alice
	if a := step(t, g, bots); a != game.Play(2) || g.Fireworks[game.Blue] != 2 || g.Strikes != 0 {
		t.Fatalf("Layered Finesse：Bob 應接著盲打 slot 3 的 b2（不是已不再被 finesse 的 slot 2），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL5AmbiguousFinesse(t *testing.T) {
	// 原文 The Ambiguous Finesse：4 人、什麼都沒打出。Alice 提示 Donald 紅色點到 r2。Bob 和 Cathy 的 finesse position
	// 都是 r1。Bob 以為是 Cathy 而不打；Cathy 看到 Bob 沒打，就知道自己也有 r1，立刻盲打
	g, bots := scenarioAt(t, 5, "", "g4 y4 b4 w4", "r1 y4 g4 b4", "r1 g4 y4 w4", "r2 y4 g4 b4")
	force(t, g, game.HintColorTo(3, game.Red))
	if a := bots[1].Act(g.View(1)); a.Type == game.ActPlay {
		t.Fatalf("Bob 看得到 Cathy 的 r1，不該盲打，卻做了 %+v", a)
	}
	force(t, g, game.Discard(3)) // Bob
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 沒盲打，Cathy 應知道自己有 r1 而盲打，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestL5BobIsLazy(t *testing.T) {
	// 挑戰題 Bob is Lazy：Alice 提示 Cathy「2」點到 slot 1 與 slot 4；Bob 的 finesse position 是 r1，
	// 他卻給了 1-for-1 的提示（不是緊急的事）。Urgency Principle：Bob 根本沒被 finesse，
	// 這是 Cathy 自己的 Self-Finesse，她盲打 finesse position（y1），她的 2 就是 y2
	g, bots := scenarioAt(t, 5, "", "g1 y4 b4 w4 g3", "r1 y4 g4 b4 w4", "y2 y1 g4 b2 w4")
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

func TestL5UrgencyPrinciple(t *testing.T) {
	// 原文 Urgency Principle：5 人，1 都打出了，只剩 1 個提示。Alice 提示 Emily 紅色點到 r3（finesse Cathy 的 r2）。
	// Bob 棄牌。Cathy 看到 Donald 的 chop 是 b5、他會棄牌：先 5 Save，延後盲打。Emily 知道 Cathy 是在做緊急的事，
	// 不把紅牌當成 r2 打出；Cathy 下一手再盲打 r2
	g, bots := scenarioAt(t, 5, allOnes, "g4 y4 w4 g4", "y4 g4 w4 g3", "r2 y4 g4 w4", "g4 y4 w4 b5", "r3 y4 g4 w4")
	g.Hints = 1
	force(t, g, game.HintColorTo(4, game.Red))
	force(t, g, game.Discard(3)) // Bob
	a := step(t, g, bots)
	if a != game.HintRankTo(3, 5) {
		t.Fatalf("Cathy 應先 5 Save Donald 的 b5（延後盲打），卻做了 %+v（%s）", a, bots[2].Why())
	}
	force(t, g, game.Discard(2)) // Donald
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("Cathy 是在做緊急的事，Emily 應再給她一回合，不該把紅牌當成 r2 打出（%s）", bots[4].Why())
	}
	force(t, g, game.Discard(3)) // Alice
	force(t, g, game.Discard(3)) // Bob
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 2 {
		t.Fatalf("Cathy 現在應盲打 r2，卻做了 %+v（%s）", a, bots[2].Why())
	}
}
