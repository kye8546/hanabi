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
