package hgroup

import (
	"fmt"
	"strings"
	"testing"

	"hanabi/game"
)

// Level 3（Basic Strategy）的範例情境，取自 H-Group 原文 level-3.mdx 與 Level 3 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。

// drawNext 讓接下來抽到的牌依序是 cs（第一張先抽）。
func drawNext(t *testing.T, g *game.Game, cs ...string) {
	t.Helper()
	for i := len(cs) - 1; i >= 0; i-- {
		c := card(t, cs[i])
		c.ID = 900 + i
		g.Deck = append(g.Deck, c)
	}
}

func TestL3PlayStartingHandOnesOldestFirst(t *testing.T) {
	// 第一回合 Alice 提示 Bob「1」點到 slot 2、3、4：起手牌的 1 由舊到新打（slot 4 → 3 → 2）
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 r3", "g4 r1 y1 b1 g3", "y4 b3 w3 g3 b4")
	g.Hints = 6
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Bob 應先打最舊的 1（slot 4），卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.Discard(4)) // Alice
	// 打出後新牌插在最左，原本的 slot 3（y1）變成 slot 4
	if a := step(t, g, bots); a != game.Play(3) || g.Fireworks[game.Yellow] != 1 {
		t.Fatalf("Bob 接著應打 y1（現在的 slot 4），卻做了 %+v", a)
	}
	if lv2, bots2 := scenarioAt(t, 2, "", "g4 y4 b4 w4 g3", "g4 r1 y1 b1 g3", "y4 b3 w3 g3 b4"); true {
		force(t, lv2, game.HintRankTo(1, 1))
		if a := step(t, lv2, bots2); a != game.Play(1) {
			t.Fatalf("Level 2 沒有起手牌由舊到新的規則，Bob 應打焦點（最左邊的新牌），卻做了 %+v", a)
		}
	}
}

func TestL3FreshOnesFirst(t *testing.T) {
	// Bob 打出 r1 後抽到新的 1；Alice 再提示「1」點到新牌與兩張起手牌的 1：新抽的 1 先打
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 g3", "g4 y1 b1 r1 g3", "y4 b3 w3 g3 b4")
	drawNext(t, g, "w1")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Bob 應先打最舊的 1（slot 4），卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.White] != 1 {
		t.Fatalf("Fresh 1's Rule：Bob 應先打新抽的 w1（slot 1），卻做了 %+v", a)
	}
}

func TestL3ChopFocusOneFirst(t *testing.T) {
	// r2、b2 已打出。Alice 棄掉 slot 5 的 r4、抽到新的 1；Bob 提示「1」點到 slot 1、2、3、5：
	// 打的順序是 5（chop 焦點）→ 1（新抽的）→ 3 → 2（起手牌由右而左）。四張 1 只需要三張（y1 g1 w1），
	// 依 Good Touch 排最後的 slot 2 永遠不會打出，是已知沒用的牌（挑戰題 Playing Multiple 1's II）。
	g, bots := scenarioAt(t, 3, "r2 b2", "r1 g1 b3 y1 r4", "g4 y4 b4 w4 g3", "y4 b3 w3 g3 b4")
	g.Discards = append(g.Discards, card(t, "r4"))
	g.Hints = 4
	drawNext(t, g, "w1")
	force(t, g, game.Discard(4))
	force(t, g, game.HintRankTo(0, 1))
	force(t, g, game.Discard(4)) // Cathy
	if a := step(t, g, bots); a != game.Play(4) || g.Fireworks[game.Yellow] != 1 {
		t.Fatalf("Alice 應先打 chop 焦點（slot 5 的 y1），卻做了 %+v", a)
	}
	if n := bots[0].s.notes[g.Hands[0][2].Card.ID]; !n.bad {
		t.Errorf("多出來的那張 1（原本 slot 2，現在 slot 3）應記成已知沒用")
	}
	bots[1].Act(g.View(1))
	if n := bots[1].s.notes[g.Hands[0][2].Card.ID]; !n.bad {
		t.Errorf("Bob 也推得出 Alice 多出來的那張 1 沒用")
	}
	force(t, g, game.Discard(4)) // Bob
	force(t, g, game.Discard(4)) // Cathy
	if a := step(t, g, bots); a != game.Play(1) || g.Fireworks[game.White] != 1 {
		t.Fatalf("Alice 接著應打新抽的 w1（現在 slot 2），卻做了 %+v", a)
	}
}

func TestL3FixClueTouchingNewCard(t *testing.T) {
	// r1 y1 b1 w1 已打出。Cathy 提示 Bob「1」：Bob 依 Good Touch 以為是 g1，其實是 b1（垃圾）。
	// Alice 提示藍色修正，同時點到 slot 1 的新藍牌：Fix Clue 不是 play clue，新藍牌不能打；Bob 棄掉 b1。
	g, bots := scenarioAt(t, 3, "r1 y1 b1 w1", "g4 y4 w4 r4 g3", "b3 g4 y4 b1 r3", "y4 b4 w3 g3 b4")
	g.Current = 2
	force(t, g, game.HintRankTo(1, 1))
	force(t, g, game.HintColorTo(1, game.Blue))
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("Fix Clue 不是 play clue，Bob 不該打牌，卻做了 %+v", a)
	}
	if a != game.Discard(3) {
		t.Errorf("Bob 應棄掉被修正為沒用的 b1（slot 4），卻做了 %+v", a)
	}
	if lv2, bots2 := scenarioAt(t, 2, "r1 y1 b1 w1", "g4 y4 w4 r4 g3", "b3 g4 y4 b1 r3", "y4 b4 w3 g3 b4"); true {
		lv2.Current = 2
		force(t, lv2, game.HintRankTo(1, 1))
		force(t, lv2, game.HintColorTo(1, game.Blue))
		if a := step(t, lv2, bots2); a != game.Play(0) {
			t.Fatalf("Level 2 沒有 Fix Clue，藍色會被當成 b2 的 play clue，卻做了 %+v", a)
		}
	}
}

func TestL3GiveFixClueBeforeMisplay(t *testing.T) {
	// 同上：Cathy 的「1」讓 Bob 以為 b1 是 g1，下一手就會打錯。Alice 應用一個提示修正（Misplay Cost Principle）。
	g, bots := scenarioAt(t, 3, "r1 y1 b1 w1", "g4 y4 w4 r4 g3", "b3 g4 y4 b1 r3", "y4 b4 w3 g3 b4")
	g.Current = 2
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a.Type != game.ActHint || a.Target != 1 {
		t.Fatalf("Alice 應給 Bob Fix Clue，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("被修正後 Bob 不該打牌，卻做了 %+v", a)
	}
	if g.Strikes != 0 {
		t.Fatalf("不該有閃電")
	}
}

func TestL3StopClue(t *testing.T) {
	// 挑戰題 Playing Multiple 1's III：第一回合 Alice 提示 Cathy「1」點到 slot 3、4（兩張都是 r1，Alice 的失誤）。
	// Bob 再提示一次「1」（不給任何新資訊的 Fix Clue）：Cathy 原本要打的 slot 4 是壞的，改打 slot 3。
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 g3", "g4 y4 b4 w4 g3", "y4 b3 r1 r1 b4")
	force(t, g, game.HintRankTo(2, 1))
	if a := bots[1].Act(g.View(1)); a.Type != game.ActHint || a.Target != 2 {
		t.Fatalf("Bob 應給 Cathy Fix Clue，卻做了 %+v", a)
	}
	force(t, g, game.HintRankTo(2, 1))
	if a := step(t, g, bots); a != game.Play(2) {
		t.Fatalf("Cathy 應改打 slot 3，卻做了 %+v", a)
	}
	if n := bots[2].s.notes[g.Hands[2][3].Card.ID]; !n.bad {
		t.Errorf("Cathy 應把原本要打的 slot 4 記成已知沒用")
	}
}

func TestL3FixRevealsDuplicatesInSameHand(t *testing.T) {
	// 第一回合 Alice 提示 Cathy「1」點到三張 1，其實全是 r1（Alice 的失誤）。「1」的 Stop Clue 只能修一張；
	// 紅色把三張補成 r1，揭露了重複，一次修好兩張（一起點到的 chop r4 只是「有用的紅牌」，不是 play clue）。
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 g3", "g4 y4 b4 w4 g3", "y4 r1 r1 r1 r4")
	force(t, g, game.HintRankTo(2, 1))
	if a := step(t, g, bots); a != game.HintColorTo(2, game.Red) {
		t.Fatalf("Bob 應給 Cathy 紅色（一次修好兩張重複的 r1），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Cathy 應打出一張 r1，卻做了 %+v", a)
	}
	bad := 0
	for _, hc := range g.Hands[2] {
		n, ok := bots[2].s.note(hc.Card.ID)
		if !ok {
			continue // 剛抽的牌，Cathy 還沒處理到
		}
		if hc.Card.Rank == 1 && bots[2].s.knownTrash(hc.Card.ID) {
			bad++
		}
		if hc.Card.Rank == 4 && bots[2].s.knownPlayable(hc.Card.ID) {
			t.Errorf("一起點到的 r4 不是 play clue 的焦點，不能當成能打（筆記 %v）", n.poss.list())
		}
	}
	if bad != 2 {
		t.Errorf("剩下兩張 r1 都應是已知沒用的牌，實際 %d 張", bad)
	}
}

func TestL3SarcasticDiscard(t *testing.T) {
	// b2 已打出，g3 棄掉一張（critical）。Bob 提示 Alice「3」救 chop（Alice 只知道是 g3 或 b3，其實是 b3）。
	// Alice 看不到自己的牌，又提示 Cathy 藍色點到 b3（重複）。Cathy 確定自己是 b3、又看到 Alice 的 b3：
	// 她是先發現重複的人，Sarcastic Discard；Alice 因此知道自己的 3 是 b3 而打出。
	g, bots := scenarioAt(t, 3, "b2", "g4 y4 w4 r4 b3", "g4 y4 b4 w4 r3", "y4 b3 w4 r4 y4")
	g.Discards = append(g.Discards, card(t, "g3"))
	g.Hints = 4
	g.Current = 1
	force(t, g, game.HintRankTo(0, 3))
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.HintColorTo(2, game.Blue))
	force(t, g, game.Discard(4)) // Bob
	if a := step(t, g, bots); a != game.Discard(2) {
		t.Fatalf("Cathy 應 Sarcastic Discard 她的 b3（slot 3），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a != game.Play(4) || g.Fireworks[game.Blue] != 3 {
		t.Fatalf("Alice 應知道自己的 3 是 b3 而打出（slot 5），卻做了 %+v", a)
	}
}

func TestL3NoStopClueOnCertainlyGoodCard(t *testing.T) {
	// w4 已打出。Bob 的 chop 被救成 5，Cathy 再提示白色：Bob 確定它是 w5、可以打。輪到 Bob 前 Alice 又提示「5」
	// （沒有任何新資訊，例如終局拖延）。w5 只有一張、又還沒打出，不可能是壞牌，Fix Clue 沒東西可修，
	// 所以這不是 Stop Clue：Bob 照樣打出 w5。
	g, bots := scenarioAt(t, 3, "w4", "g4 y4 b4 r4 g3", "g4 y4 b4 r4 w5", "y4 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 5))
	force(t, g, game.Discard(0)) // Bob
	force(t, g, game.HintColorTo(1, game.White))
	force(t, g, game.HintRankTo(1, 5))
	if a := step(t, g, bots); a != game.Play(4) || g.Fireworks[game.White] != 5 {
		t.Fatalf("Bob 應打出確定的 w5（slot 5），卻做了 %+v", a)
	}
}

func TestL3FixClueMayTouchTrashNewCard(t *testing.T) {
	// 第一回合 Alice 提示 Bob「1」點到兩張 r1（Alice 的失誤）。Bob 打出最舊的那張、又抽到一張 r1。
	// 他剩下那張被提示的 1 下一手就會打錯：Cathy 用紅色修正，雖然也會點到新抽的 r1（違反 Good Touch，
	// 但一個提示換一次閃電是值得的，Misplay Cost Principle）。
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 g3", "g4 b4 r1 r1 y4", "y4 b3 w3 g3 b4")
	drawNext(t, g, "r1")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Bob 應先打最舊的 1（slot 4），卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type != game.ActHint || a.Target != 1 {
		t.Fatalf("Cathy 應給 Bob Fix Clue，卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Alice
	step(t, g, bots)
	if g.Strikes != 0 {
		t.Fatalf("Bob 不該打錯")
	}
}

func TestPromptDoesNotDisambiguateChopSave(t *testing.T) {
	// w1 已打出，w4 棄掉一張（critical）。Cathy 的 chop 被「2」救下（她不知道是 w2）。Bob 的 chop 是 w3：
	// 白色點到它可能是 w3（prompt Cathy 的 2）也可能是救 critical 的 w4。save clue 不會觸發 prompt，
	// Cathy 打出 w2 也證明不了什麼，Bob 永遠分不出來——這不能當成 play clue。
	g, bots := scenarioAt(t, 3, "w1", "g4 y4 b4 r4 g3", "g4 y4 b4 w3 r4", "y3 b3 g3 r3 w2")
	g.Discards = append(g.Discards, card(t, "w4"))
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 2))
	force(t, g, game.Discard(4)) // Bob
	force(t, g, game.Discard(0)) // Cathy
	bots[0].Act(g.View(0))
	ev, _ := bots[0].s.evalClue(1, game.Hint{Kind: game.HintColor, Color: game.White}, 1)
	if ev.kind == kindPlay {
		t.Fatalf("白色可能是救 w4，不能當成 play clue")
	}
}

func TestL3InformationLockOnMyInBetweenCard(t *testing.T) {
	// 4 人：r4 b3 w3 已打出。Bob 確定有 r5，另一張「4」只知道是 b4 或 w4（兩種都能打）。Alice 提示 Cathy 藍色點到 b5：
	// b5 要靠 b4，別人手上都沒有，所以 Bob 的 4 鎖定成 b4（Information Lock on In-Between Cards）。
	// 能接上別人已被提示的牌優先，Bob 先打 b4（不是 r5），Cathy 就能接著打 b5。
	g, bots := scenarioAt(t, 3, "r4 b3 w3", "g4 y4 r3 g3", "g1 r5 y2 b4", "b5 g3 y3 g4", "y4 g2 r2 w2")
	g.Current = 2
	force(t, g, game.HintColorTo(1, game.Red))
	force(t, g, game.HintRankTo(1, 4))
	force(t, g, game.HintColorTo(2, game.Blue))
	if a := step(t, g, bots); a != game.Play(3) || g.Fireworks[game.Blue] != 4 {
		t.Fatalf("Bob 應先打鎖定成 b4 的牌（slot 4），卻做了 %+v", a)
	}
	if lv2, bots2 := scenarioAt(t, 2, "r4 b3 w3", "g4 y4 r3 g3", "g1 r5 y2 b4", "b5 g3 y3 g4", "y4 g2 r2 w2"); true {
		lv2.Current = 2
		force(t, lv2, game.HintColorTo(1, game.Red))
		force(t, lv2, game.HintRankTo(1, 4))
		force(t, lv2, game.HintColorTo(2, game.Blue))
		if a := step(t, lv2, bots2); a != game.Play(1) {
			t.Fatalf("Level 2 沒有 Information Lock，Bob 照一般順序先打 r5，卻做了 %+v", a)
		}
	}
}

func TestL3FreshOneBeforeKnownStartingOne(t *testing.T) {
	// Bob 起手牌的 w1、b1、y1 被提示「1」，先打 chop 上的 y1，抽到新的 r1。Cathy 提示藍色點到 b1、b2（b1 因此確定）。
	// Alice 再提示「1」點到新的 r1 和 w1、b1：新抽的 1 先打（Fresh 1's Rule）。b1 能接上自己手上的 b2
	// （Level 25 的表）也不能打亂 Level 3 的 1 的順序。
	g, bots := scenarioAt(t, 3, "", "g4 y4 b4 w4 g3", "w1 b1 r3 b2 y1", "y4 b3 w3 g3 b4")
	drawNext(t, g, "r1")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應先打 chop 上的 y1，卻做了 %+v", a)
	}
	force(t, g, game.HintColorTo(1, game.Blue))
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(0) || g.Fireworks[game.Red] != 1 {
		t.Fatalf("Bob 應先打新抽的 r1（slot 1），卻做了 %+v", a)
	}
}

// misplayAfterPlayScenario：Bob 為了救 chop 上唯一看得到的 g1，綠色點到 Alice 的 g2、g2、g1（Save Principle 允許違反 Good Touch）。
// Alice 打出 g1 與其中一張 g2 後，另一張 g2 依 Good Touch 被她當成 g3/g4（g5 在 Bob 手上看得到）。
// 輪到 Cathy，她手上被 prompt 的 g3 可以打——但打出後 Alice 就會把那張 g2 當成 g4 而打錯。
func misplayAfterPlayScenario(t *testing.T) (*game.Game, []*Bot) {
	t.Helper()
	g, bots := scenarioAt(t, 3, "", "y4 g2 w4 g2 g1", "g5 y3 b3 w3 r3", "w2 g3 r4 y4 b4")
	g.Hints = 2
	drawNext(t, g, "r4", "y4", "b4", "r4", "y4", "w4")
	force(t, g, game.Discard(0))                 // Alice
	force(t, g, game.HintColorTo(0, game.Green)) // Bob：救 Alice chop 上的 g1
	force(t, g, game.Discard(4))                 // Cathy
	force(t, g, game.Play(4))                    // Alice：g1
	force(t, g, game.HintColorTo(2, game.Green)) // Bob：Cathy 的 g3（prompt Alice 的 g2）
	force(t, g, game.Discard(4))                 // Cathy
	force(t, g, game.Play(2))                    // Alice：g2
	force(t, g, game.Discard(4))                 // Bob
	if g.Fireworks[game.Green] != 2 {
		t.Fatal("情境錯誤：g1、g2 應已打出")
	}
	return g, bots
}

func TestL3FixBeforeMyPlayMakesNextMisplay(t *testing.T) {
	// Misplay Cost Principle：Cathy 先給 Fix Clue（「2」點到 Alice 那張 g2），g3 晚點打
	g, bots := misplayAfterPlayScenario(t)
	if a := step(t, g, bots); a != game.HintRankTo(0, 2) {
		t.Fatalf("Cathy 應先給 Alice「2」修正那張 g2，卻做了 %+v", a)
	}
}

func TestL3HoldPlayThatMakesNextMisplayWithoutClues(t *testing.T) {
	// 沒有提示可修時，Cathy 先不打 g3（打出後 Alice 下一手就會打錯），棄牌換回提示讓 Bob 修
	g, bots := misplayAfterPlayScenario(t)
	g.Hints = 0
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("Cathy 不該打出 g3（Alice 會把 g2 當成 g4 打錯），卻做了 %+v", a)
	}
}

func TestDiscardKnownTrashBeforeFiveStall(t *testing.T) {
	// Early Game：Bob 手上有已知垃圾（Fix Clue 揭露的重複 r1）。棄已知垃圾不會結束 Early Game
	// （Level 1：Early Game 是有人棄掉 chop 之前；Level 9：Discarding known-trash does not count），
	// 還換回一個提示，所以先棄它，不必 5 Stall（Cathy 的 g5）
	g, bots := scenarioAt(t, 3, "", "y3 r5 b3 w4 y4", "g3 b4 w3 r1 r1", "b4 g5 w3 y3 g4")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應先打最舊的 1，卻做了 %+v", a)
	}
	force(t, g, game.HintRankTo(0, 5))         // Cathy：5 Stall Alice
	force(t, g, game.HintColorTo(1, game.Red)) // Alice：Fix Clue，Bob 剩下的 1 也是 r1
	if a := step(t, g, bots); a != game.Discard(4) {
		t.Fatalf("Bob 應棄掉已知垃圾的 r1（slot 5），卻做了 %+v", a)
	}
	bots[2].Act(g.View(2))
	if bots[2].s.midGame {
		t.Error("棄掉已知垃圾（已被提示的牌）不會結束 Early Game")
	}
}

func TestClueOnesWhenDuplicateIsDeducible(t *testing.T) {
	// 除了 w1 以外的 1 都打出了，Bob 手上有兩張 w1：「1」點到兩張時，Bob 推得出兩張都只可能是 w1，
	// 照 Playing Multiple 1's 打掉一張，另一張就是已知垃圾（不會誤解，和點到推得出是垃圾的牌一樣不算違反 Good Touch）
	g, bots := scenarioAt(t, 3, "r1 y1 g1 b1", "r3 y3 g3 b3 w3", "w1 g3 w1 b4 y4", "r3 y3 g4 b3 w3")
	g.Hints = 5 // 不是提示滿 8 的拖延
	if a := step(t, g, bots); a != game.HintRankTo(1, 1) {
		t.Fatalf("Alice 應給 Bob「1」，卻做了 %+v", a)
	}
	if a := step(t, g, bots); a.Type != game.ActPlay || g.Fireworks[game.White] != 1 {
		t.Fatalf("Bob 應打出 w1，卻做了 %+v", a)
	}
	if n := bots[1].s.notes[g.Hands[1][1].Card.ID]; !n.bad && !bots[1].s.knownTrash(g.Hands[1][1].Card.ID) {
		t.Error("Bob 剩下的那張 1 應是已知垃圾")
	}
}

func TestL3NoFixWhenHolderSeesTheCopy(t *testing.T) {
	// 使用者回報 20261015-100922 t51：r5 y5 b5 已打出、w4 g4 也是。Cathy 兩張被「5」點到的牌都是「g5 或 w5」——
	// 兩張、兩種、各只剩 1 張，所以一張是 g5、一張是 w5。Bob 被白色點到的牌共同筆記只剩 w5（其實是 w2），
	// 但他看得到 Cathy 的 w5，自己就知道那張是垃圾、不會打：不必 Fix Clue，Cathy 打出自己的 5
	g, bots := scenarioAt(t, 5, "r5 y5 b5 g4 w4", "r1 y1 b1 g1 r2", "w2 r3 y3 b3 g3", "g5 w5 r4 y4 b4")
	g.Hints = 5
	force(t, g, game.HintRankTo(2, 5))           // Alice：Cathy 的 g5、w5
	force(t, g, game.Discard(4))                 // Bob
	force(t, g, game.Discard(4))                 // Cathy
	force(t, g, game.HintColorTo(1, game.White)) // Alice：Bob 的 w2
	force(t, g, game.Discard(4))                 // Bob
	a := step(t, g, bots)
	if a.Type == game.ActHint && a.Target == 1 {
		t.Fatalf("Bob 看得到 Cathy 的 w5，知道自己的白牌是垃圾，Cathy 不必修，卻做了 %+v（%s）", a, bots[2].Why())
	}
	if a.Type != game.ActPlay {
		t.Fatalf("Cathy 的兩張 5 都能打，應打出其中一張，卻做了 %+v（%s）", a, bots[2].Why())
	}
}

// replayCodes 照紀錄的種子與行動代碼重現局面（座位 0 是真人；AI 每回合都先行動過，狀態和實際對局一致）。
func replayCodes(t *testing.T, n int, seed uint64, level int, codes string) (*game.Game, []*Bot) {
	t.Helper()
	return replayCodesVariant(t, n, game.NoVariant, seed, level, codes)
}

func replayCodesVariant(t *testing.T, n int, va game.Variant, seed uint64, level int, codes string) (*game.Game, []*Bot) {
	t.Helper()
	g, err := game.NewVariant(n, va, seed)
	if err != nil {
		t.Fatal(err)
	}
	bots := make([]*Bot, n)
	for i := range bots {
		bots[i] = New(fmt.Sprint(i), level)
	}
	for _, code := range strings.Fields(codes) {
		if p := g.Current; p != 0 {
			bots[p].Act(g.View(p))
		}
		a, err := parseCode(code)
		if err != nil {
			t.Fatal(err)
		}
		force(t, g, a)
	}
	return g, bots
}

const fixedDupG2 = "h15 h31 h3B p3 h25 h03 h12 p4 p4 h24 p4 h22 d4 d1 h02 h0G"

func TestL3FocusDiscardedCancelsFinesse(t *testing.T) {
	// 使用者回報 20261015-173634：t15「2」救使用者 chop 上的 y2，也點到 slot 1 的 g2——AI-1 手上已提示的 2 就是 g2，重複了；
	// t16 AI-3 用綠色修正（Fix Clue），t17 使用者知道 g2 是重複的而棄掉（Sarcastic Discard）。
	// AI-1 不能把綠色當成要他 Reverse Finesse g1 的 tempo clue：焦點被棄掉，那個 play 解讀就不成立
	g, bots := replayCodes(t, 4, 1792056994110306000, 5, fixedDupG2+" d1")
	if a := step(t, g, bots); g.Strikes != 0 {
		t.Fatalf("AI-1 不該照「g1 被 finesse」盲打，卻做了 %+v（%s）", a, bots[1].Why())
	}
}

func TestL3AssumeOwnCardsGoodUntilShownOtherwise(t *testing.T) {
	// 同上，但使用者 t17 棄的是別張、g2 還留著：AI-1 預設自己的牌都是好的（Level 3 Fix Clue：「we assume by default
	// that all of our cards are good unless it is explicitly demonstrated otherwise」），不認為自己已提示的 2 和使用者的
	// g2 重複，綠色就是要使用者打 g2 的 play clue、g1 由他 Reverse Finesse。他不盲打的話，使用者會以為是 self-finesse
	g, bots := replayCodes(t, 4, 1792056994110306000, 5, fixedDupG2+" d2")
	fp := g.Hands[1][0].Card.ID
	bots[1].Act(g.View(1))
	if !bots[1].s.notes[fp].finessed {
		t.Fatalf("AI-1 應照 play clue 的讀法把 finesse position 記成被 finesse 的 g1（%s）", bots[1].Why())
	}
}
