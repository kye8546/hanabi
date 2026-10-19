package hgroup

import (
	"slices"
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

func TestL5GivesLayeredFinesse(t *testing.T) {
	// Bob 的 finesse position 是 b1、後面是 r1，Cathy 有 r2：Alice 提示 Cathy 紅色，Bob 會盲打出 b1、再盲打 r1（Layered Finesse），
	// 一個提示打出三張；比直接提示 Bob 的「1」好
	g, bots := scenarioAt(t, 5, "", "g4 y4 b4 w4 g3", "b1 r1 g3 y3 w4", "r2 g4 y4 b3 w3")
	if a := step(t, g, bots); a != game.HintColorTo(2, game.Red) && a != game.HintRankTo(2, 2) {
		t.Fatalf("Alice 應給 Cathy 的 r2 play clue（Layered Finesse Bob），卻做了 %+v（%s）", a, bots[0].Why())
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Blue] != 1 {
		t.Fatalf("Bob 應盲打 finesse position（打出 b1），卻做了 %+v", a)
	}
	step(t, g, bots) // Cathy 等
	step(t, g, bots) // Alice
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 應繼續盲打 r1，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL5NoLayeredSelfFinesse(t *testing.T) {
	// Bob 的 finesse position 是 b1、後面是 r1，slot 3 是 r2：「2」點到 r2 要 Bob 自己 Layered Finesse。
	// Bob 看不到自己的牌，盲打出 b1 就會以為他的 2 是 b2——不能這樣給（改用紅色直接點 r1、r2）
	g, bots := scenarioAt(t, 5, "", "g4 y4 b4 w4 g3", "b1 r1 r2 g4 y3", "y4 g4 w4 b3 w3")
	bots[0].Act(g.View(0))
	ev, ok := bots[0].s.evalClue(1, game.Hint{Kind: game.HintRank, Rank: 2}, 1)
	if !ok || ev.kind == kindPlay {
		t.Fatalf("「2」要 Bob 自己 Layered Finesse，不該當成 play clue 給：%+v", ev)
	}
}

// layeredOn：ev 是 play clue，計畫要 p Layered Finesse。
func layeredOn(ev clueEval, p int) bool {
	return ev.kind == kindPlay && slices.ContainsFunc(ev.plan, func(d dep) bool { return d.player == p && d.layers > 0 })
}

func TestL5NoLayerThroughGottenCard(t *testing.T) {
	// r3 已打出，Donald 已被提示、確定的 r4。Bob 的 finesse position 是 r4、後面是 w1：Layered Finesse 不能拿 r4 當 layer——
	// Donald 先打出他的 r4，Bob 輪到時盲打 r4 就打錯了（反過來，Donald 的 r4 會變成他不知道的重複牌）
	g, bots := scenarioAt(t, 5, "r3", "g4 y4 b4 g3", "r4 w1 g3 y3", "w2 g4 y4 b3", "r4 b2 g2 y2")
	g.Hints = 5
	g.Current = 2
	force(t, g, game.HintColorTo(3, game.Red)) // Cathy：Donald 的 r4
	force(t, g, game.Discard(3))               // Donald
	bots[0].Act(g.View(0))
	if ev, _ := bots[0].s.evalClue(2, game.Hint{Kind: game.HintColor, Color: game.White}, 2); layeredOn(ev, 1) {
		t.Fatalf("Bob 的 r4 是 Donald 已提示的牌的重複，不能當 layer：%+v", ev)
	}
}

func TestL5NoFinesseOnPlayerIAmWaitingFor(t *testing.T) {
	// Alice 提示 Bob 紅色點到 r2：Bob 的牌可能是 r1，也可能是 Reverse Finesse Cathy 的 r1，他在等 Cathy。
	// 這時 Bob 不能給要 Cathy 盲打的提示（例如提示 Donald 的 y2，Layered Finesse Cathy 的 r1 → y1）：
	// Cathy 的 finesse position 有沒有被佔走還不確定，兩人認定的位置可能不同（等待時只做無關的動作）
	g, bots := scenarioAt(t, 5, "", "g4 b4 w4 g3", "r2 g4 b4 w4", "r1 y1 g3 b3", "y2 g4 b4 w3")
	g.Hints = 5
	force(t, g, game.HintColorTo(1, game.Red))
	bots[1].Act(g.View(1))
	if ev, _ := bots[1].s.evalClue(3, game.Hint{Kind: game.HintColor, Color: game.Yellow}, 2); ev.kind == kindPlay {
		t.Fatalf("Bob 還在等 Cathy 盲打，不該給要她盲打的提示：%+v", ev)
	}
}

// ambiguousPass：5 人局 Lv5 自我對局重播到 codes 之後，下一位不能打錯（他誤以為沒盲打的人是在讓 finesse 給他）。
func ambiguousPass(t *testing.T, seed uint64, codes, why string) {
	t.Helper()
	g, bots := replayCodes(t, 5, seed, 5, codes)
	p := g.Current
	if a := step(t, g, bots); g.Strikes != 0 {
		t.Fatalf("P%d %+v 打錯（%s）：%s", p, a, bots[p].s.why, why)
	}
}

func TestL5PassIsQueuedBlindPlay(t *testing.T) {
	// t3 白色點到 P3 的 w3：可能是直接的 w2，也可能是 finesse P1 的 w2（P3 還沒記成被 finesse）；t8 綠色要 P1 盲打 g1。
	// t12 P1 先盲打 w2（Queued Finesse 照提示先後），不是沒接 g1 的 finesse——P3 不能以為 g1 在自己身上
	ambiguousPass(t, 514, "h3B p1 h3W h05 h0W h2B p2 h4G p4 h3R h21 p2 p4",
		"P1 盲打的是另一個提示的 finesse")
}

func TestL5PassIsOtherBlindPlay(t *testing.T) {
	// t4 白色點到 P4 的 w3，P2 的 finesse position 是 w1、後面是 w2（Double Finesse）。t11 藍色要 P2 盲打 b2，
	// t13 P2 盲打出 w2（P4 已排除了 Double Finesse 的解讀）：他是在接他認為的 finesse，不是沒接藍色的
	ambiguousPass(t, 507, "h3R p1 h14 h4W p1 h3W h4B p1 p3 p1 h3B p4 p2 h1R",
		"盲打沒被提示的牌就是在接某個 finesse")
}

func TestL5PassBecauseItLooksLikeASave(t *testing.T) {
	// t1「2」點到 P3 chop 上的 g2。P1 的 finesse position 是 g1、手上另有 g2——P2 看得到 P1 的 g2，知道不是 2 Save、
	// 要 P1 盲打 g1；但 P1 看不到自己的 g2，從他的角度是 2 Save，不盲打是對的，P2 不能因此以為 g1 在自己身上
	ambiguousPass(t, 1025, "h32 h01 d4 d3 h3B p4 h4G",
		"從沒盲打的人的角度可能是 save clue")
}

func TestL5PassWhenHisFinessePositionIsNotTheCard(t *testing.T) {
	ambiguousPass(t, 500, "h11 p1 h31 p2 h0Y p4 p4 h4B p2 h0R h4G p4 h04 p2",
		"他的 finesse position 不是那張時，他不打另有原因")
}

func TestL5PassAfterFinessedCardWasClued(t *testing.T) {
	// t10 紅色點到 P2 的 r4：finesse P3 的 r2、P0 的 r3。P3 盲打 r2 後，t15 P4 用紅色直接點到 P0 的 r3，
	// t16 P0 先打確定的 y1——r3 已經是被提示的牌，他照一般順序打就好，不是沒接 finesse，P1 不能以為 r3 在自己身上
	ambiguousPass(t, 1249, "h3G h4W h01 d2 h21 p1 h43 p4 p2 h2R p4 h3Y h35 p1 h0R p4",
		"被 finesse 的牌後來被直接提示了")
}

func TestL5PassWhileQueued(t *testing.T) {
	// t9 綠色點到 P2 的 g5：prompt P1 的 g2、finesse 他的 g3、g4；t11 黃色點到 P4 chop 上的 y2，要 P1 盲打 y1。
	// t12 P1 先打 g2（Queued Finesse：之前那個提示的牌先打完），不是沒接 y1 的 finesse，P3 不能以為 y1 在自己身上
	ambiguousPass(t, 1271, "h22 p1 h31 p3 h12 h4R h05 p1 h2G p4 h4Y p3 p3",
		"他還有排在前面的 finesse")
}

func TestL5NoClueOnFinessedCardUnknownToSomeone(t *testing.T) {
	// 自我對局 5 人局 seed 393：t30 P4 紅色給 P0 的 r5，finesse 了 P3 的 r3；P0 看不到焦點，不知道 r3 被 finesse。
	// t32 P1 若給 P3「3」（點到那張 r3 與新的 y3），知道 finesse 的人讀成 y3、要 Reverse Finesse P0 的 y2，
	// P0 卻把 r3 當成新牌、以為焦點是能打的 r3 而不盲打，後面的人又依 Ambiguous Finesse 以為是自己而打錯
	g, bots := replayCodes(t, 5, 393, 5, "h13 h01 h3B h2G h2B p4 p1 p3 h14 d4 p4 d1 p2 h22 h24 p4 p2 p1 h05 d4 p2 p4 p2 h4R p1 p2 d3 h4B p1 h0R d4")
	if a := bots[1].Act(g.View(1)); a == game.HintRankTo(3, 3) {
		t.Errorf("P1 不該給點到別人不知道被 finesse 的牌的「3」（%s）", bots[1].Why())
	}
}

func TestL5NoLayerThatLooksLikeFocus(t *testing.T) {
	// 自我對局 5 人局 seed 492：P4 的「2」焦點是 g2，要 P1 Layered Finesse（layer 是 y1）。y1 一打出，
	// P4 就會以為焦點是 y2（Bob is Lazy 的讀法），中間別人的提示讓他把 chop 上的 g3 讀成 g1 打錯——不給
	g, bots := replayCodes(t, 5, 492, 5, "")
	if a := bots[0].Act(g.View(0)); a == game.HintRankTo(4, 2) {
		t.Errorf("P0 不該給 layer 會被讀成焦點前一張的 Layered Finesse（%s）", bots[0].Why())
	}
}

func TestL5PassAfterStrikeIsNotAmbiguousFinesse(t *testing.T) {
	// 自我對局 5 人局 seed 948：t42 藍色給 P3 的 b3，要 Reverse Finesse P4 的 b2；t44 P3 把同一個提示點到的 b4 打錯，
	// 相關的筆記照 What to Do After a Strike 作廢，P4 沒盲打。P0 不能依 Ambiguous Finesse 以為是自己而盲打
	g, bots := replayCodes(t, 5, 948, 5, "h2Y h4G p1 h0Y p1 h3R p1 d3 h2G p2 h1W p2 p4 p1 p1 p4 h2Y h4W d4 p3 p3 p4 p1 h05 p4 p1 h24 p3 p1 h3G h3R d4 p4 p2 h0G h3Y d4 d4 p3 d4 p1 h3B h33 p4 d4")
	if a := bots[0].Act(g.View(0)); a == game.Play(0) {
		t.Errorf("P0 不該盲打（%s）", bots[0].Why())
	}
}

func TestL5PromptMovesToOtherCluedCardBeforeHiddenFinesse(t *testing.T) {
	// 自我對局 5 人局 seed 1643：P4 有兩張被提示的 1（左 y1、右 r1）。t16「2」prompt 左邊那張當 r1，筆記寫成 r1；
	// t19 黃色的 y4 要 prompt y1，他打出右邊那張，結果是 r1。y1 的 prompt 要移到另一張可能是 y1 的已提示牌（提示資訊上
	// 左邊那張仍可能是 y1），不是 Hidden Finesse——以前他因此盲打 finesse position 的 w3
	g, bots := replayCodes(t, 5, 1643, 5, "h41 h4W h3W h23 p3 p1 h25 d1 h21 p4 p2 p1 p1 p2 h2G h12 d4 p1 h1Y p4 h33 p3 p4 p4")
	if a := bots[4].Act(g.View(4)); a == game.Play(1) {
		t.Errorf("P4 不該盲打 finesse position（%s）", bots[4].Why())
	}
}

func TestL5CluedFinesseCardIsNotLayer(t *testing.T) {
	// 自我對局 5 人局 seed 2054：要 P0 盲打的 finesse position（當 y2）後來被紅色點到、他打出 r3。
	// 提示資訊已排除它是 y2，這不是 Layered Finesse 的 layer，他不該再盲打下一張當 y2（是 w4）
	g, bots := replayCodes(t, 5, 2054, 5, "h42 h0B p1 h1B h3R h2W h4G p2 h0Y p3 h1R d1 p1 p1 p2 p4 p4 h1W d4 p3 p4 h35 d4 h43 d3 p1 p3 h1Y d2 h0R p1 p4 h3W d2 h35")
	if a := bots[0].Act(g.View(0)); a == game.Play(1) {
		t.Errorf("P0 不該盲打 slot 2（%s）", bots[0].Why())
	}
}

func TestL5NoClueStealingPendingLayer(t *testing.T) {
	// 自我對局 5 人局 seed 6171：t1 白色要 P1 依序盲打 layer r1、g1 再打 w1。P3 不能給要 P4 盲打 g1 的提示（綠色給 P2 的 g2）：
	// P4 先打出 g1，P1 照計畫盲打的 g1 就是重複的牌
	g, bots := replayCodes(t, 5, 6171, 5, "h4W p1 h04")
	if a := bots[3].Act(g.View(3)); a == game.HintColorTo(2, game.Green) {
		t.Errorf("P3 不該給會讓別人打出 layer 牌面的提示（%s）", bots[3].Why())
	}
}

func TestL5LayerIsNotEvidenceWhenAnotherClueNeedsIt(t *testing.T) {
	// 自我對局 5 人局 seed 491：黃色要 P3 盲打 y1；之後的「2」也有「w2，P3 的 y1 是 layer」的解讀。P3 盲打出 y1 正好是
	// 黃色要的牌面，不能拿來當「2」是 w2 的證據（實際是 b2）
	g, bots := replayCodes(t, 5, 491, 5, "h4Y h3G h42 p1 p4 h31 h33 h1W p3 h0G h2W p2 d4 p3")
	if a := bots[4].Act(g.View(4)); a == game.Play(3) {
		t.Errorf("P4 不該把 slot 4 當成 w2 打出（%s）", bots[4].Why())
	}
}

func TestL5BlindPlayBelongsToEarlierClue(t *testing.T) {
	// 自我對局 5 人局 seed 1226：紅色可能是 r2、要 P1 finesse 的 r1，他的 finesse position 是 layer y1。之後的「2」救 P0 chop 上的 w2，
	// P1 盲打出 y1 時，P0 不能把它當成「2」是 y2 的證明（以前就這樣丟掉 save 的解讀、把 w2 當 y2 打出）
	g, bots := replayCodes(t, 5, 1226, 5, "h2B h4G p4 h0R h02 h3R p1 h4Y h15 p2")
	if a := bots[0].Act(g.View(0)); a == game.Play(3) {
		t.Errorf("P0 不該把 chop 上的 w2 當成 y2 打出（%s）", bots[0].Why())
	}
}
