package hgroup

import (
	"strings"
	"testing"

	"hanabi/game"
)

// 依 H-Group Beginner's Guide / Level 1 的範例設計的情境測試。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1，最新）到右（chop 方向）。

func card(t *testing.T, s string) game.Card {
	t.Helper()
	c, ok := game.ParseColor(s[:1])
	if !ok || len(s) != 2 || s[1] < '1' || s[1] > '5' {
		t.Fatalf("看不懂牌 %q", s)
	}
	return game.Card{Color: c, Rank: game.Rank(s[1] - '0')}
}

// scenario 建立指定煙火（例如 "r2 b1"）與手牌的局面；牌堆換成不會干擾測試的牌。AI 使用 Level 1。
func scenario(t *testing.T, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	return scenarioAt(t, 1, stacks, hands...)
}

// scenarioAt 同 scenario，但 AI 使用指定的慣例等級。
func scenarioAt(t *testing.T, level int, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	return scenarioVariant(t, game.NoVariant, level, stacks, hands...)
}

// scenarioVariant 同 scenarioAt，但用指定的變體（黑色的牌寫成 k1–k5）。
func scenarioVariant(t *testing.T, va game.Variant, level int, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	g, err := game.NewVariant(len(hands), va, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Fields(stacks) {
		c := card(t, f)
		g.Fireworks[c.Color] = c.Rank
	}
	id := 100
	for p, h := range hands {
		g.Hands[p] = nil
		for _, f := range strings.Fields(h) {
			c := card(t, f)
			c.ID = id
			id++
			g.Hands[p] = append(g.Hands[p], game.HandCard{Card: c, Know: game.NewKnowledge(g.Variant)})
		}
	}
	g.Deck = nil
	for range 20 {
		c := card(t, "g4")
		c.ID = id
		id++
		g.Deck = append(g.Deck, c)
	}
	bots := make([]*Bot, len(hands))
	for i := range bots {
		bots[i] = New([]string{"Alice", "Bob", "Cathy", "Donald", "Emily"}[i], level)
	}
	return g, bots
}

// force 讓目前玩家執行指定行動（模擬給提示的人）。
func force(t *testing.T, g *game.Game, a game.Action) {
	t.Helper()
	if _, err := g.Apply(a); err != nil {
		t.Fatalf("force %+v: %v", a, err)
	}
}

// step 讓目前玩家的 AI 行動並回傳它的選擇。
func step(t *testing.T, g *game.Game, bots []*Bot) game.Action {
	t.Helper()
	p := g.Current
	a := bots[p].Act(g.View(p))
	if _, err := g.Apply(a); err != nil {
		t.Fatalf("%s 的行動 %+v 不合法: %v", bots[p].Name(), a, err)
	}
	return a
}

// notesOf 回傳 bot 對座位 p 第 slot 張牌的筆記（需在 bot 行動過之後）。
func notesOf(b *Bot, g *game.Game, p, slot int) idSet {
	return b.s.notes[g.Hands[p][slot].Card.ID].poss
}

func idents(t *testing.T, s string) idSet {
	var out idSet
	for _, f := range strings.Fields(s) {
		out |= setOf(identOfCard(card(t, f)))
	}
	return out
}

func TestPlayClueIsPlayed(t *testing.T) {
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 r1 g4 b4 w4", "y4 b3 w3 g3 b4")
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Bob 應打出被 play clue 的 r1（slot 2），卻做了 %+v", a)
	}
}

func TestChopFocus(t *testing.T) {
	// 紅色點到 slot 1、2、5：點到 chop → 焦點在 chop（Single Card Focus）
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "r3 r4 g4 y4 r1", "y4 b3 w3 g3 b4")
	force(t, g, game.HintColorTo(1, game.Red))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應打出 chop 上的 r1（slot 5），卻做了 %+v", a)
	}
	if got := notesOf(bots[1], g, 1, 1); got.has(identOf(game.Red, 1)) {
		t.Errorf("非焦點的紅牌依 Good Touch 不會是已打出的 r1，筆記卻是 %v", got.list())
	}
}

func TestFiveSave(t *testing.T) {
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 g4 b3 w4 y5", "y4 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 5))
	a := step(t, g, bots)
	if a.Type == game.ActPlay && a.Slot == 4 {
		t.Fatal("5 Save 不是 play clue，Bob 不該打出")
	}
	want := idents(t, "r5 y5 g5 b5 w5")
	if got := bots[1].s.notes[g.Hands[1][len(g.Hands[1])-1].Card.ID].poss; got != want {
		t.Errorf("5 Save 的筆記應為任何 5，得到 %v", got.list())
	}
}

func TestTwoSave(t *testing.T) {
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 g4 b3 w4 r2", "y4 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 2))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("2 Save 後 Bob 不該打牌，卻做了 %+v", a)
	}
}

func TestGiverSavesFive(t *testing.T) {
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 g4 b3 w4 y5", "y4 b3 w3 g3 b4")
	if a := step(t, g, bots); a != game.HintRankTo(1, 5) {
		t.Fatalf("Alice 應對 Bob chop 上的 y5 做 5 Save，卻做了 %+v", a)
	}
}

func TestGiverTwoSaveVisibleRule(t *testing.T) {
	// Cathy 手上看得到另一張 r2：不能對 Bob 做 2 Save
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 g4 b3 w4 r2", "y4 r2 w3 g3 b4")
	if a := step(t, g, bots); a == game.HintRankTo(1, 2) {
		t.Fatal("另一張 r2 在 Cathy 手上，Alice 不應對 Bob 做 2 Save")
	}
	// 看不到另一張時就要救
	g, bots = scenario(t, "", "g4 y4 b4 w4 g3", "y3 g4 b3 w4 r2", "y4 b3 w3 g3 b4")
	if a := step(t, g, bots); a != game.HintRankTo(1, 2) {
		t.Fatalf("Alice 應對 Bob 的 r2 做 2 Save，卻做了 %+v", a)
	}
}

func TestGiverPrefersColorPlayClue(t *testing.T) {
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 r1 g4 b4 w4", "y4 b3 w3 g3 b4")
	if a := step(t, g, bots); a != game.HintColorTo(1, game.Red) {
		t.Fatalf("Alice 應用紅色提示 Bob 的 r1，卻做了 %+v", a)
	}
}

func TestPrompt(t *testing.T) {
	// Bob 有一張不確定的紅牌；Alice 提示 Cathy 紅色（r4）→ Bob 被 prompt 打出 r3
	g, bots := scenario(t, "r1", "g4 y4 b4 w4 g3", "r2 y4 g4 r3 w4", "y4 r4 w3 g3 b4")
	force(t, g, game.HintColorTo(1, game.Red)) // 焦點 r2，slot 4 的 r3 只知道是之後有用的紅牌
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 應先打出 r2，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.HintColorTo(2, game.Red))
	if got := notesOf(bots[1], g, 1, 3); got.count() < 2 {
		t.Fatalf("prompt 之前 Bob 的紅牌應該還不確定，得到 %v", got.list())
	}
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Bob 被 prompt，應打出 slot 4 的紅牌，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 4 {
		t.Fatalf("Cathy 應接著打出 r4，卻做了 %+v（紅色煙火 %d）", a, g.Fireworks[game.Red])
	}
}

func TestFinesse(t *testing.T) {
	// Bob 沒有被提示的牌；Alice 提示 Cathy 紅色（r4）→ Bob 從 finesse position 盲打 r3
	g, bots := scenario(t, "r2", "g4 y4 b4 w4 g3", "r3 y4 g4 b4 w4", "y4 r4 w3 g3 b4")
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 被 finesse，應盲打 slot 1，卻做了 %+v", a)
	}
	if g.Fireworks[game.Red] != 3 {
		t.Fatal("Bob 的盲打應成功打出 r3")
	}
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Cathy 看到盲打後應打出 r4，卻做了 %+v", a)
	}
}

func TestFinesseNotHappeningMeansDirect(t *testing.T) {
	// Cathy 的紅牌其實是 r3：Bob 看得到所以不盲打；Cathy 看到 Bob 沒盲打就知道是 r3
	g, bots := scenario(t, "r2", "g4 y4 b4 w4 g3", "r3 y4 g4 b4 w4", "y4 r3 w3 g3 b4")
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatal("Cathy 的牌可以直接打，Bob 不該盲打")
	}
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Cathy 應打出 r3，卻做了 %+v", a)
	}
}

func TestGiverAvoidsReverseFinesse(t *testing.T) {
	// 給 Bob 的 r2 提示需要 Cathy（在 Bob 之後）盲打 r1：那是 Level 2 的 Reverse Finesse
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "y3 r2 g4 b4 w4", "r1 y4 w3 g3 b4")
	a := step(t, g, bots)
	if a.Type == game.ActHint && a.Target == 1 && (a.Hint == game.Hint{Kind: game.HintColor, Color: game.Red}) {
		t.Fatal("Level 1 不能給需要 Reverse Finesse 的提示")
	}
}

func TestStrikeResetsNotes(t *testing.T) {
	// Alice 想 finesse 但 Bob 的 finesse position 不是 r3：Bob 打錯後，Cathy 不能再相信那個提示
	g, bots := scenario(t, "r2", "g4 y4 b4 w4 g3", "b5 y4 g4 b4 w4", "y4 r4 w3 g3 b4")
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(0) || g.Strikes != 1 {
		t.Fatalf("Bob 以為被 finesse 應盲打（並打錯），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("打錯後資訊重置，Cathy 不該打出紅牌，卻做了 %+v", a)
	}
}

func TestEndGameBurn(t *testing.T) {
	// 還差 W5、Y4、Y5，牌堆剩 1 張，pace = 22 + 1 + 3 − 25 = 1。
	// Bob 手上沒有能打的牌，Alice 的牌可能是 Y4：Alice 不能棄牌抽走最後一張（Pace +1 Rule），要拖延。
	g, bots := scenario(t, "r5 y3 g5 b5 w4", "y4 b1 g1 r1 b2", "b1 g2 r2 g1 b3", "w5 b1 r1 g1 y5")
	g.Deck = g.Deck[:1]
	g.Current = 1
	force(t, g, game.HintRankTo(2, 5)) // Bob：5 Save Cathy 的 y5（也點到 w5）
	force(t, g, game.HintColorTo(1, game.Blue))
	a := step(t, g, bots)
	if a.Type != game.ActHint {
		t.Fatalf("Alice 應給提示拖延，而不是 %+v", a)
	}
	if g.Deck == nil || len(g.Deck) != 1 {
		t.Fatal("拖延後牌堆應該還剩 1 張")
	}
}

func TestPlayClueOverSaveClue(t *testing.T) {
	// Bob 的 chop 是 g5，但他手上有三張不同的 1：給 1 讓他忙著打牌，比現在 5 Save 好
	// （Beginner's Guide: Give Play Clues Over Save Clues）
	g, bots := scenario(t, "", "g4 y4 b4 w4 g3", "w3 g1 y1 w1 g5", "y4 b3 w3 g3 b4")
	if a := step(t, g, bots); a != game.HintRankTo(1, 1) {
		t.Fatalf("Alice 應先給 Bob 數字 1 的 play clue，卻做了 %+v", a)
	}
}

func TestEarlyGameTrustsNextPlayerToClue(t *testing.T) {
	// Early Game：Bob 的 chop 是 g5，但 Cathy 手上有能打的 b1，Bob 棄牌前一定會先提示 Cathy，
	// 所以 Alice 不用急著處理 Bob 的 chop，直接打出自己被提示的 1
	g, bots := scenario(t, "", "g4 y4 b4 r1 g3", "w3 g1 y1 w1 g5", "y4 b3 b1 g3 b4")
	g.Current = 2
	force(t, g, game.HintColorTo(0, game.Red)) // Cathy 提示 Alice 的 r1（Alice 確定是 r1，打出也不影響 Cathy 的 b1）
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Alice 應打出被提示的 1，卻做了 %+v", a)
	}
}

func TestNoClueThatMayDuplicateMyCard(t *testing.T) {
	// Alice 手上有一張不知道顏色的已提示 1：給 Bob「1」可能和它重複，違反 Good Touch
	g, bots := scenario(t, "", "g4 y4 b4 r1 g3", "w3 g1 y1 w1 g5", "y4 b3 w3 g3 b4")
	g.Current = 2
	force(t, g, game.HintRankTo(0, 1)) // Cathy 提示 Alice 的 1
	if a := step(t, g, bots); a == game.HintRankTo(1, 1) {
		t.Fatal("Alice 自己有未知的 1，不該再提示 Bob 的 1")
	}
}

func TestSaveDoesNotExposeCriticalChop(t *testing.T) {
	// Bob 的 chop 是 g3（唯一看得到的可打牌，要救），左邊是最後一張 g4（另一張已棄）。
	// 「3」會讓 Bob 不確定是 w3 還是 g3 而先不打，chop 移到 g4 後下一手就被棄掉；
	// 「綠色」同時點到 g4 和 g3，兩張都救下來。
	g, bots := scenario(t, "r1 y1 g1 w2", "y4 b4 r3 b4 r3", "y4 w5 b3 g4 g3", "g2 y3 b2 w3 y4")
	g.Discards = append(g.Discards, game.Card{ID: 999, Color: game.Green, Rank: 4})
	g.Current = 1
	force(t, g, game.HintColorTo(2, game.Green)) // Bob 提示 Cathy 的 g2
	force(t, g, game.Discard(4))                 // Cathy 棄牌（進入 Mid-Game）
	a := step(t, g, bots)
	if a == game.HintRankTo(1, 3) {
		t.Fatal("「3」會讓 Bob 的 chop 移到最後一張 g4 上，不該這樣救")
	}
	if a != game.HintColorTo(1, game.Green) {
		t.Fatalf("Alice 應用綠色同時救 g4、g3，卻做了 %+v", a)
	}
}

func TestNoSaveWhenNextPlayerWillPlay(t *testing.T) {
	// Bob 提示 Alice 藍色（b1）。Cathy 在兩人之間、看不到自己的牌，但她知道焦點就是 b1、自己不用盲打，
	// 所以 Alice 輪到時一定確定能打 b1、不會棄 chop 上的 w5：Cathy 不需要現在 5 Save
	g, bots := scenario(t, "r1 y1 g1 w2", "b1 y4 g4 g3 w5", "b2 r4 y4 r3 b4", "g2 y3 b2 w3 g5")
	g.Current = 1
	force(t, g, game.HintColorTo(0, game.Blue))
	if a := step(t, g, bots); a == game.HintRankTo(0, 5) {
		t.Fatal("Alice 下一手會打 b1，Cathy 不需要 5 Save")
	}
}

func TestDeduceByGoodTouchAfterPlay(t *testing.T) {
	// Alice 的紅牌（非焦點）原本是 r3/r4/r5。Bob 之後提示 Cathy 的紅牌時看得到 Alice 那張，
	// 所以 Cathy 打出 r3 後，依 Good Touch Alice 那張不是 r3；r5 又在 Bob 手上看得到 → 一定是 r4
	g, bots := scenario(t, "r1", "g4 y4 b4 r4 r2", "b2 y3 r5 w3 w4", "r3 y4 b4 g4 y3")
	g.Current = 1
	force(t, g, game.HintColorTo(0, game.Red)) // 焦點 chop 上的 r2；slot 4 的紅牌只知道是之後有用的
	force(t, g, game.Discard(3))               // Cathy
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Alice 應先打出 r2，卻做了 %+v", a)
	}
	force(t, g, game.HintColorTo(2, game.Red)) // Bob 提示 Cathy 的 r3
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 3 {
		t.Fatalf("Cathy 應打出 r3，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(4) || g.Fireworks[game.Red] != 4 {
		t.Fatalf("Alice 應推出自己的紅牌是 r4 並打出，卻做了 %+v", a)
	}
}

func TestTempoClueThatFinesses(t *testing.T) {
	// 4 人局。Donald 的 y4 先前被黃色點到（非焦點，只知道是 y3/y4/y5）。
	// Bob 再提示 Donald 黃色只點到舊牌，但會讓 Cathy 從 finesse position 盲打 y3、Donald 接著打 y4：
	// 有新牌被打出，符合 MCVP。Cathy 的 chop 是 r5，她下一手盲打就不會棄牌，所以比直接提示她的 y3 更好。
	g, bots := scenario(t, "y1", "g4 b3 w4 g3", "g4 b4 w4 g3", "y3 b1 g2 r5", "y2 b5 y4 w3")
	force(t, g, game.HintColorTo(3, game.Yellow)) // Alice：焦點 y2，y4 只知道是之後有用的黃牌
	force(t, g, game.Discard(1))                  // Bob（棄 b4，不影響別人的 chop）
	force(t, g, game.HintRankTo(0, 4))            // Cathy
	force(t, g, game.Play(0))                     // Donald 打出 y2
	force(t, g, game.HintRankTo(1, 4))            // Alice
	if a := step(t, g, bots); a != game.HintColorTo(3, game.Yellow) {
		t.Fatalf("Bob 應再提示 Donald 黃色（讓 Cathy finesse 盲打 y3），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Yellow] != 3 {
		t.Fatalf("Cathy 應從 finesse position 盲打 y3，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Yellow] != 4 {
		t.Fatalf("Donald 應接著打出 y4，卻做了 %+v（%s）\n%s", a, bots[3].Why(), dumpHand(bots[3], 3))
	}
}

func TestDoNotTakeNextPlayersOnlyClue(t *testing.T) {
	// 4 人局 Early Game。Cathy 的 chop 是 b5；Alice 手上有能打的 g1。
	// Bob 若自己提示 Alice 的 g1，Cathy 就沒提示可給、只好棄掉 b5；
	// 所以 Bob 應先 5 Save Cathy 的 b5，把 g1 留給 Cathy 提示。
	g, bots := scenario(t, "y1", "b2 r2 g1 g3", "g4 w3 w4 b4", "y1 w2 b3 b5", "w4 b3 w2 y4")
	g.Current = 1
	if a := step(t, g, bots); a != game.HintRankTo(2, 5) {
		t.Fatalf("Bob 應先對 Cathy 做 5 Save，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestSaveLaterChopWithLastHint(t *testing.T) {
	// 4 人局，只剩 1 個提示。Bob 的 chop 是最後一張 r4。Cathy（下一位）手上有已知可打的牌、
	// Alice 輪到 Bob 前也沒提示可用：如果 Donald 拿最後一個提示去給別的 play clue，就沒人能救 r4。
	g, bots := scenario(t, "r2 y1", "r3 g2 w1 w1", "g3 r1 r3 r4", "y2 w4 b3 y4", "b2 r5 b5 g5")
	g.Discards = append(g.Discards, game.Card{ID: 998, Color: game.Red, Rank: 4})
	g.Current = 1
	force(t, g, game.HintColorTo(2, game.Yellow)) // Bob 提示 Cathy 的 y2（Cathy 之後會打牌）
	g.Hints = 1
	g.Current = 3
	a := step(t, g, bots)
	if a != game.HintRankTo(1, 4) && a != game.HintColorTo(1, game.Red) {
		t.Fatalf("Donald 應用最後一個提示救 Bob 的 r4，卻做了 %+v（%s）", a, bots[3].Why())
	}
}

func TestNoTwoSaveWhenOtherCopyInSameHand(t *testing.T) {
	// Bob 的 chop 是 y2，另一張 y2 就在他自己的 slot 1：不用 2 Save（之後救那張就好），
	// 而且「2」會同時點到兩張 y2，違反 Good Touch
	g, bots := scenario(t, "r2 g2", "w3 y4 r3 w5", "y2 y5 r4 y2", "b3 g4 w4 b3", "r4 g5 w3 w2")
	if a := step(t, g, bots); a == game.HintRankTo(1, 2) {
		t.Fatal("另一張 y2 在 Bob 自己手上，不該 2 Save")
	}
}

func TestNoSelfPromptWhenSimplerReadingExists(t *testing.T) {
	// Alice 有兩張被 2 Save 的 2（g2、r2），Bob 有已知的 w2。Cathy 提示 Alice「3」點到 g3：
	// Alice 會讀成接在 w2 後面的 w3（delayed play clue），不會想到要 self-prompt 自己的 2，
	// 所以 Cathy 不該給這個提示，Alice 也不該把那張當成可能的 g3 而去打自己的 2。
	g, bots := scenario(t, "w1 g1", "g3 r4 b4 g2 r2", "y4 r5 w2 g4 r1", "w4 g4 b3 r3 y4")
	g.Current = 2
	force(t, g, game.HintRankTo(0, 2)) // Cathy 2 Save Alice 的 r2（也點到 g2）
	force(t, g, game.HintRankTo(1, 2)) // Alice 提示 Bob 的 w2
	g.Current = 2
	if a := step(t, g, bots); a == game.HintRankTo(0, 3) {
		t.Fatal("Alice 會把「3」讀成 w3（delayed），不會 self-prompt：Cathy 不該這樣提示 g3")
	}
}

func TestNoSelfPromptWhenGiverHoldsConnection(t *testing.T) {
	// 同上，但 w2 在給提示者 Cathy 自己手上（Alice 提示過她「2」，她不確定是哪個 2）。
	// Alice 看得到 Cathy 的 w2，一樣會把「3」讀成 w3；Cathy 看不到自己的牌，也要想到這點。
	g, bots := scenario(t, "w1 g1", "g3 r4 b4 g2 r2", "y4 r5 w4 g4 r1", "w2 g4 b3 r3 y4")
	g.Current = 1
	force(t, g, game.HintRankTo(0, 2)) // Bob 2 Save Alice 的 r2（也點到 g2）
	force(t, g, game.HintRankTo(1, 1)) // Cathy 提示 Bob 的 r1
	force(t, g, game.HintRankTo(2, 2)) // Alice 提示 Cathy 的 w2
	force(t, g, game.Discard(0))       // Bob
	if a := step(t, g, bots); a == game.HintRankTo(0, 3) {
		t.Fatal("Alice 看得到 Cathy 的 w2，會把「3」讀成 w3：Cathy 不該這樣提示 g3")
	}
}

func TestReceiverEliminatesVisibleCopies(t *testing.T) {
	// Bob 的 chop 是 w2、slot 3 是 w3。一張 w4 已棄掉，最後一張 w4 在 Alice 手上、w5 在 Cathy 手上。
	// 白色點到 chop：Bob 看得到最後一張 w4 在 Alice 手上，知道不是 w4 的 critical save，所以是 w2；
	// w3 也一樣確定。一個白色提示讓兩張牌都能打，比只點 w2 的「2」好。
	g, bots := scenario(t, "r5 y3 g5 b4 w1", "b2 w4 r4 w1 b5", "r3 r1 w3 w2 y5", "w5 b1 w2 r1 r2")
	g.Discards = append(g.Discards, game.Card{ID: 997, Color: game.White, Rank: 4})
	force(t, g, game.HintRankTo(1, 5)) // Alice 5 Save Bob 的 y5，Bob 的 chop 變成 w2
	force(t, g, game.HintRankTo(0, 5)) // Bob 5 Save Alice 的 b5
	if a := step(t, g, bots); a != game.HintColorTo(1, game.White) {
		t.Fatalf("Cathy 應給 Bob 白色，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestBadTouchedCardIsNotPlayed(t *testing.T) {
	// 「1」點到 Bob 的 y1（已打出，垃圾）和 chop 上的 w1。焦點在 chop（w1）；
	// 非焦點那張找不到「之後有用、又不是 w1」的可能，是違反 Good Touch 的牌，Bob 要打 chop 上的焦點
	g, bots := scenario(t, "r1 y1 g2 b1", "g4 r5 b4 g4 w5", "g5 w3 y1 r4 w1", "y4 b5 r3 b3 b3")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應打出焦點（chop 上的 w1，slot 5），卻做了 %+v", a)
	}
}

func TestTouchingProvableTrashIsAllowed(t *testing.T) {
	// Bob 的 chop 是 w1，另有一張 y1（已打出）。w4 已棄一張、最後一張不在看得到的地方，
	// 所以白色會被讀成「w1 或 w4 的 critical save」；「1」雖然也點到 y1，但 Bob 推得出那張
	// 不可能有用（有用的 1 只剩焦點 w1），不會誤會：Alice 應給「1」。
	g, bots := scenario(t, "r1 y1 g2 b1", "g4 r5 b4 g4 w5", "g5 w3 y1 r4 w1", "y4 b5 r3 b3 b3")
	g.Discards = append(g.Discards, game.Card{ID: 996, Color: game.White, Rank: 4})
	if a := step(t, g, bots); a != game.HintRankTo(1, 1) {
		t.Fatalf("Alice 應給 Bob「1」，卻做了 %+v（%s）", a, bots[0].Why())
	}
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應打出焦點（chop 上的 w1），卻做了 %+v", a)
	}
	// w1 打出後，y1 那張被提示的牌就是已知沒用的牌（不會是 w1 以外的有用牌）
	y1 := g.Hands[1][3].Card.ID
	bots[1].s.sync(g.View(1))
	if !bots[1].s.knownTrash(y1) {
		t.Errorf("Bob 應知道那張被點到的 1 已經沒用，筆記是 %v", bots[1].s.notes[y1].poss.list())
	}
}

func TestNoPlayableSaveThatMayDuplicateMine(t *testing.T) {
	// Bob 的 chop 是 r1（看得到的唯一 r1），但 Alice 手上有兩張不知道顏色的已提示 1：
	// 其中一張可能就是 r1，Alice 去救可能造成重複，r1 也不是 critical，交給別人
	g, bots := scenario(t, "", "y4 r3 g3 w1 r1", "b3 r2 y3 b4 r1", "b2 g5 g2 r2 g3")
	g.Current = 2
	force(t, g, game.HintRankTo(0, 1)) // Cathy 提示 Alice 的兩張 1
	a := step(t, g, bots)
	if a.Type == game.ActHint && a.Target == 1 && (a.Hint == game.Hint{Kind: game.HintColor, Color: game.Red} || a.Hint == game.Hint{Kind: game.HintRank, Rank: 1}) {
		t.Fatalf("Alice 自己可能有 r1，不該去救 Bob 的 r1：%+v（%s）", a, bots[0].Why())
	}
}

func TestKnowsNextPlayerWillPlay(t *testing.T) {
	// Bob 有兩張被提示、不知道顏色的 1（實際 w1、r1）。Alice 提示 Cathy 黃色（y1）時看得到 Bob 的 1，
	// 依 Good Touch Bob 的 1 不是 y1，所以 Cathy 確定自己那張是 y1、下一手會打，不會棄 chop 上的 g5：
	// Bob 不必急著救，直接打自己的 1。
	g, bots := scenario(t, "", "b2 g4 g2 r2 g3", "g4 r3 g3 w1 r1", "b3 r2 r4 g5 y1")
	force(t, g, game.HintRankTo(1, 1))            // Alice 提示 Bob 的兩張 1
	force(t, g, game.Discard(0))                  // Bob
	force(t, g, game.HintRankTo(0, 2))            // Cathy
	force(t, g, game.HintColorTo(2, game.Yellow)) // Alice 提示 Cathy 的 y1（chop 移到 g5）
	if a := step(t, g, bots); a.Type != game.ActPlay {
		t.Fatalf("Cathy 下一手會打 y1，Bob 應直接打自己的 1，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestNoUntimelySelfPrompt(t *testing.T) {
	// Alice 有一張被 2 Save 的 r2（不知道顏色），Cathy 有兩張已提示、還沒打的 1（w1、r1）。
	// 「3」點到 Alice 的 r3 會是 self-prompt，但她的 2 可能是 y2/g2（現在能打）或 r2/w2（要等 Cathy 的 1），
	// 她不知道什麼時候能打。Bob 應改用紅色：r2 直接被填成紅色，r3 是 delayed play clue。
	g, bots := scenario(t, "y1 g1", "r3 w5 y1 y5 r2", "b2 g5 r4 b3 y4", "y3 b5 w1 y3 r1")
	g.Current = 2
	force(t, g, game.HintRankTo(0, 2)) // Cathy 2 Save Alice 的 r2
	force(t, g, game.HintRankTo(2, 1)) // Alice 提示 Cathy 的 w1、r1
	if a := step(t, g, bots); a == game.HintRankTo(0, 3) {
		t.Fatal("Alice 不知道她的 2 什麼時候能打，Bob 不該給需要 self-prompt 的「3」")
	}
}

func TestFinesseWhenVisiblePromptCandidateIsWrong(t *testing.T) {
	// Cathy 有一張非焦點的已提示 4（實際 r4，她只知道是 r4/b4/w4 之一）。Alice 提示 Cathy 的 w5：
	// 如果 Bob 不動，Cathy 會把自己的 4 當 w4 self-prompt；但 Bob 看得到那張是 r4、Alice 也看得到，
	// Alice 不可能想 prompt 它，所以這是 finesse Bob（他的 finesse position 是 w4）。
	g, bots := scenario(t, "r3 y3 g5 b2 w3", "r1 b1 g2 b2 b5", "w4 b3 y2 r5 y5", "w5 w1 y4 r4 w1")
	g.Current = 1
	force(t, g, game.HintRankTo(2, 4)) // Bob：焦點 y4，r4 是非焦點
	force(t, g, game.Play(2))          // Cathy 打出 y4
	force(t, g, game.HintRankTo(1, 5)) // Alice 5 Save Bob 的 r5、y5
	force(t, g, game.HintRankTo(0, 1)) // Bob
	force(t, g, game.HintRankTo(0, 2)) // Cathy
	force(t, g, game.HintRankTo(2, 5)) // Alice 提示 Cathy 的 w5
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 4 {
		t.Fatalf("Bob 應從 finesse position 盲打 w4，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestNoTrustWhenNextHasOnlyDirtyClues(t *testing.T) {
	// Early Game。Cathy（Bob 的下一位）chop 上是 y5，她看得到能打的只有 Alice 的兩張 y2：
	// 黃色和「2」都會同時點到兩張 y2（違反 Good Touch），所以她沒有乾淨的提示可給、只能棄 y5。
	// Bob 不能相信她會先給提示，要先 5 Save。
	// Bob 自己有已知能打的 r1，但要先救（Check Team Chops：就算自己有牌可打也先救）
	g, bots := scenario(t, "y1", "y2 r4 g5 y2 b4", "y3 r1 g4 w4 b3", "g4 b2 r4 y4 y5")
	g.Current = 2
	force(t, g, game.HintColorTo(1, game.Red)) // Cathy 提示 Bob 的 r1
	force(t, g, game.HintRankTo(1, 3))         // Alice
	if a := step(t, g, bots); a != game.HintRankTo(2, 5) {
		t.Fatalf("Bob 應先 5 Save Cathy，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestBurnWhenTeammateHoldsTwoNeededCards(t *testing.T) {
	// 還差 g5、r5，兩張都在 Cathy 手上，牌堆剩 1 張。Bob 若棄牌（就算是已知垃圾），
	// 最後一輪 Cathy 只剩一回合、只能打一張；Bob 應給拖延提示，讓 Cathy 打 g5 時才抽最後一張。
	g, bots := scenario(t, "r4 y5 g4 b5 w5", "b4 r1 r3 y4 w1", "g1 r4 b3 b2 g3", "g5 b1 y2 b1 r5")
	g.Deck = g.Deck[:1]
	force(t, g, game.HintRankTo(2, 5)) // Alice 提示 Cathy 的兩張 5（Cathy 知道都能打）
	a := step(t, g, bots)
	if a.Type != game.ActHint || len(g.Deck) != 1 {
		t.Fatalf("Bob 應給提示拖延（不抽走最後一張），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestBurnWhenTeammateNeedsMoreTurns(t *testing.T) {
	// 還差 b4（Cathy）與 r5、y5、b5（都在 Alice 手上），牌堆剩 2 張。Bob 若棄牌，
	// 之後每回合都抽牌的話 Alice 只剩一回合；Bob 不抽牌就能讓她多一回合 → 應給拖延提示
	g, bots := scenario(t, "r4 y4 g5 b3 w5", "w1 b2 b5 r5 y5", "r1 g2 w3 y1 g1", "b4 g3 r2 w2 y2")
	g.Deck = g.Deck[:2]
	g.Hints = 6
	force(t, g, game.HintColorTo(2, game.Blue)) // Alice：Cathy 的 b4
	force(t, g, game.HintRankTo(0, 5))          // Bob：Alice 的三張 5
	force(t, g, game.HintRankTo(0, 1))          // Cathy
	force(t, g, game.HintRankTo(2, 3))          // Alice
	a := step(t, g, bots)
	if a.Type != game.ActHint || len(g.Deck) != 2 {
		t.Fatalf("Bob 應給提示拖延（不抽牌），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestDelayedReadingDropsWhenConnectionIsWrong(t *testing.T) {
	// Bob 有一張已提示、不知道顏色的 1（實際 y1）。Cathy 提示 Bob「2」（y2）：
	// 當下可能是 y2、g2（都靠 Bob 那張 1 接上）或 b2。Bob 打出 1 發現是 y1 後，g2 已經接不上，
	// 那張 2 只剩 y2/b2，都能打。
	g, bots := scenario(t, "b1", "y5 b3 r3 g2 y3", "b4 y1 b5 y2 g3", "y4 w2 w3 g4 b4")
	force(t, g, game.HintRankTo(1, 1)) // Alice 提示 Bob 的 y1
	force(t, g, game.HintRankTo(0, 3)) // Bob
	force(t, g, game.HintRankTo(1, 2)) // Cathy 提示 Bob 的 y2
	force(t, g, game.HintRankTo(2, 4)) // Alice
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Yellow] != 1 {
		t.Fatalf("Bob 應先打出 1（y1），卻做了 %+v", a)
	}
	force(t, g, game.HintRankTo(0, 5)) // Cathy
	force(t, g, game.HintRankTo(2, 3)) // Alice
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Yellow] != 2 {
		t.Fatalf("Bob 應打出 2（y2），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestTouchedDuplicateOfCluedCardIsTrash(t *testing.T) {
	// Bob 已知 chop 上的是 y1（數字 1 提示）；之後抽到 b1，Cathy 的「1」同時點到它和 y1（1 只剩 y1 有用）：
	// 新的那張只會是垃圾或 y1 的重複，Bob 不能把它當成 y1 打
	g, bots := scenario(t, "r1 g1 b1 w1", "g4 y4 b4 w4 g3", "g4 r3 b4 r4 y1", "y4 b3 w3 g3 b4")
	b1 := card(t, "b1")
	b1.ID = 999
	g.Deck = append(g.Deck, b1)
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 1))
	force(t, g, game.Discard(0))       // Bob 抽到 b1
	force(t, g, game.HintRankTo(1, 1)) // Cathy
	force(t, g, game.Discard(0))       // Alice
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應打出已知的 y1（slot 5），不是新點到的那張 1，卻做了 %+v", a)
	}
	bots[1].s.sync(g.View(1))
	if !bots[1].s.knownTrash(999) {
		t.Errorf("被點到的另一張 1 應是已知沒用的牌，筆記卻是 %v", bots[1].s.notes[999].poss.list())
	}
}

func TestPlayFocusFirst(t *testing.T) {
	// 「1」點到 Bob 的 slot 1 和 chop（slot 3；slot 4、5 已被提示）。剩下能打的 1 都能打，
	// 兩張都確定能打，但 play clue 的意思是打出焦點牌（Single Card Focus），焦點在 chop → 先打 chop
	g, bots := scenario(t, "r1 g1", "g4 y4 b4 w4 g3", "y1 y3 b1 r4 w4", "y4 b3 w3 g3 b4")
	g.Hints = 5
	force(t, g, game.HintRankTo(1, 4))
	force(t, g, game.HintRankTo(2, 3)) // Bob
	force(t, g, game.Discard(4))       // Cathy
	force(t, g, game.HintRankTo(1, 1)) // Alice
	if a := step(t, g, bots); a != game.Play(2) {
		t.Fatalf("Bob 應先打焦點（chop 上的 1，slot 3），卻做了 %+v", a)
	}
}

func TestPlayInsteadOfSaveWhenPlayUnlocksNext(t *testing.T) {
	// Bob 已知自己有 b2；Alice 提示 Cathy「3」點到 b3（等 Bob 的 b2 的 delayed play clue）。
	// Cathy 的 chop 是 b5、她現在沒牌可打，但 Bob 打出 b2 後她的 3 就能打、不會棄 chop：
	// Bob 應直接打 b2，不必花提示救
	g, bots := scenario(t, "b1", "r3 y3 y4 r2 r5", "w4 b2 g3 w3 w5", "b3 g4 g4 y1 b5")
	force(t, g, game.HintColorTo(1, game.Blue)) // Alice：Bob 的 b2
	force(t, g, game.Discard(3))                // Bob（Early Game 結束）
	force(t, g, game.HintRankTo(1, 4))          // Cathy
	force(t, g, game.HintRankTo(2, 3))          // Alice：Cathy 的 b3
	if a := step(t, g, bots); a != game.Play(2) {
		t.Fatalf("Bob 應打出 b2（slot 3），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestDeferTwoSaveWhenMyClued2MayDuplicate(t *testing.T) {
	// Alice 的 chop 被 2 Save（實際 w2，她只知道是某張 2）；Donald 的 chop 也是 w2。
	// Alice 救 Donald 的 w2 可能和自己的 2 重複（違反 Good Touch），而 Bob、Cathy 都沒有已提示的 2：
	// 交給他們（他們看得到 Alice 的是 w2，根本不用救）
	g, bots := scenario(t, "", "r3 y3 g4 w2", "y4 g3 r4 b4", "b3 g4 y4 r4", "b4 g3 y3 w2")
	force(t, g, game.HintRankTo(1, 4)) // Alice
	force(t, g, game.HintRankTo(0, 2)) // Bob：2 Save Alice 的 w2
	force(t, g, game.HintRankTo(1, 3)) // Cathy
	force(t, g, game.HintRankTo(2, 4)) // Donald
	a := step(t, g, bots)
	if a.Type == game.ActHint && a.Target == 3 && a.Hint.Matches(g.Hands[3][3].Card) {
		t.Fatalf("Alice 不該點 Donald 的 w2（可能和自己的 2 重複），卻做了 %+v（%s）", a, bots[0].Why())
	}
}

func TestClueBeforeOwnPlayWhenCluePerishes(t *testing.T) {
	// Bob 已知 w1 能打。Cathy 有 y1、r1、b1，Alice 也有 r1：Bob 若先打 w1，Cathy 多半會提示 Alice 的 r1，
	// 之後再給 Cathy「1」就會點到重複的 r1。趁現在給 Cathy「1」（一次三張），w1 晚點打
	g, bots := scenario(t, "", "b4 r2 r1 r2 g2", "b3 g3 r3 w1 y2", "b1 b5 w3 r1 y1")
	force(t, g, game.HintColorTo(1, game.White))
	if a := step(t, g, bots); a != game.HintRankTo(2, 1) {
		t.Fatalf("Bob 應先給 Cathy「1」，卻做了 %+v（%s）", a, bots[1].Why())
	}
	// 對照：Alice 沒有 r1 時，晚點給也一樣乾淨，Bob 照常先打 w1
	g, bots = scenario(t, "", "b4 r2 g4 r2 g2", "b3 g3 r3 w1 y2", "b1 b5 w3 r1 y1")
	force(t, g, game.HintColorTo(1, game.White))
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("提示不會失效時 Bob 應先打 w1，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestNonFocusCardIsNotDuplicateOfCluedCard(t *testing.T) {
	// y3 已打出。Bob 的 w5、y5 被 5 Save；Cathy 的 y4 已被提示。Alice 提示 Bob 黃色，點到 y1、y2（chop，焦點）
	// 和 y5（現在明確是 y5）。非焦點的 y1 依 Good Touch 只會是有用、且別處沒被提示的牌：y4、y5 都已被拿到 → 是沒用的牌，
	// 不能把它記成 y5，否則 Bob 會以為真正的 y5 是重複而棄掉
	g, bots := scenario(t, "y3 g2", "r4 b4 r3 b3 g3", "g2 y1 y2 w5 y5", "y4 b4 r4 w4 g4")
	force(t, g, game.HintRankTo(1, 5))            // Alice：5 Save
	force(t, g, game.HintColorTo(2, game.Yellow)) // Bob：Cathy 的 y4
	force(t, g, game.HintRankTo(0, 4))            // Cathy
	force(t, g, game.HintColorTo(1, game.Yellow)) // Alice
	a := step(t, g, bots)
	for _, c := range g.Discards {
		if c.Color == game.Yellow && c.Rank == 5 {
			t.Fatalf("Bob 不該棄掉 y5，卻做了 %+v（%s）", a, bots[1].Why())
		}
	}
	if a.Type == game.ActPlay {
		t.Fatalf("Bob 沒有能打的牌，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestPlayCardThatLeadsIntoOthersCluedCardFirst(t *testing.T) {
	// Cathy 已知 b1 與 g2 都能打；Bob 有已被提示的 g3。先打 g2（接上別人已提示的牌，Level 25 Priority 的順序），
	// 不是數字小的 b1
	g, bots := scenario(t, "g1", "r4 y4 r3 y3 w3", "g3 r2 y2 w4 b4", "b1 r4 y4 g2 w3")
	g.Hints = 5
	force(t, g, game.HintColorTo(2, game.Green)) // Alice：Cathy 的 g2
	force(t, g, game.HintRankTo(2, 1))           // Bob：Cathy 的 b1
	force(t, g, game.HintColorTo(1, game.Green)) // Cathy：Bob 的 g3
	force(t, g, game.Discard(0))                 // Alice
	force(t, g, game.Discard(4))                 // Bob
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Cathy 應先打能接上 Bob 的 g3 的 g2（slot 4），卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestDoNotTakeLastHintFromLockedNextPlayer(t *testing.T) {
	// Cathy 手牌全被提示、沒有能打的牌，提示只剩 1 個。Bob 若用掉它（給 Alice r2 的 play clue），
	// Cathy 只能棄一張已提示的牌；Bob 應自己棄牌把提示換回來
	g, bots := scenario(t, "r1 y1 g1 b1 w1", "r2 w3 w2 g3 b2", "b5 g4 r3 y3 y5", "b4 r4 r5 w5 w4")
	g.Hints = 3
	force(t, g, game.HintRankTo(2, 4)) // Alice
	force(t, g, game.HintRankTo(2, 5)) // Bob
	force(t, g, game.HintRankTo(1, 5)) // Cathy
	force(t, g, game.Discard(3))       // Alice
	if a := step(t, g, bots); a.Type != game.ActDiscard {
		t.Fatalf("Bob 應棄牌把提示留給鎖手的 Cathy，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestEarlyGameTrustNeedsCluePlayersCanReallyGive(t *testing.T) {
	// Early Game。Cathy 的 chop 是 y2、她沒牌可打。Alice 的 r2 看起來能給 play clue，但紅色會點到 chop 上的 r4
	// （焦點變成 r4）、「2」的焦點是 g2：Cathy 其實沒有乾淨的提示可給，會棄牌。Bob 不能相信她，要先處理
	g, bots := scenario(t, "r1", "g2 r2 y4 r5 r4", "w1 b2 b1 b1 y3", "b4 y1 b3 b2 y2")
	force(t, g, game.HintColorTo(1, game.White)) // Alice：Bob 的 w1
	a := step(t, g, bots)
	if a.Type != game.ActHint || a.Target != 2 {
		t.Fatalf("Bob 應先處理 Cathy 的 chop（給她 play clue 或救 y2），卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestNoFinesseThatMayDuplicateMyCluedCard(t *testing.T) {
	// Bob 有兩張不知道顏色的已提示 1。Cathy 的 chop 是 g5、finesse position 是 y1；用黃色點 Alice 的 y2 去 finesse
	// Cathy 的 y1，可能和 Bob 自己的 1 重複（違反 Good Touch，刻意這樣做是 Level 10 Sarcastic Finesse）：不該給
	g, bots := scenario(t, "", "y2 g2 b3 b4 r3", "w1 g1 w5 b5 g4", "y1 b2 r3 w4 g5")
	force(t, g, game.HintRankTo(1, 1)) // Alice：Bob 的兩張 1
	if a := step(t, g, bots); a.Type == game.ActHint && a.Target == 0 && a.Hint.Matches(g.Hands[0][0].Card) {
		t.Fatalf("Bob 不該提示 Alice 的 y2 去 finesse 可能和自己的 1 重複的 y1：%+v（%s）", a, bots[1].Why())
	}
}

func TestNoTempoClueForPendingSelfPrompt(t *testing.T) {
	// Cathy 的紅牌 r2 早已被提示（和 r1 一起）。Alice 提示 Cathy「3」點到 r3：Cathy 看得到 Bob 的 finesse position
	// 不是 2，只剩 self-prompt 她自己的紅牌 r2 這個解讀，下一手就會打 r2。Bob 不必再提示紅色——只重新點到舊牌、
	// prompt 出的也是已被提示的牌，不符合 Minimum Clue Value Principle。
	for lv := 1; lv <= MaxLevel; lv++ {
		g, bots := scenarioAt(t, lv, "y1 g1 b1", "g4 y4 b4 w4 g3", "w4 y4 b4 g4 y3", "r1 r2 y4 b4 g4")
		g.Hints = 5
		g.Deck = append(g.Deck[:10:10], g.Deck...) // 牌堆夠多，不會進入終局拖延
		force(t, g, game.HintColorTo(2, game.Red))
		force(t, g, game.Discard(4)) // Bob
		drawNext(t, g, "r3")
		if a := step(t, g, bots); a != game.Play(0) {
			t.Fatalf("Lv%d：Cathy 應打出 r1，卻做了 %+v", lv, a)
		}
		force(t, g, game.HintRankTo(2, 3))
		if a := step(t, g, bots); a.Type == game.ActHint && a.Target == 2 {
			t.Fatalf("Lv%d：Bob 不該再提示 Cathy（她已經會打 r2），卻做了 %+v", lv, a)
		}
		if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.Red] != 2 {
			t.Fatalf("Lv%d：Cathy 應 self-prompt 打出 r2（slot 2），卻做了 %+v", lv, a)
		}
	}
}

func TestSaveLaterCriticalChopOverNextPlayableChop(t *testing.T) {
	// 只剩 1 個提示。下一位 Bob 的 chop 是唯一看得到的可打牌 y3（不是 critical），再下一位 Cathy 的 chop 是 critical 的 r4。
	// 我用掉最後一個提示救 Bob，輪到 Cathy 前 Bob 沒提示可給，Cathy 就會棄掉 r4：應該改救 Cathy。
	g, bots := scenarioAt(t, 1, "y2 g1", "g4 b4 w4 g3 b3", "g4 b4 w4 g3 y3", "b3 g4 w3 r4 g1")
	g.Discards = append(g.Discards, card(t, "r4"))
	g.Hints = 0
	g.Current = 2
	force(t, g, game.Discard(4)) // Cathy 棄掉 g1，進入 Mid-Game，提示變 1
	if a := step(t, g, bots); a.Type != game.ActHint || a.Target != 2 {
		t.Fatalf("Alice 應用最後一個提示救 Cathy 的 critical r4，卻做了 %+v", a)
	}
}

func TestPriorityLeadsIntoMeansNextCard(t *testing.T) {
	// 5 人：r2 y2 已打出。Bob 確定有 r3（slot 2）和 y3（slot 3），都能打。Cathy 有已提示的 y4 和 r5。
	// y3 打出後 Cathy 的 y4 就能打；r3 打出後 r5 還要等 r4，接不上——Level 25 Priority：先打能接上別人已提示的牌
	// （「接上」是指那張正好是下一張，否則照順序會先打左邊的 r3）。
	g, bots := scenarioAt(t, 2, "r2 y2", "g4 b4 w4 g3", "w4 r3 y3 b4", "y4 r5 b3 w3", "b4 w3 b2 w2", "w4 b3 b2 g4")
	g.Current = 2
	force(t, g, game.HintColorTo(1, game.Red))
	force(t, g, game.HintColorTo(1, game.Yellow))
	force(t, g, game.HintRankTo(2, 5))
	force(t, g, game.HintColorTo(2, game.Yellow))
	if a := step(t, g, bots); a != game.Play(2) || g.Fireworks[game.Yellow] != 3 {
		t.Fatalf("Bob 應先打能接上 Cathy y4 的 y3（slot 3），卻做了 %+v", a)
	}
}

func TestPlayInsteadOfSaveWhenItTellsNextHeCanPlay(t *testing.T) {
	// b3 w3 已打出。Alice 確定有 b4。Bob 被提示的藍牌（b5）他只知道是 b4 或 b5。
	// Alice 打出 b4 後，Bob 依 Good Touch 排除已打出的 b4，就知道藍牌是能打的 b5，不會棄 chop（playLetsNextPlay）。
	g, bots := scenarioAt(t, 3, "b3 w3", "g3 y2 b4 r2", "b5 y3 g3 w4", "y4 g4 r3 w3", "r4 y2 g2 w2")
	g.Current = 2
	force(t, g, game.HintRankTo(0, 4))
	force(t, g, game.HintColorTo(1, game.Blue))
	bots[0].Act(g.View(0))
	s := bots[0].s
	s.notes[g.Hands[1][0].Card.ID].poss = idents(t, "b4 b5") // 實戰中常見的筆記（例如 b4 直接打、或 b5 接上 Alice 的 4）
	if !s.playLetsNextPlay(1) {
		t.Fatal("Alice 打出 b4 後 Bob 的藍牌只能是 b5，應判定他會打牌")
	}
}

func TestPreferClueThatPlaysMoreCards(t *testing.T) {
	// Bob 的 chop 是 b1，另有 w1、b4：「1」讓 b1、w1 都能打，比藍色（b1 能打、b4 只是有用）好
	g, bots := scenario(t, "r1 g1", "g4 y4 r4 w4 g3", "r4 g2 b4 w1 b1", "r3 y3 g3 r4 y4")
	if a := step(t, g, bots); a != game.HintRankTo(1, 1) {
		t.Fatalf("Alice 應給 Bob「1」（b1、w1 都能打），卻做了 %+v", a)
	}
}

func TestRefreshFocusIgnoresReadingsContradictingVisibleCard(t *testing.T) {
	// 我（Alice）看得到 Bob 被提示的牌是 b2，但我推測的接收者解讀只剩 y2、w2（例如被別的提示的 layer 誤導）：
	// 推測不可靠，不能把 Bob 那張的筆記縮成 y2/w2——否則之後我會以為沒有人有 b2，把需要 b2 的提示讀成 finesse 自己
	g, bots := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "b2 r4 g4 y4 w4", "y3 r4 g3 b3 w3")
	force(t, g, game.HintRankTo(1, 2))
	bots[0].Act(g.View(0))
	s := bots[0].s
	id := g.Hands[1][0].Card.ID
	before := s.notes[id].poss
	rec := &clueRec{giver: 2, target: 1, focus: id, cands: []cand{{id: identOf(game.Yellow, 2)}, {id: identOf(game.White, 2)}}}
	s.refreshFocus(rec)
	if got := s.notes[id].poss; got != before {
		t.Fatalf("看得到實際是 b2，筆記不該被縮成 %v（原本 %v）", got.list(), before.list())
	}
}

func TestLeaveSaveToPlayerWhoSeesEverything(t *testing.T) {
	// 使用者回報 20261014-162711：AI-1 用「2」救了 Cathy（AI-2）chop 上的 g2（Double Chop 2's，Alice 的 chop 也是 g2）。
	// Cathy 不知道自己兩張 2 是哪張，可能就是 g2：她不該冒重複的險去救 Alice 的 g2——Bob 手上沒有被提示的 2，
	// 看得到兩邊，交給他判斷（他看到 Cathy 的 g2，依 Visible Rule 就不會救）。2 人局沒有這樣的人，照救
	g, bots := scenarioAt(t, 5, "", "r3 y2 w5 g4 g2", "y2 r1 r3 y1 y1", "r1 b2 w3 w3 g2")
	force(t, g, game.HintColorTo(1, game.Red))
	force(t, g, game.HintRankTo(2, 2))
	a := step(t, g, bots)
	if a.Type == game.ActHint && a.Target == 0 && (a.Hint.Matches(g.Hands[0][4].Card)) {
		t.Fatalf("Cathy 的 2 可能是 g2，不該救 Alice 的 g2（Bob 看得到兩邊），卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestNoUrgentSaveWhenNextHasKnownTrash(t *testing.T) {
	// 使用者回報 20261014-175719 t23：1 都打出了。Cathy 手上有被「1」點到、大家都知道是垃圾的牌，chop 是 y5。
	// 她下一手會先棄那張垃圾，不會棄 y5——Bob 不必急著 5 Save，先打自己確定能打的 r2
	g, bots := scenarioAt(t, 3, allOnes, "g4 y4 b4 w4 g3", "r2 g4 b3 w3 y3", "w4 r1 g4 b4 y5")
	g.Current = 2
	force(t, g, game.HintColorTo(1, game.Red)) // Cathy：Bob 的 r2
	force(t, g, game.HintRankTo(2, 1))         // Alice：Cathy 的 r1（垃圾）
	if a := step(t, g, bots); a.Type != game.ActPlay {
		t.Fatalf("Cathy 會先棄已知垃圾，Bob 應先打 r2，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestMyClueRulesOutReadingsThatNeedMyBlindPlay(t *testing.T) {
	// 使用者回報 20261016-094141 t7：AI-3 chop 上的藍牌（play clue 的焦點）是「b3 或 b4」——b4 那種要 AI-2 盲打 b3。
	// AI-1 新抽到 b5，AI-2 提示藍色就是 prompt AI-3 另一張藍牌當 b4。AI-2 給提示（不盲打），大家就排除了要他盲打的那種解讀，
	// AI-3 知道那張是 b3。以前模擬 AI-3 的角度時還用「b3 或 b4」，以為他不會被 prompt、提示會誤導而改給 5 Stall
	g, bots := replayCodes(t, 4, 1792092289206223300, 5, "h12 h2B p3 h15 h3B p4")
	if a := step(t, g, bots); a != game.HintColorTo(1, game.Blue) {
		t.Fatalf("AI-2 應提示 AI-1 藍色（prompt AI-3 的 b4），卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestLockedHandDiscardsRatherThanMislead(t *testing.T) {
	// Rainbow 2 人局 seed 0（Lv4）t39：P0 鎖手，能給的提示都會誤導——例如紅色點到 P1 的 r4、m2、r2，
	// P1 會把 r4 當成 r3（之後打錯）。這時寧可棄掉最不可能 critical 的已提示牌，也不給誤導的拖延提示
	g, bots := replayCodesVariant(t, 2, game.RainbowSix, 0, 4,
		"h11 p5 d5 p5 h1G p1 h12 p1 d5 h0R p1 d5 d5 h05 d5 h0G d5 h02 h1B d2 h1W h0R d3 h03 h13 p1 h1G h0Y h14 h0G d4 h0W d4 h04 p2 d3 d2 h05")
	if a := step(t, g, bots); a.Type != game.ActDiscard {
		t.Fatalf("鎖手又只剩誤導的提示時應棄牌，卻 %+v（%s）", a, bots[0].s.why)
	}
}

func TestNextPlayerMayNotClueMyPossibleDuplicate(t *testing.T) {
	// 使用者回報 20261016-164435（Rainbow、Lv5）：t1 使用者用「1」點到 AI-1 的 w1。AI-2 chop 上是 m2（唯一看得到的那張）。
	// AI-1 不能以為 AI-2 會先給使用者「1」（點到 g1、w1）而不救：AI-1 那張 1 可能就是 w1 或 g1，AI-2 看得到它，
	// 那個提示可能違反 Good Touch（實際上 w1 就是重複的）。Early Game 中他沒有確定能給的提示，就要 2 Save 他
	g, bots := replayCodesVariant(t, 3, game.RainbowSix, 1792140275979048000, 5, "h11")
	if a := step(t, g, bots); a != game.HintRankTo(2, 2) {
		t.Fatalf("AI-1 應 2 Save AI-2 chop 上的 m2，卻 %+v（%s）", a, bots[1].s.why)
	}
}

func TestOwnUnknownCardNotCountedTwice(t *testing.T) {
	// 自我對局 5 人局 seed 1667：P4 被紅色點到兩張紅牌，chop 上的焦點是「r1 或 r2」。t3 別人提示 P0 的 r4，P4 替它找連接牌時
	// 把自己那張焦點同時算成 r1 和 r2，剩下的 r3 就落在自己另一張紅牌上（實際是 r1）、寫成 r3，後來變成 r5 而打出
	g, bots := replayCodes(t, 5, 1667, 5, "h4R h25 h0R p1 h15 h4Y h34 h3Y p2 p1 p1 h4W d2 p1 p4 h22 p1 d1 p3 p4 p2 h4B d1 p4 p3 h31 h32 p2 p2")
	if a := bots[4].Act(g.View(4)); a == game.Play(3) {
		t.Errorf("P4 不該把 slot 4（r1）當成 r5 打出（%s）", bots[4].Why())
	}
}
