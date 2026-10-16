package game

import (
	"errors"
	"slices"
	"testing"
)

// newTestGame 建立一局 n 人遊戲，並把每位玩家手牌換成指定的牌（未指定者保留原牌）。
func newTestGame(t *testing.T, n int, hands ...[]Card) *Game {
	t.Helper()
	g, err := New(n, 1)
	if err != nil {
		t.Fatal(err)
	}
	for p, cs := range hands {
		g.Hands[p] = nil
		for _, c := range cs {
			g.Hands[p] = append(g.Hands[p], HandCard{Card: c, Know: NewKnowledge(NoVariant)})
		}
	}
	return g
}

func c(col Color, r Rank) Card { return Card{Color: col, Rank: r} }

func mustApply(t *testing.T, g *Game, a Action) []Event {
	t.Helper()
	evs, err := g.Apply(a)
	if err != nil {
		t.Fatalf("Apply(%+v) 失敗: %v", a, err)
	}
	return evs
}

func TestNewGame(t *testing.T) {
	for n := MinPlayers; n <= MaxPlayers; n++ {
		g, err := New(n, 42)
		if err != nil {
			t.Fatal(err)
		}
		hs := HandSize(n)
		for p, h := range g.Hands {
			if len(h) != hs {
				t.Errorf("%d 人局 座位 %d 手牌 %d 張，想要 %d", n, p, len(h), hs)
			}
		}
		if got := len(g.Deck) + n*hs; got != 50 {
			t.Errorf("%d 人局 總牌數 %d，想要 50", n, got)
		}
	}
	if _, err := New(1, 0); err == nil {
		t.Error("1 人局應該回傳錯誤")
	}
	if _, err := New(6, 0); err == nil {
		t.Error("6 人局應該回傳錯誤")
	}
}

func TestDeckComposition(t *testing.T) {
	for _, va := range Variants {
		g, err := NewVariant(2, va, 7)
		if err != nil {
			t.Fatal(err)
		}
		count := map[Card]int{}
		all := append([]Card(nil), g.Deck...)
		for _, h := range g.Hands {
			all = append(all, cardsOf(h)...)
		}
		ids := map[int]bool{}
		for _, cd := range all {
			ids[cd.ID] = true
			count[Card{Color: cd.Color, Rank: cd.Rank}]++
		}
		size := map[Variant]int{NoVariant: 50, SixSuits: 60, BlackSix: 55, RainbowSix: 60}[va]
		if len(ids) != size || len(all) != size {
			t.Errorf("%s：應有 %d 張不同 ID 的牌，有 %d 張、%d 種 ID", va.Name(), size, len(all), len(ids))
		}
		for _, col := range AllColors {
			for r := Rank(1); r <= MaxRank; r++ {
				want := Copies(col, r)
				if !va.HasColor(col) {
					want = 0
				}
				if got := count[c(col, r)]; got != want {
					t.Errorf("%s：%s 有 %d 張，想要 %d", va.Name(), c(col, r), got, want)
				}
			}
		}
	}
}

// 5 色時 NewVariant 的牌序必須和舊版 New 相同，舊的對局紀錄才能重播。
func TestFiveSuitsSameDeckAsNew(t *testing.T) {
	a, _ := New(3, 42)
	b, _ := NewVariant(3, NoVariant, 42)
	for i := range a.Deck {
		if a.Deck[i] != b.Deck[i] {
			t.Fatalf("第 %d 張不同：%v vs %v", i, a.Deck[i], b.Deck[i])
		}
	}
}

func TestTealHintOnlyInSixSuits(t *testing.T) {
	g := newTestGame(t, 2, nil, []Card{c(Teal, 1), c(Red, 1)})
	if _, err := g.Apply(HintColorTo(1, Teal)); !errors.Is(err, ErrBadHint) {
		t.Errorf("5 色局不應能提示青色，得到 %v", err)
	}
	g6, _ := NewVariant(2, SixSuits, 1)
	g6.Hands[1] = []HandCard{{Card: c(Teal, 1), Know: NewKnowledge(SixSuits)}, {Card: c(Red, 1), Know: NewKnowledge(SixSuits)}}
	mustApply(t, g6, HintColorTo(1, Teal))
	if k := g6.Hands[1][0].Know; !k.CanBeColor(Teal) || k.CanBeColor(Red) {
		t.Errorf("被點到的牌應確定是青色：%+v", k)
	}
	if k := g6.Hands[1][1].Know; k.CanBeColor(Teal) || !k.CanBeColor(White) {
		t.Errorf("沒被點到的牌應排除青色：%+v", k)
	}
	if _, err := g6.Apply(HintColorTo(0, Black)); !errors.Is(err, ErrBadHint) {
		t.Errorf("6 Suits 不應能提示黑色，得到 %v", err)
	}
}

func TestBlackVariant(t *testing.T) {
	g, _ := NewVariant(2, BlackSix, 1)
	if g.Variant.MaxScore() != 30 || g.Variant.HasColor(Teal) || !g.Variant.HasColor(Black) {
		t.Fatalf("Black (6 Suits) 應是 R/Y/G/B/W/K、滿分 30：%v", g.Variant.Colors())
	}
	g.Hands[1] = []HandCard{{Card: c(Black, 2), Know: NewKnowledge(BlackSix)}, {Card: c(Red, 1), Know: NewKnowledge(BlackSix)}}
	if k := g.Hands[1][1].Know; !k.CanBeColor(Black) || k.CanBeColor(Teal) {
		t.Errorf("新牌的認知應含黑色、不含青色：%+v", k)
	}
	mustApply(t, g, HintColorTo(1, Black))
	if k := g.Hands[1][0].Know; !k.CanBeColor(Black) || k.CanBeColor(Red) {
		t.Errorf("被點到的牌應確定是黑色：%+v", k)
	}
	for _, va := range Variants {
		if got, ok := ParseVariant(va.Code()); !ok || got != va {
			t.Errorf("ParseVariant(%q) = %v, %v", va.Code(), got, ok)
		}
	}
}

func TestRainbowVariant(t *testing.T) {
	g, _ := NewVariant(2, RainbowSix, 1)
	if g.Variant.MaxScore() != 30 || !g.Variant.HasColor(Rainbow) || g.Variant.CanClue(Rainbow) || len(g.Variant.ClueColors()) != 5 {
		t.Fatalf("Rainbow (6 Suits) 應是 R/Y/G/B/W/M、滿分 30、彩虹色不能提示：%v", g.Variant.Colors())
	}
	k := func() Knowledge { return NewKnowledge(RainbowSix) }
	g.Hands[1] = []HandCard{{Card: c(Rainbow, 2), Know: k()}, {Card: c(Red, 1), Know: k()}, {Card: c(Blue, 3), Know: k()}}
	if _, err := g.Apply(HintColorTo(1, Rainbow)); !errors.Is(err, ErrBadHint) {
		t.Errorf("不應能提示彩虹色，得到 %v", err)
	}
	evs := mustApply(t, g, HintColorTo(1, Red))
	if !slices.Equal(evs[0].Touched, []int{0, 1}) {
		t.Errorf("紅色應點到彩虹 2 與紅 1：%v", evs[0].Touched)
	}
	if k := g.Hands[1][0].Know; !k.CanBeColor(Red) || !k.CanBeColor(Rainbow) || k.CanBeColor(Blue) {
		t.Errorf("被紅色點到的牌是紅色或彩虹色：%+v", k)
	}
	if k := g.Hands[1][2].Know; k.CanBeColor(Red) || k.CanBeColor(Rainbow) || !k.CanBeColor(Blue) {
		t.Errorf("沒被紅色點到的牌不是紅色也不是彩虹色：%+v", k)
	}
	g.Current = 0
	mustApply(t, g, HintColorTo(1, Blue))
	if c, ok := g.Hands[1][0].Know.KnownColor(); !ok || c != Rainbow {
		t.Errorf("被紅色與藍色點到的牌一定是彩虹色：%+v", g.Hands[1][0].Know)
	}
	if c, ok := g.Hands[1][0].Know.HintedColor(); !ok || c != Rainbow {
		t.Errorf("只看正向提示也看得出是彩虹色：%v %v", c, ok)
	}
	// 只被紅色點到的牌，只看正向提示時就是紅色（被兩種顏色點到才看得出是彩虹色）
	if c, ok := g.Hands[1][1].Know.HintedColor(); !ok || c != Red {
		t.Errorf("只被紅色點到的牌，正向資訊是紅色：%v %v", c, ok)
	}
}

func TestSameSeedSameGame(t *testing.T) {
	a, _ := New(3, 99)
	b, _ := New(3, 99)
	for i := range a.Deck {
		if a.Deck[i] != b.Deck[i] {
			t.Fatal("相同 seed 應產生相同牌序")
		}
	}
}

func TestPlaySuccessAndDraw(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(Blue, 3), c(Red, 1), c(Green, 2)})
	deck := len(g.Deck)
	evs := mustApply(t, g, Play(1))
	if evs[0].Type != EvPlay || evs[0].Card.Color != Red {
		t.Fatalf("想要成功打出 R1，得到 %+v", evs[0])
	}
	if g.Fireworks[Red] != 1 {
		t.Errorf("紅色煙火 = %d，想要 1", g.Fireworks[Red])
	}
	if evs[1].Type != EvDraw || len(g.Deck) != deck-1 {
		t.Error("打出後應抽一張牌")
	}
	if h := g.Hands[0]; len(h) != 3 || h[1].Card != c(Blue, 3) || h[2].Card != c(Green, 2) {
		t.Errorf("新牌應插在最左，其餘往右移：%v", cardsOf(h))
	}
	if g.Current != 1 || g.Turn != 2 {
		t.Errorf("應換到座位 1 第 2 回合，得到座位 %d 第 %d 回合", g.Current, g.Turn)
	}
}

func TestMisplay(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(Red, 2)})
	evs := mustApply(t, g, Play(0))
	if evs[0].Type != EvMisplay || g.Strikes != 1 || len(g.Discards) != 1 {
		t.Errorf("R2 在紅色 0 時應打出失敗：%+v strikes=%d", evs[0], g.Strikes)
	}
	if g.Fireworks[Red] != 0 {
		t.Error("失敗不應推進煙火")
	}
}

func TestPlayFiveRegainsHint(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(Red, 5), c(Blue, 5)})
	g.Fireworks[Red], g.Fireworks[Blue] = 4, 4
	g.Hints = 5
	evs := mustApply(t, g, Play(0))
	if !evs[0].Bonus || g.Hints != 6 {
		t.Errorf("打出 5 應回收提示，hints=%d", g.Hints)
	}
	// 提示已滿時打出 5 不再增加
	g.Current, g.Hints = 0, MaxHints
	evs = mustApply(t, g, Play(1))
	if evs[0].Bonus || g.Hints != MaxHints {
		t.Errorf("提示已滿時不應回收，hints=%d", g.Hints)
	}
}

func TestDiscard(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(Red, 1), c(Red, 2)})
	if _, err := g.Apply(Discard(0)); !errors.Is(err, ErrHintsFull) {
		t.Errorf("提示 8 時棄牌應被拒絕，得到 %v", err)
	}
	g.Hints = 3
	mustApply(t, g, Discard(1))
	if g.Hints != 4 || len(g.Discards) != 1 || g.Discards[0] != c(Red, 2) {
		t.Errorf("棄牌後 hints=%d discards=%v", g.Hints, g.Discards)
	}
}

func TestBadSlot(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(Red, 1)})
	for _, a := range []Action{Play(1), Play(-1)} {
		if _, err := g.Apply(a); !errors.Is(err, ErrBadSlot) {
			t.Errorf("%+v 應回傳 ErrBadSlot，得到 %v", a, err)
		}
	}
}

func TestHintRules(t *testing.T) {
	g := newTestGame(t, 3, nil, []Card{c(Red, 1), c(Blue, 1), c(Red, 3), c(Green, 4)})
	if _, err := g.Apply(HintColorTo(0, Red)); !errors.Is(err, ErrHintSelf) {
		t.Errorf("提示自己應被拒絕，得到 %v", err)
	}
	if _, err := g.Apply(HintColorTo(5, Red)); !errors.Is(err, ErrBadTarget) {
		t.Errorf("不存在的玩家應被拒絕，得到 %v", err)
	}
	if _, err := g.Apply(HintColorTo(1, White)); !errors.Is(err, ErrHintTouchesNone) {
		t.Errorf("點不到任何牌應被拒絕，得到 %v", err)
	}
	if _, err := g.Apply(HintRankTo(1, 9)); !errors.Is(err, ErrBadHint) {
		t.Errorf("數字 9 應被拒絕，得到 %v", err)
	}

	evs := mustApply(t, g, HintColorTo(1, Red))
	if g.Hints != MaxHints-1 {
		t.Errorf("提示後 hints=%d", g.Hints)
	}
	if got := evs[0].Touched; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("紅色應點到位置 0、2，得到 %v", got)
	}
	h := g.Hands[1]
	if col, ok := h[0].Know.KnownColor(); !ok || col != Red || !h[0].Know.Clued {
		t.Error("位置 0 應被認知為紅色且標記為被點到")
	}
	if h[1].Know.CanBeColor(Red) || !h[1].Know.CanBeColor(Blue) || h[1].Know.Clued {
		t.Error("位置 1 應得到「非紅」的負向資訊，且不算被點到")
	}
	if c, ok := h[0].Know.HintedColor(); !ok || c != Red {
		t.Error("位置 0 的正向提示顏色應為紅")
	}
	if _, ok := h[1].Know.HintedColor(); ok {
		t.Error("位置 1 沒被點到，不應有正向提示顏色")
	}

	g.Current = 0
	mustApply(t, g, HintRankTo(1, 1))
	if col, ok := h[0].Know.KnownColor(); !ok || col != Red {
		t.Error("數字提示不應影響顏色認知")
	}
	if r, ok := h[0].Know.KnownRank(); !ok || r != 1 {
		t.Error("位置 0 應被認知為 1")
	}
	if h[3].Know.CanBeRank(1) || h[3].Know.CanBeColor(Red) {
		t.Error("位置 3 應同時累積「非紅」「非 1」")
	}
	if r, ok := h[0].Know.HintedRank(); !ok || r != 1 {
		t.Error("位置 0 的正向提示數字應為 1")
	}
	if _, ok := h[3].Know.HintedRank(); ok {
		t.Error("位置 3 沒被點到，不應有正向提示數字")
	}

	g.Current, g.Hints = 0, 0
	if _, err := g.Apply(HintRankTo(1, 1)); !errors.Is(err, ErrNoHints) {
		t.Errorf("沒有提示 token 時應被拒絕，得到 %v", err)
	}
}

func TestThreeStrikesEndsWithZero(t *testing.T) {
	// 新抽的牌會插在最左，所以一直打最右邊的 R3
	g := newTestGame(t, 2, []Card{c(Red, 3), c(Red, 3), c(Red, 3)})
	var evs []Event
	for range 3 {
		g.Current = 0
		evs = mustApply(t, g, Play(2))
	}
	if !g.Over || g.EndReason != EndStrikes {
		t.Fatal("第三道閃電應結束遊戲")
	}
	last := evs[len(evs)-1]
	if last.Type != EvGameOver || last.Score != 0 || g.Score() != 0 {
		t.Errorf("三道閃電應得 0 分，得到 %d", g.Score())
	}
	if _, err := g.Apply(HintColorTo(1, Red)); !errors.Is(err, ErrGameOver) {
		t.Error("結束後不應接受行動")
	}
}

func TestPerfectEndsImmediately(t *testing.T) {
	g := newTestGame(t, 2, []Card{c(White, 5)})
	g.Fireworks = [NumColors]Rank{5, 5, 5, 5, 4}
	mustApply(t, g, Play(0))
	if !g.Over || g.EndReason != EndPerfect || g.Score() != NoVariant.MaxScore() {
		t.Errorf("25 分應立即結束，over=%v reason=%v score=%d", g.Over, g.EndReason, g.Score())
	}
}

func TestFinalRound(t *testing.T) {
	for n := MinPlayers; n <= MaxPlayers; n++ {
		g, _ := New(n, 3)
		g.Hints = 0
		g.Deck = g.Deck[len(g.Deck)-1:] // 只剩一張
		drawer := g.Current
		mustApply(t, g, Discard(0))
		if len(g.Deck) != 0 || g.FinalTurnsLeft != n {
			t.Fatalf("%d 人局：抽走最後一張後應剩 %d 回合，得到 %d", n, n, g.FinalTurnsLeft)
		}
		turns := 0
		for !g.Over {
			if g.Current == drawer && turns < n-1 {
				t.Fatalf("%d 人局：抽最後一張者在其他人之前又輪到了", n)
			}
			// 只用提示消耗回合，避免依賴手牌內容
			g.Hints = 1
			mustApply(t, g, g.View(g.Current).LegalHints()[0])
			turns++
		}
		if turns != n || g.EndReason != EndDeckOut {
			t.Errorf("%d 人局：最後一輪應有 %d 回合，得到 %d（reason=%v）", n, n, turns, g.EndReason)
		}
	}
}

func TestViewHidesOwnCards(t *testing.T) {
	g, _ := New(3, 5)
	g.Hints = 0
	mustApply(t, g, Discard(0)) // 座位 0 抽了一張新牌
	v := g.View(0)
	for _, vc := range v.MyHand() {
		if vc.Visible || vc.Card.Rank != 0 {
			t.Errorf("自己的牌不應可見：%+v", vc)
		}
	}
	for _, e := range v.Log {
		if e.Type == EvDraw && e.Player == 0 && e.Card.Rank != 0 {
			t.Error("自己抽到的牌不應出現在紀錄中")
		}
	}
	for _, vc := range v.Hands[1] {
		if !vc.Visible || vc.Card.Rank == 0 {
			t.Errorf("別人的牌應可見：%+v", vc)
		}
	}
	if g.View(1).Log[1].Card.Rank == 0 {
		t.Error("別人應看得到座位 0 抽到的牌")
	}
}
