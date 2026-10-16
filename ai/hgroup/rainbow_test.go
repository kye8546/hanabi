package hgroup

import (
	"testing"

	"hanabi/game"
)

// Rainbow 變體的慣例（https://hanabi.github.io/variant-specific/rainbow）與 Level 5 Prompts in Multi-Color Variants。
// 彩虹色的牌寫成 m1–m5。

func rainbowScenario(t *testing.T, level int, stacks string, hands ...string) (*game.Game, []*Bot) {
	t.Helper()
	return scenarioVariant(t, game.RainbowSix, level, stacks, hands...)
}

// promptSetup：Bob 的 slot 1、slot 2 是已提示的牌，正向資訊（hinted）與筆記照參數設定；回傳 Bob 的 state 與兩張牌的 ID。
func promptSetup(t *testing.T, level int, h1, h2 uint16, p1, p2 idSet) (*state, int, int) {
	t.Helper()
	g, bots := rainbowScenario(t, level, "r2", "g4 y4 b4 w4 g3", "m4 r3 y3 b3 w3", "r4 y4 g3 b4 w4")
	g.Current = 1
	bots[1].Act(g.View(1))
	s := bots[1].s
	a, b := g.Hands[1][0].Card.ID, g.Hands[1][1].Card.ID
	for _, x := range []struct {
		id   int
		h    uint16
		poss idSet
	}{{a, h1, p1}, {b, h2, p2}} {
		n := s.notes[x.id]
		n.clued, n.hinted, n.poss, n.explicit = true, x.h, x.poss, x.poss
	}
	return s, a, b
}

func colorSet(cs ...game.Color) idSet {
	var out idSet
	for _, c := range cs {
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			out |= setOf(identOf(c, r))
		}
	}
	return out
}

func TestL5MultiColorPromptMostPositiveInfo(t *testing.T) {
	// 原文 Level 5 Prompts in Multi-Color Variants：r2 已打出，Alice 提示 Cathy 紅色點到 r4，要 prompt Bob 的 r3。
	// Bob 的 slot 1 只有紅色提示（任何紅牌或彩虹牌），slot 2 有紅色與 3（r3 或 m3）：正向資訊多的 slot 2 先
	red, three := hintBit(game.Hint{Kind: game.HintColor, Color: game.Red}), hintBit(game.Hint{Kind: game.HintRank, Rank: 3})
	r3 := identOf(game.Red, 3)
	for _, lv := range []int{4, 5} {
		s, a, b := promptSetup(t, lv, red, red|three, colorSet(game.Red, game.Rainbow), setOf(r3, identOf(game.Rainbow, 3)))
		d, _, ok := s.findPrompt(1, 0, r3, nil)
		want := b
		if lv < lvMultiPrompt {
			want = a // Level 4 以下不用這個慣例：最左邊
		}
		if !ok || d.card != want {
			t.Errorf("Lv%d：應 prompt %d，卻是 %+v（%v）", lv, want, d, ok)
		}
	}
}

func TestL5MultiColorPromptIgnoresNegativeInfo(t *testing.T) {
	// 同上，但 slot 2 是正向紅色、負向綠色（只可能是紅牌）：比較時不看負向資訊，兩張一樣多，照常 prompt 最左邊的 slot 1
	red := hintBit(game.Hint{Kind: game.HintColor, Color: game.Red})
	r3 := identOf(game.Red, 3)
	s, a, _ := promptSetup(t, 5, red, red, colorSet(game.Red, game.Rainbow), colorSet(game.Red))
	if d, _, ok := s.findPrompt(1, 0, r3, nil); !ok || d.card != a {
		t.Errorf("負向資訊不算，應 prompt 最左邊的 %d，卻是 %+v（%v）", a, d, ok)
	}
}

// freeChoicePlan：r1 已打出，Bob（座位 1）的 slot 2 是被紅色點到、不知道是什麼的牌；Alice 用顏色 c 點到 Cathy 的 m2
// （Cathy 沒有紅、藍、白色的牌，這三種顏色都只會點到 m2：Free Choice）。回傳 Bob 算出的計畫與他兩張牌的 ID。
func freeChoicePlan(t *testing.T, level int, bob string, c game.Color) ([]dep, int, int) {
	t.Helper()
	g, bots := rainbowScenario(t, level, "r1", "g4 y4 b4 w4 g3", bob, "y2 g3 m2 y5 g4")
	g.Current = 1
	bots[1].Act(g.View(1))
	s := bots[1].s
	a, b := g.Hands[1][0].Card.ID, g.Hands[1][1].Card.ID
	n := s.notes[b]
	n.clued, n.hinted = true, hintBit(game.Hint{Kind: game.HintColor, Color: game.Red})
	n.poss = colorSet(game.Red, game.Rainbow) &^ setOf(identOf(game.Red, 1))
	n.explicit = n.poss
	it := s.interpret(0, 2, game.Hint{Kind: game.HintColor, Color: c}, []int{g.Hands[2][2].Card.ID})
	return it.plan, a, b
}

func TestFreeChoicePrompt(t *testing.T) {
	// 原文 The Free Choice Finesse 第一個例子：Alice 選了和 Bob 那張牌一樣的紅色 → prompt，Bob 打出 slot 2 的 m1
	plan, _, b := freeChoicePlan(t, 1, "y3 m1 b3 w3 g2", game.Red)
	if len(plan) != 1 || plan[0].card != b || plan[0].finesse {
		t.Fatalf("紅色是 prompt Bob 的 slot 2：%+v", plan)
	}
}

func TestFreeChoiceFinesse(t *testing.T) {
	// 第二個例子：Alice 有 Free Choice 卻選了藍色（不是 Bob 那張牌的紅色）→ finesse，Bob 盲打 slot 1 的 m1
	plan, a, _ := freeChoicePlan(t, 1, "m1 r4 b3 w3 g2", game.Blue)
	if len(plan) != 1 || plan[0].card != a || !plan[0].finesse {
		t.Fatalf("藍色是 Free Choice Finesse，應 finesse Bob 的 slot 1：%+v", plan)
	}
}

func TestRainbowCriticalSavedWithAnyColor(t *testing.T) {
	// 原文 Saving Rainbow Cards：critical 的彩虹 2/3/4 可以用任何顏色救。另一張 m3 已被棄，Alice 用紅色點到 Cathy chop 上的 m3：
	// 可能是 save clue，Bob 不能當成 play clue 而盲打 finesse position 的 r1、r2
	g, bots := rainbowScenario(t, 2, "", "g4 y4 b4 w4 g3", "r1 r2 y3 b3 w3", "y4 g4 b4 w4 m3")
	g.Discards = append(g.Discards, card(t, "m3"))
	force(t, g, game.HintColorTo(2, game.Red))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("紅色點到 chop 上 critical 的 m3 可能是 save clue，Bob 不該盲打：%+v", a)
	}
}

func TestRainbowTwoSave(t *testing.T) {
	// 彩虹 2 和一般的 2 一樣要用 2 Save：「2」點到 Cathy chop 上的 m2，Bob 不盲打
	g, bots := rainbowScenario(t, 1, "", "g4 y4 b4 w4 g3", "m1 r2 y3 b3 w3", "y4 g4 b4 w4 m2")
	force(t, g, game.HintRankTo(2, 2))
	if a := step(t, g, bots); a.Type == game.ActPlay {
		t.Fatalf("「2」點到 chop 上的 m2 是 2 Save，Bob 不該盲打：%+v", a)
	}
}

func TestBlindPlayRulesOutMyPromptedCard(t *testing.T) {
	// 使用者回報 20261016-181213（Lv5）：t1 使用者黃色點到 AI-2 全部的牌；t7「3」點到 AI-2 的 m3，AI-2 以為是 self-prompt
	// 自己黃色的 slot 3（m4）當 m2 → m3（或 y2 → y3）。t8 AI-1 盲打出 m2：給「3」的使用者看得到 AI-2 那張牌，不會 finesse 和它重複的 m2，
	// 所以那張不是 m2。之後白色把它縮成「m2」以外的彩虹牌，AI-2 不能當成已打出的 m2 而在 t18 棄掉
	g, bots := replayCodesVariant(t, 3, game.RainbowSix, 1792145533378154000, 5, "h2Y h05 p5 h11 p4 h02 h23 p1 h1B h2W p4 p4 h1R p4 p1 p4 p4")
	if a := step(t, g, bots); a == game.Discard(2) {
		t.Fatalf("AI-2 的 slot 3 是 m4，不該當成 m2 棄掉（%s）", bots[2].s.why)
	}
}
