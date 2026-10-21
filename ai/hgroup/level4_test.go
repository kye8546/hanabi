package hgroup

import (
	"testing"

	"hanabi/game"
)

// Level 4（Chop Moves）的範例情境，取自 H-Group 原文 level-4.mdx 與 Level 4 挑戰題。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1）到右（chop）。

// cmOf 回傳 bot 是否把座位 p 第 slot 張牌記成 Chop Moved。
func cmOf(b *Bot, g *game.Game, p, slot int) bool {
	return b.s.notes[g.Hands[p][slot].Card.ID].cm
}

const allOnes = "r1 y1 g1 b1 w1"

func TestL4TrashChopMove(t *testing.T) {
	// 1 全打出了，Alice 提示 Bob「1」只點到 slot 4：Trash Chop Move，Bob 的 slot 5 被 Chop Move，他棄掉那張 1
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w3 g3", "r3 y4 g4 b1 w4", "y3 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 1))
	cmID := g.Hands[1][4].Card.ID
	if a := step(t, g, bots); a != game.Discard(3) {
		t.Fatalf("Bob 應棄掉已知是垃圾的 1（slot 4），卻做了 %+v", a)
	}
	if !bots[1].s.notes[cmID].cm {
		t.Error("Bob 的 slot 5 應被 Chop Move")
	}
	bots[2].Act(g.View(2))
	if !bots[2].s.notes[cmID].cm {
		t.Error("Cathy 也應把 Bob 原本的 slot 5 記成 Chop Moved")
	}
	if c := bots[2].s.chop(1); c != g.Hands[1][3].Card.ID {
		t.Errorf("Bob 的新 chop 應是 slot 4（原本的 slot 3），卻是 %d", c)
	}
	if lv3, bots3 := scenarioAt(t, 3, allOnes, "y4 g4 r4 w3 g3", "r3 y4 g4 b1 w4", "y3 b3 w3 g3 b4"); true {
		force(t, lv3, game.HintRankTo(1, 1))
		step(t, lv3, bots3)
		if bots3[1].s.notes[cmID].cm {
			t.Error("Level 3 沒有 Chop Move")
		}
	}
}

func TestL4TrashChopMoveMultiple(t *testing.T) {
	// 「1」點到 slot 1：slot 2–5 全被 Chop Move（四張）
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w3 g3", "b1 r3 y4 g4 w4", "y3 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 1))
	bots[2].Act(g.View(2))
	for slot := 1; slot <= 4; slot++ {
		if !cmOf(bots[2], g, 1, slot) {
			t.Errorf("Bob 的 slot %d 應被 Chop Move", slot+1)
		}
	}
	if a := step(t, g, bots); a != game.Discard(0) {
		t.Fatalf("Bob 應棄掉那張 1（slot 1），卻做了 %+v", a)
	}
}

func TestL4TrashChopMoveOnlyRightOfRightmostTrash(t *testing.T) {
	// 「1」點到 slot 2 與 slot 4：只有最右邊那張垃圾右邊的 slot 5 被 Chop Move
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w3 g3", "r3 b1 y4 g1 w4", "y3 b3 w3 g3 b4")
	force(t, g, game.HintRankTo(1, 1))
	bots[2].Act(g.View(2))
	if !cmOf(bots[2], g, 1, 4) || cmOf(bots[2], g, 1, 2) {
		t.Errorf("只有 slot 5 應被 Chop Move（slot 3：%v、slot 5：%v）", cmOf(bots[2], g, 1, 2), cmOf(bots[2], g, 1, 4))
	}
}

func TestL4AWastedClue(t *testing.T) {
	// 挑戰題 A Wasted Clue?：3 全打出了，「3」點到 Bob 的 slot 3：slot 3 是垃圾，slot 4、5 被 Chop Move
	g, bots := scenarioAt(t, 4, "r3 y3 g3 b3 w3", "y4 g4 r4 w4 g5", "r4 y4 b3 g4 w4", "y4 b4 w4 g4 b4")
	force(t, g, game.HintRankTo(1, 3))
	bots[2].Act(g.View(2))
	if !cmOf(bots[2], g, 1, 3) || !cmOf(bots[2], g, 1, 4) || cmOf(bots[2], g, 1, 1) {
		t.Error("Bob 的 slot 4、5 應被 Chop Move，slot 2 不是")
	}
	if !bots[2].s.notes[g.Hands[1][2].Card.ID].bad {
		t.Error("Bob 的 slot 3 應記成已知垃圾")
	}
}

// midGame 讓三人各棄掉 slot 5（結束 Early Game），之後每人的 slot 1 是新抽的 g4、其餘右移一格。
func midGame(t *testing.T, g *game.Game) {
	t.Helper()
	g.Hints = 2 // 棄完是 5 個（提示滿 8 時「5」是拖延，不是 5's Chop Move）
	for range 3 {
		force(t, g, game.Discard(4))
	}
}

func TestL4FivesChopMove(t *testing.T) {
	// 不是 Early Game：「5」點到 one-away from chop 的 5 → Chop Move chop（slot 5），新 chop 是 slot 3
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 b3", "r3 y4 r5 w4 y3", "y3 b3 w3 g3 b4")
	midGame(t, g)
	force(t, g, game.HintRankTo(1, 5))
	a := step(t, g, bots)
	if a.Type == game.ActPlay {
		t.Fatalf("5's Chop Move 不是 play clue，Bob 不該打牌，卻做了 %+v", a)
	}
	if !bots[1].s.notes[g.Hands[1][4].Card.ID].cm && !(a.Type == game.ActDiscard && a.Slot == 4) {
		t.Error("Bob 的 slot 5 應被 Chop Move")
	}
	if a.Type == game.ActDiscard && a.Slot != 2 {
		t.Errorf("Bob 棄牌時應棄新的 chop（slot 3），卻棄了 slot %d", a.Slot+1)
	}
}

func TestL4FivesChopMoveMultipleFives(t *testing.T) {
	// 「5」點到 slot 2 與 slot 4 的 5：只 Chop Move slot 5
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 b3", "y5 y4 r5 w4 y3", "y3 b3 w3 g3 b4")
	midGame(t, g)
	force(t, g, game.HintRankTo(1, 5))
	bots[2].Act(g.View(2))
	if !cmOf(bots[2], g, 1, 4) || cmOf(bots[2], g, 1, 2) {
		t.Error("只有 Bob 的 slot 5 應被 Chop Move")
	}
}

func TestL4FivesChopMoveNotInEarlyGame(t *testing.T) {
	// 挑戰題 5's Chop Move：第一回合「5」點到 Cathy slot 4 的 5 是 5 Stall（優先於 5's Chop Move），不 Chop Move
	g, bots := scenarioAt(t, 4, "", "y4 g4 r4 w4 g3", "y3 b3 w3 g3 b4", "g3 r3 y4 r5 w4")
	force(t, g, game.HintRankTo(2, 5))
	bots[1].Act(g.View(1))
	if cmOf(bots[1], g, 2, 4) {
		t.Error("Early Game 的 5 Stall 不 Chop Move")
	}
}

func TestL4FiveTwoAwayIsNotChopMove(t *testing.T) {
	// 離 chop 兩張的 5 不是 5's Chop Move（只能 Chop Move 一張）
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 b3", "r5 y4 g4 w4 y3", "y3 b3 w3 g3 b4")
	midGame(t, g)
	force(t, g, game.HintRankTo(1, 5))
	bots[2].Act(g.View(2))
	if cmOf(bots[2], g, 1, 4) {
		t.Error("離 chop 兩張的 5 不應 Chop Move")
	}
}

func TestL4OrderChopMove(t *testing.T) {
	// 第一回合 Alice 提示 Bob「1」點到 slot 1、2、3（都是起手牌，應先打 slot 3）
	for _, c := range []struct {
		slot, target int
	}{{2, -1}, {1, 2}, {0, 0}} {
		g, bots := scenarioAt(t, 4, "", "y4 g4 r4 w4 g3", "r1 y1 g1 b4 w4", "y3 b3 w3 g3 b4")
		force(t, g, game.HintRankTo(1, 1))
		force(t, g, game.Play(c.slot))
		for p := range 3 {
			if p == 1 {
				continue
			}
			obs := (p + 1) % 3 // 由另一位旁觀者的筆記檢查
			if p == 0 {
				obs = 2
			}
			bots[obs].Act(g.View(obs))
			if got := cmOf(bots[obs], g, p, 4); got != (p == c.target) {
				t.Errorf("Bob 打 slot %d：座位 %d 的 slot 5 Chop Moved = %v，應為 %v", c.slot+1, p, got, p == c.target)
			}
		}
	}
}

func TestL4OrderIsImportant(t *testing.T) {
	// 挑戰題 Order Is Important：Bob 的 slot 2 是新抽的 1、slot 3 是起手牌的 1，Fresh 1's 應先打 slot 2；
	// Bob 打了 slot 3 → Order Chop Move Cathy，她的 slot 3 被 Chop Move，下一次棄 slot 2
	g, bots := scenarioAt(t, 4, "r1 b2", "y4 g4 r4 w4 g3", "g1 b3 y4 b4 r4", "g4 y4 w4 y2 g2")
	drawNext(t, g, "w1", "r4", "y3")
	force(t, g, game.HintRankTo(2, 2)) // Alice：2 Save Cathy 的兩張 2
	force(t, g, game.Play(1))          // Bob：b3
	force(t, g, game.HintRankTo(0, 4)) // Cathy
	force(t, g, game.Discard(4))       // Alice
	force(t, g, game.Play(3))          // Bob：b4
	force(t, g, game.HintRankTo(0, 4)) // Cathy
	force(t, g, game.HintRankTo(1, 1)) // Alice：「1」點到 Bob 的 slot 2（新抽的 w1）與 slot 3（起手牌的 g1）
	force(t, g, game.Play(2))          // Bob：打 slot 3 的 g1
	if g.Fireworks[game.Green] != 1 {
		t.Fatal("情境錯誤：Bob 的 slot 3 應是 g1")
	}
	bots[2].Act(g.View(2))
	if !cmOf(bots[2], g, 2, 2) {
		t.Error("Cathy 的 slot 3 應被 Chop Move")
	}
	if c := bots[2].s.chop(2); c != g.Hands[2][1].Card.ID {
		t.Errorf("Cathy 的 chop 應是 slot 2，卻是 %d", c)
	}
}

func TestL4FocusWhenChopMoved(t *testing.T) {
	// 挑戰題 Focus When Chop Moved：Alice 的「1」對 Cathy 是 Trash Chop Move（slot 4、5 被 Chop Move），
	// Bob 的藍色點到 Cathy 全部的牌：焦點是 chop（slot 2），是 b2 的 play clue
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 g3", "y3 r3 w3 g3 r4", "b3 b2 b1 b4 b5")
	force(t, g, game.HintRankTo(2, 1))
	force(t, g, game.HintColorTo(2, game.Blue))
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Cathy 應打出 chop 上的 b2（slot 2），卻做了 %+v", a)
	}
}

func TestL4NewCardFocusOverChopMoved(t *testing.T) {
	// Chop Moves & New Clues：Trash Chop Move 後 Bob 棄掉垃圾、抽到 b2；Cathy 的藍色點到新牌（slot 1）與
	// 被 Chop Move 的 slot 5：焦點是新牌，Bob 打 slot 1
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 g3", "r3 y4 g4 r1 b4", "y3 w3 g3 r3 w4")
	drawNext(t, g, "b2")
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Discard(3) {
		t.Fatalf("Bob 應先棄掉垃圾 1，卻做了 %+v", a)
	}
	force(t, g, game.HintColorTo(1, game.Blue))
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 應打出新牌 b2（slot 1），卻做了 %+v", a)
	}
}

func TestL4OnlyOldAndChopMovedFocusLeftmostChopMoved(t *testing.T) {
	// 只點到舊牌與第一次被點到的 Chop Moved 牌：焦點是最左邊那張 Chop Moved 的牌（slot 4 的 b2），不是 chop 方向的 slot 5
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 g3", "y3 r3 w3 g3 r4", "r3 y4 b1 b2 b4")
	force(t, g, game.HintRankTo(2, 1))          // Alice：Trash Chop Move，slot 4、5 被 Chop Move
	force(t, g, game.HintColorTo(2, game.Blue)) // Bob：點到 b1（舊）、b2、b4（Chop Moved）
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("Cathy 應打出 slot 4 的 b2，卻做了 %+v", a)
	}
}

func TestL4ChopMovedCardIsNotPrompted(t *testing.T) {
	// Chop Moves & Prompts：Bob 的 slot 5 被 Chop Move（沒被提示），Alice 提示 Cathy 紅色點到 r3：
	// Chop Moved 的牌不會被 prompt，Bob 盲打 finesse position（slot 1 的 r2）
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 b4 w4 g3", "y4 g4 w4 b1 r4", "r3 y3 g3 b3 w3")
	drawNext(t, g, "r2")
	force(t, g, game.HintRankTo(1, 1)) // Alice：Trash Chop Move
	force(t, g, game.Discard(3))       // Bob 棄掉 b1，抽到 r2
	force(t, g, game.HintRankTo(0, 3)) // Cathy
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("Bob 應盲打 slot 1（finesse），不是 Chop Moved 的 slot 5，卻做了 %+v", a)
	}
}

func TestL4UndoChopMoveAfterCriticalDiscard(t *testing.T) {
	// 5's Chop Move 後 Bob 下一次棄牌棄掉 critical（y2，另一張已棄）：那次 Chop Move 是誤會，撤銷
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 r4 w4 y2", "b4 y2 r5 w4 y3", "y3 b3 w3 g3 b4")
	midGame(t, g) // Alice 棄掉 y2
	force(t, g, game.HintRankTo(1, 5))
	cmID := g.Hands[1][4].Card.ID
	bots[2].Act(g.View(2))
	if !bots[2].s.notes[cmID].cm {
		t.Fatal("情境錯誤：Bob 的 slot 5 應先被 Chop Move")
	}
	force(t, g, game.Discard(2)) // Bob 棄掉 critical 的 y2（slot 3）
	bots[2].Act(g.View(2))
	if bots[2].s.notes[cmID].cm {
		t.Error("棄掉 critical 後應撤銷 Chop Move")
	}
}

func TestL4GiveFivesChopMove(t *testing.T) {
	// Bob 的 chop 是 critical 的 r4，chop 左邊一張是 g5：Alice 用「5」同時救 g5 並 Chop Move r4
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 b4 w4 b3", "b3 w3 g5 r4 y3", "y3 b3 w3 g3 w4")
	g.Discards = append(g.Discards, card(t, "r4"))
	midGame(t, g)
	if a := step(t, g, bots); a != game.HintRankTo(1, 5) {
		t.Fatalf("Alice 應給 Bob「5」（5's Chop Move），卻做了 %+v", a)
	}
	bots[2].Act(g.View(2))
	if !cmOf(bots[2], g, 1, 4) {
		t.Error("Bob 的 r4 應被 Chop Move")
	}
}

func TestL4GiveTrashChopMoveForTwoCriticals(t *testing.T) {
	// Bob 的 chop 連著兩張 critical（slot 4 的 y4、slot 5 的 r4），slot 3 是垃圾 b1：
	// 一個 save clue 只能救一張，Trash Chop Move 一次保護兩張
	g, bots := scenarioAt(t, 4, allOnes, "y3 g4 b4 w4 b3", "g3 b1 y4 r4 y3", "y3 b3 w3 g3 w4")
	g.Discards = append(g.Discards, card(t, "r4"), card(t, "y4"))
	midGame(t, g)
	if a := step(t, g, bots); a != game.HintRankTo(1, 1) {
		t.Fatalf("Alice 應給 Bob「1」（Trash Chop Move 兩張 critical），卻做了 %+v", a)
	}
}

func TestL4GiveOrderChopMove(t *testing.T) {
	// Alice 有兩張起手牌的 1（slot 2、3，照順序先打 slot 3）；下一位 Bob 的 chop 是 critical 的 r4、沒牌可打：
	// 改打 slot 2（跳過一張）= Order Chop Move Bob，不花提示就救下 r4
	g, bots := scenarioAt(t, 4, "", "g4 r1 y1 b4 w4", "y3 b3 w3 g3 r4", "y3 b3 w3 g3 b4")
	g.Discards = append(g.Discards, card(t, "r4"))
	g.Hints = 6
	force(t, g, game.HintRankTo(2, 3)) // Alice
	force(t, g, game.HintRankTo(0, 1)) // Bob：「1」點到 Alice 的 slot 2、3
	force(t, g, game.Discard(4))       // Cathy
	if a := step(t, g, bots); a != game.Play(1) {
		t.Fatalf("Alice 應打 slot 2 的 1（Order Chop Move Bob），卻做了 %+v", a)
	}
	bots[1].Act(g.View(1))
	if !cmOf(bots[1], g, 1, 4) {
		t.Error("Bob 應把自己的 r4 記成 Chop Moved")
	}
}

func TestL4FinessedOneIsNotOrderChopMove(t *testing.T) {
	// 4 人局：Alice 給 Donald 紅色點到 r2，finesse Cathy slot 1 的 r1；Bob 接著給 Cathy「1」點到 slot 1、2。
	// Cathy 打 slot 1（被 finesse 的那張）不是 Order Chop Move。Donald 是紅色的接收者，看不到自己的牌
	// （他那張也可能直接是 r1），沒把 Cathy 那張記成被 finesse，但它可能是之前的提示 finesse 的牌，一樣不算
	g, bots := scenarioAt(t, 4, "", "g4 y4 b4 w4", "g3 y3 b3 w3", "r1 y1 g4 b4", "r2 g3 w4 b3")
	force(t, g, game.HintColorTo(3, game.Red))
	force(t, g, game.HintRankTo(2, 1))
	force(t, g, game.Play(0))
	bots[3].Act(g.View(3))
	if cmOf(bots[3], g, 3, 3) {
		t.Error("Cathy 打出被 finesse 的 1 不是 Order Chop Move，Donald 不該 Chop Move 自己的 chop")
	}
}

func TestL4TrashChopMoveOnlyOnCommonKnowledge(t *testing.T) {
	// Bob 的 w4 只被提示過白色（大家只知道是白色）。Alice 給 Cathy「4」點到另一張 w4：其他 4 都打出了，
	// Cathy 看得到 Bob 那張就是 w4，私下推得出自己這張是重複的垃圾；但 Bob 看不到自己的牌，會當成 w4 的 play clue。
	// Trash Chop Move 只能靠大家都知道的資訊，所以不是 Trash Chop Move
	g, bots := scenarioAt(t, 4, "r4 y4 g4 b4 w3", "g2 y2 b2 r2 g3", "y3 w4 b3 r3 g3", "y2 r2 w4 b3 g2")
	g.Hints = 6
	force(t, g, game.HintColorTo(1, game.White))
	force(t, g, game.Discard(4)) // Bob
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.HintRankTo(2, 4))
	for _, p := range []int{1, 2} {
		bots[p].Act(g.View(p))
		if cmOf(bots[p], g, 2, 3) || cmOf(bots[p], g, 2, 4) {
			t.Errorf("座位 %d 把 Cathy 的牌記成 Chop Moved，但這不是大家都看得出的 Trash Chop Move", p)
		}
	}
}

func TestL4GiveFivesChopMoveEvenIfItLocks(t *testing.T) {
	// Bob 的 chop 是 critical 的 r4，左邊四張都是 5：「5」是 5's Chop Move，之後他鎖手又沒牌可打。
	// 但直接救 r4 的話，5 就成了 chop，還得一張張再花提示救——保護的牌本來就都必須保留，鎖手是遲早的事
	g, bots := scenarioAt(t, 4, allOnes, "y4 g4 b4 w4 b3", "b5 w5 g5 r4 y3", "y3 b3 w3 g3 w4")
	g.Discards = append(g.Discards, card(t, "r4"))
	g.Deck[len(g.Deck)-2].Color = game.Yellow // midGame 時 Bob 抽到的是 y5
	g.Deck[len(g.Deck)-2].Rank = 5
	midGame(t, g)
	if a := step(t, g, bots); a != game.HintRankTo(1, 5) {
		t.Fatalf("Alice 應給 Bob「5」（5's Chop Move，一次救四張 5 和 r4），卻做了 %+v", a)
	}
}

func TestL4NoTrashChopMoveWhenDirectSaveWorks(t *testing.T) {
	// 使用者回報 20261021-094401 t29：AI-2 chop 上是 critical 的 g4，「4」能直接救它（也點到已知垃圾的 r4）。
	// 原文：能直接用 save clue 救的牌不能 Trash Chop Move（有正向資訊總是比較好）。以前 AI-1 的筆記把那張 r4 記成 r1，
	// 「4」讓筆記矛盾就被當成誤導，改給了「1」的 Trash Chop Move
	g, bots := replayCodes(t, 3, 1792547041335921000, 6, "h11 p2 h0B p5 p2 h0Y p1 h2Y h0B p3 h22 p4 p1 h23 p3 p2 h25 p4 d5 h2R p3 d5 d5 h11 h2W p1 p2 d5")
	if a := bots[1].Act(g.View(1)); a != game.HintRankTo(2, 4) {
		t.Errorf("AI-1 應直接給 AI-2「4」救 g4，卻做了 %+v（%s）", a, bots[1].Why())
	}
}
