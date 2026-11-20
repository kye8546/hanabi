package hgroup

import (
	"testing"

	"hanabi/game"
)

// Black (6 Suits) 變體的慣例（https://hanabi.github.io/variant-specific/black）。
// 座位 0 = Alice、1 = Bob、2 = Cathy；手牌字串由左（slot 1，最新）到右（chop 方向）。

func blackScenario(t *testing.T, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	return scenarioVariant(t, game.BlackSix, 1, stacks, hands...)
}

// 黑色提示點到 chop 上的 k2（沒有例外）不是 save clue，是 play clue：Bob 看得到 Cathy 的 k2，
// 要在 finesse position 盲打 k1。
func TestBlackColorOnChopK2IsPlayClue(t *testing.T) {
	g, bots := blackScenario(t, "", "g4 y4 b4 w4 g3", "k1 r3 y3 b3 w3", "r4 y4 g3 b4 k2")
	force(t, g, game.HintColorTo(2, game.Black))
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("黑色點到 chop 上的 k2 是 play clue，Bob 應盲打 finesse 的 k1（slot 1），卻做了 %+v", a)
	}
}

// 例外：黑色提示點到兩張之前沒被提示的黑牌時可以救 k2，Bob 不盲打。
func TestBlackColorTwoNewBlackCardsCanSaveK2(t *testing.T) {
	g, bots := blackScenario(t, "", "g4 y4 b4 w4 g3", "k1 r3 y3 b3 w3", "r4 k4 g3 b4 k2")
	force(t, g, game.HintColorTo(2, game.Black))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("黑色點到兩張新的黑牌時可以是救 k2 的 save clue，Bob 不該盲打，卻做了 %+v", a)
	}
}

// 例外：黑色提示補上一張已提示牌的資訊（filled in an ancillary card）時可以救 k2。
func TestBlackColorFillingInCanSaveK2(t *testing.T) {
	g, bots := blackScenario(t, "", "g4 y4 b4 w4 g3", "k1 r3 y3 b3 w3", "r4 k4 g3 b4 k2")
	g.Hands[2][1].Know = game.Knowledge{Colors: 0x7f, Ranks: 1 << 3, HintedRanks: 1 << 3, Clued: true}
	force(t, g, game.HintColorTo(2, game.Black))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("黑色補上已提示的 4 的資訊時可以是救 k2 的 save clue，Bob 不該盲打，卻做了 %+v", a)
	}
}

// 數字 3 點到 chop 上的 k3（沒有點到別張可能是 critical 的牌）不是 save clue，是 play clue。
func TestRankOnChopK3IsPlayClue(t *testing.T) {
	g, bots := blackScenario(t, "k1", "g4 y4 b4 w4 g3", "k2 r3 y3 b3 w3", "r4 y4 g4 b4 k3")
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a != game.Play(0) {
		t.Fatalf("「3」點到 chop 上的 k3 是 play clue，Bob 應盲打 finesse 的 k2（slot 1），卻做了 %+v", a)
	}
}

// Florrat Save：「3」同時點到另一張可能是 critical 的 3（另一張 r3 已被棄掉）時可以救 k3，Bob 不盲打。
func TestFlorratSave(t *testing.T) {
	g, bots := blackScenario(t, "k1", "g4 y4 b4 w4 g3", "k2 r1 y1 b1 w1", "r4 r3 g4 b4 k3")
	g.Discards = append(g.Discards, game.Card{ID: 999, Color: game.Red, Rank: 3})
	force(t, g, game.HintRankTo(2, 3))
	if a := step(t, g, bots); a.Type == game.ActPlay && a.Slot == 0 {
		t.Fatalf("「3」也點到可能是 critical 的 r3，是 Florrat Save，Bob 不該盲打 k2，卻做了 %+v", a)
	}
}

// 給提示：救 chop 上的 k2 用數字 2、救 k3 用黑色。
func TestSaveBlackWithRightClue(t *testing.T) {
	for _, tc := range []struct {
		chop string
		want game.Action
	}{
		{"k2", game.HintRankTo(1, 2)},
		{"k5", game.HintRankTo(1, 5)},
		{"k3", game.HintColorTo(1, game.Black)},
		{"k4", game.HintColorTo(1, game.Black)},
	} {
		g, bots := blackScenario(t, "", "g4 y4 b4 w4 g3", "r4 y4 g4 b4 "+tc.chop, "r3 y3 w3 b3 g5")
		if a := step(t, g, bots); a != tc.want {
			t.Errorf("救 Bob chop 上的 %s 應給 %+v，卻做了 %+v", tc.chop, tc.want, a)
		}
	}
}

// 「1」點到 chop 時可能是救 k1 的 save clue，但同一個提示點到的牌照樣依 Good Touch 不會重複：
// Bob 打出 chop 上的 y1 後，另外兩張 1 就不是 y1，都能打（Save Principle 一節：接收者照樣依 Good Touch 寫筆記）
func TestOnesOnChopStillDistinctAfterOnePlays(t *testing.T) {
	g, bots := scenarioVariant(t, game.BlackSix, 4, "", "g4 r4 b4 w4 g3", "g4 k1 r4 b1 y1", "r3 y3 g3 b3 w3")
	g.Hints = 6
	force(t, g, game.HintRankTo(1, 1))
	if a := step(t, g, bots); a != game.Play(4) {
		t.Fatalf("Bob 應先打 chop 上的 1（y1），卻做了 %+v", a)
	}
	force(t, g, game.Discard(4)) // Cathy
	force(t, g, game.Discard(4)) // Alice
	if a := step(t, g, bots); a.Type != game.ActPlay {
		t.Fatalf("另外兩張 1 依 Good Touch 不是 y1、都能打，Bob 應打出其中一張，卻做了 %+v", a)
	}
}

// 現在就能打的牌面不算 save：「1」點到 chop 永遠是 play clue，就算 Black 的 k1 是 critical
// （那種可能本身就是 play clue，接收者會直接打出）
func TestOneOnChopIsNeverSave(t *testing.T) {
	g, bots := blackScenario(t, "", "g4 r4 b4 w4 g3", "g4 r4 y4 b4 k1", "r3 y3 g3 b3 w3")
	bots[0].Act(g.View(0))
	s := bots[0].s
	h := game.Hint{Kind: game.HintRank, Rank: 1}
	it := s.interpret(0, 1, h, []int{g.Hands[1][4].Card.ID})
	for _, c := range it.cands {
		if c.save {
			t.Errorf("「1」點到 chop 不該有 save 解讀，cands=%+v", it.cands)
		}
	}
	if len(it.cands) == 0 {
		t.Error("「1」點到 chop 應是 play clue")
	}
}

func TestBlindPlayNotExplainedBySaveClueFallback(t *testing.T) {
	// 使用者回報 20261120-144720（Black，4 人局）：t25 使用者白色點 AI-2 chop 上的 W3（記成「W2 或 W3」）；t31 AI-2 的「5」
	// 救 AI-1 chop 上的 W5，fallback 留著「W5，AI-1 Self-Finesse W2–W4」；t33 使用者白色點 AI-3 的 W4，t34 AI-1 盲打出 W2。
	// 「5」可能是 save clue，不會讓 AI-1 盲打，這次盲打證明了白色是 W4（finesse AI-1 的 W2），AI-2 的白牌就是被 prompt 的 W3
	const codes = "h3B p1 h11 h2R h25 p4 p1 h0K d2 p4 h35 d3 h35 p1 d4 p4 p1 p2 h1G p3 p1 d3 h12 p2 h2W p4 h3G p3 h33 p3 h15 p4 h3W p1"
	g, bots := replayCodesVariant(t, 4, game.BlackSix, 1795157240339150000, 12, codes)
	if c := g.Hands[2][3].Card; c.Color != game.White || c.Rank != 3 {
		t.Fatalf("布置錯誤：AI-2 slot 4 應是 W3，卻是 %+v", c)
	}
	if a := step(t, g, bots); a != game.Play(3) {
		t.Fatalf("AI-2 應打出被 prompt 的 W3（slot 4），卻做了 %+v（%s）", a, bots[2].Why())
	}
}

func TestFlorratSaveNeedsActuallyCriticalCard(t *testing.T) {
	// 使用者回報 20261120-155843 t30（Black，4 人局）：AI-3 chop 上是 K4，「4」會一起點到 slot 3 的 G4。B4、W4 各棄了一張，
	// 接收者看不到 G4、以為它可能是 critical，但原文 Florrat Save 要「also touched another critical card」：看得到 G4 的人
	// 會把「4」讀成 K4 的 play clue。AI-1 要用黑色救 K4
	const codes = "h31 h35 h32 p2 h3R h03 h0R p1 d4 h04 d4 p3 p3 h0G d4 p3 p3 h22 d3 p4 p2 d4 h11 h12 d2 p4 p4 d4 h1W"
	g, bots := replayCodesVariant(t, 4, game.BlackSix, 1795161523285457000, 12, codes)
	if c := g.Hands[3][3].Card; c.Color != game.Black || c.Rank != 4 {
		t.Fatalf("布置錯誤：AI-3 slot 4 應是 K4，卻是 %+v", c)
	}
	if a := step(t, g, bots); a != game.HintColorTo(3, game.Black) {
		t.Fatalf("AI-1 應用黑色救 K4（一起點到的 G4 不是 critical，「4」不是 Florrat Save），卻做了 %+v（%s）", a, bots[1].Why())
	}
}
