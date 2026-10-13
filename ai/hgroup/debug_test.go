package hgroup

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"hanabi/game"
)

func evStr(e game.Event, names func(int) string) string {
	switch e.Type {
	case game.EvPlay:
		return fmt.Sprintf("%s 打出 %s(slot%d)", names(e.Player), e.Card, e.Slot+1)
	case game.EvMisplay:
		return fmt.Sprintf("%s 打錯 %s(slot%d) ✗", names(e.Player), e.Card, e.Slot+1)
	case game.EvDiscard:
		return fmt.Sprintf("%s 棄 %s(slot%d)", names(e.Player), e.Card, e.Slot+1)
	case game.EvHint:
		h := ""
		if e.Hint.Kind == game.HintColor {
			h = e.Hint.Color.Letter()
		} else {
			h = fmt.Sprint(e.Hint.Rank)
		}
		sl := []string{}
		for _, s := range e.Touched {
			sl = append(sl, fmt.Sprint(s+1))
		}
		return fmt.Sprintf("%s 提示 P%d %s → %s", names(e.Player), e.Target, h, strings.Join(sl, ","))
	}
	return ""
}

func handStr(g *game.Game, p int) string {
	var parts []string
	for _, hc := range g.Hands[p] {
		parts = append(parts, hc.Card.String())
	}
	return strings.Join(parts, " ")
}

// debugLevel 是除錯測試使用的慣例等級（HLEVEL，預設 1）。
func debugLevel() int {
	lv := 1
	fmt.Sscan(os.Getenv("HLEVEL"), &lv)
	return lv
}

func TestTraceSeed(t *testing.T) {
	if os.Getenv("HTRACE") == "" {
		t.Skip()
	}
	var n int
	var seed uint64
	var card, dumpTurn int
	fmt.Sscan(os.Getenv("HTRACE"), &n, &seed, &card, &dumpTurn)
	g, _ := game.NewVariant(n, envVariant(), seed)
	bots := make([]*Bot, n)
	for i := range bots {
		bots[i] = New(fmt.Sprint(i), debugLevel())
	}
	names := func(p int) string { return fmt.Sprintf("P%d", p) }
	for !g.Over {
		p := g.Current
		if bots[p].s == nil {
			bots[p].s = newState(g.View(p), bots[p].level)
			bp := p
			bots[p].s.trace = func(e game.Event, id int, old, nw idSet) {
				if card < 0 || id == card {
					fmt.Printf("        [P%d 視角] card#%d(%s @P%d): %v → %v  (t%d)\n", bp, id, cardOf(g, id), bots[bp].s.holder(id), old.list(), nw.list(), e.Turn)
				}
			}
		}
		a := bots[p].Act(g.View(p))
		why := bots[p].s.why
		if g.Turn == dumpTurn {
			for q := range n {
				if bots[q].s != nil {
					fmt.Printf("    P%d's view of P0: %s\n", q, dumpHand(bots[q], 0))
				}
			}
			st := bots[p].s
			for _, ev := range st.allClues() {
				fmt.Printf("    EV %+v\n", ev)
				var touched []int
				for _, id := range st.hands[ev.target] {
					if ev.act.Hint.Matches(st.cards[id]) {
						touched = append(touched, id)
					}
				}
				it := st.interpret(st.me, ev.target, ev.act.Hint, touched)
				fmt.Printf("       cands=%+v plan=%+v planOK=%v saveOK=%v chop=%v\n", it.cands, it.plan, it.planOK, it.saveOK, it.chopFocus)
			}
		}
		evs, err := g.Apply(a)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("t%d %-30s (%s)   stacks=%v\n", evs[0].Turn, evStr(evs[0], names), why, g.Fireworks)
		for q := range n {
			fmt.Printf("      P%d: %s\n", q, handIDs(g, q))
		}
	}
}

func cardOf(g *game.Game, id int) string {
	for _, h := range g.Hands {
		for _, hc := range h {
			if hc.Card.ID == id {
				return hc.Card.String()
			}
		}
	}
	return "?"
}

func handIDs(g *game.Game, p int) string {
	var parts []string
	for _, hc := range g.Hands[p] {
		parts = append(parts, fmt.Sprintf("%s#%d", hc.Card, hc.Card.ID))
	}
	return strings.Join(parts, " ")
}

func TestDebugMisplays(t *testing.T) {
	if os.Getenv("HDEBUG") == "" {
		t.Skip()
	}
	n := 2
	fmt.Sscan(os.Getenv("HN"), &n)
	shown := 0
	for seed := range uint64(200) {
		g, _ := game.NewVariant(n, envVariant(), seed)
		bots := make([]*Bot, n)
		for i := range bots {
			bots[i] = New(fmt.Sprint(i), debugLevel())
		}
		names := func(p int) string { return fmt.Sprintf("P%d", p) }
		reasons := map[int]string{}
		for !g.Over {
			p := g.Current
			snap := make([]string, n)
			for q := range n {
				snap[q] = handStr(g, q)
			}
			a := bots[p].Act(g.View(p))
			var noteS string
			if a.Type == game.ActPlay {
				id := g.Hands[p][a.Slot].Card.ID
				nt := bots[p].s.notes[id]
				noteS = fmt.Sprintf("poss=%v myPoss=%v fin=%v clued=%v", nt.poss.list(), bots[p].s.myPoss(id).list(), nt.finessed, nt.clued)
			}
			why := bots[p].s.why
			evs, err := g.Apply(a)
			if err != nil {
				t.Fatal(err)
			}
			reasons[evs[0].Turn] = why
			if evs[0].Type == game.EvMisplay && shown < 12 {
				shown++
				fmt.Printf("=== seed %d turn %d: %s  [%s]\n", seed, evs[0].Turn, evStr(evs[0], names), noteS)
				fmt.Printf("    stacks %v\n", g.Fireworks)
				for q := range n {
					fmt.Printf("    P%d hand(before): %s\n", q, snap[q])
				}
				lg := g.Log
				start := max(0, len(lg)-14)
				for _, e := range lg[start:] {
					if s := evStr(e, names); s != "" {
						fmt.Printf("      t%d %-28s  (%s)\n", e.Turn, s, reasons[e.Turn])
					}
				}
			}
		}
	}
}

func TestDebugCriticalDiscards(t *testing.T) {
	if os.Getenv("HCRIT") == "" {
		t.Skip()
	}
	n := 2
	fmt.Sscan(os.Getenv("HCRIT"), &n)
	reasons := map[string]int{}
	shown := 0
	for seed := range uint64(200) {
		g, _ := game.NewVariant(n, envVariant(), seed)
		bots := make([]*Bot, n)
		for i := range bots {
			bots[i] = New(fmt.Sprint(i), debugLevel())
		}
		for !g.Over {
			p := g.Current
			hints := g.Hints
			a := bots[p].Act(g.View(p))
			why := bots[p].s.why
			var crit bool
			var c game.Card
			chopSlot := -1
			if a.Type == game.ActDiscard {
				c = g.Hands[p][a.Slot].Card
				disc := 0
				for _, d := range g.Discards {
					if d.Color == c.Color && d.Rank == c.Rank {
						disc++
					}
				}
				crit = c.Rank > g.Fireworks[c.Color] && disc == game.Copies(c.Color, c.Rank)-1
				st := bots[p].s
				if ch := st.chop(p); ch >= 0 {
					for i, id := range st.hands[p] {
						if id == ch {
							chopSlot = i
						}
					}
				}
			}
			g.Apply(a)
			if crit {
				key := fmt.Sprintf("%s/hints=%d/chop=%v", why, hints, chopSlot == a.Slot)
				reasons[key]++
				if shown < 8 && os.Getenv("HSHOW") != "" && hints >= 4 {
					shown++
					fmt.Printf("seed %d turn %d P%d discards critical %s (%s) hints=%d\n", seed, g.Turn-1, p, c, why, hints)
				}
			}
		}
	}
	for k, v := range reasons {
		fmt.Println(v, k)
	}
}

func dumpHand(b *Bot, p int) string {
	var parts []string
	for _, id := range b.s.hands[p] {
		n := b.s.notes[id]
		extra := ""
		if p == b.s.me {
			extra = fmt.Sprintf(" myPoss=%v", b.s.myPoss(id).list())
		}
		if n.bad {
			extra += " BAD"
		}
		parts = append(parts, fmt.Sprintf("#%d{clued=%v fin=%v will=%v poss=%v%s}", id, n.clued, n.finessed, n.willPlay, n.poss.list(), extra))
	}
	return strings.Join(parts, " ")
}

// TestReplayLog 重播玩家的對局紀錄（logs/*.log），確認 AI 行動一致，並可傾印某回合各 AI 的筆記：
// HREPLAY="logs/xxx.log 12" go test ./ai/hgroup -run TestReplayLog -v（HFREE=11：第 11 回合起 AI 照自己的決定）
func TestReplayLog(t *testing.T) {
	if os.Getenv("HREPLAY") == "" {
		t.Skip()
	}
	var path string
	var dumpTurn int
	fmt.Sscan(os.Getenv("HREPLAY"), &path, &dumpTurn)
	free, _ := strconv.Atoi(os.Getenv("HFREE")) // 從這回合起 AI 不照紀錄、照自己的決定（看改動後的走向）
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var seed uint64
	var n, level int
	va := game.NoVariant      // 舊紀錄沒有 variant= / suits=，是 5 色
	codes := map[int]string{} // 回合 → 行動代碼
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "seed=") {
			fmt.Sscanf(line, "seed=%d players=%d level=%d", &seed, &n, &level)
			for _, f := range strings.Fields(line) {
				if code, ok := strings.CutPrefix(f, "variant="); ok {
					va, _ = game.ParseVariant(code)
				} else if f == "suits=6" {
					va = game.SixSuits
				}
			}
		}
		var turn, seat int
		var code string
		if _, err := fmt.Sscanf(line, "t%d P%d %s", &turn, &seat, &code); err == nil {
			codes[turn] = code
		}
	}
	g, err := game.NewVariant(n, va, seed)
	if err != nil {
		t.Fatal(err)
	}
	bots := make([]*Bot, n)
	for i := range bots {
		bots[i] = New(fmt.Sprint(i), level)
	}
	names := func(p int) string { return fmt.Sprintf("P%d", p) }
	for !g.Over {
		p, turn := g.Current, g.Turn
		code, ok := codes[turn]
		if !ok {
			break
		}
		want := parseCode(t, code)
		var a game.Action
		if p == 0 {
			a = want
			if bots[0].s == nil {
				bots[0].s = newState(g.View(0), bots[0].level)
			}
			bots[0].s.sync(g.View(0))
		} else {
			a = bots[p].Act(g.View(p))
			if a != want {
				extra := ""
				if a.Type == game.ActPlay {
					c := g.Hands[p][a.Slot].Card
					extra = fmt.Sprintf("［會打出 %s：%v］", c, c.Rank == g.Fireworks[c.Color]+1)
				}
				t.Errorf("t%d P%d：重播得到 %+v，紀錄是 %s（AI 程式可能已改變）%s", turn, p, a, code, extra)
				if free == 0 || turn < free {
					a = want
				} else {
					fmt.Printf("    （HFREE：P%d 改照 AI 行動，之後真人照紀錄行動可能不合理）\n", p)
				}
			}
		}
		if turn == dumpTurn && p != 0 {
			st := bots[p].s
			for _, ev := range st.allClues() {
				fmt.Printf("  EV %s→P%d kind=%d focus=#%d und=%v mis=%v danger=%v bad=%d good=%d score=%d\n",
					evHint(ev.act.Hint), ev.target, ev.kind, ev.focus, ev.understood, ev.misleads, ev.danger, ev.bad, ev.newGood, ev.score)
				if ev.kind != kindNone {
					it := st.interpret(st.me, ev.target, ev.act.Hint, ev.touched)
					fmt.Printf("     cands=%v plan=%+v\n", it.cands, it.plan)
				}
			}
		}
		if turn == dumpTurn {
			fmt.Printf("=== 第 %d 回合開始前（輪到 P%d）\n", turn, p)
			for q := range n {
				fmt.Printf("  實際 P%d: %s\n", q, handIDs(g, q))
			}
			for q, b := range bots {
				if b.s == nil {
					continue
				}
				fmt.Printf("  P%d 的筆記：\n", q)
				for h := range n {
					fmt.Printf("    P%d: %s\n", h, dumpHand(b, h))
				}
				for _, r := range b.s.recs {
					fmt.Printf("    rec P%d→P%d focus=#%d cands=%+v fallback=%+v plan=%+v\n", r.giver, r.target, r.focus, r.cands, r.fallback, r.plan)
				}
			}
		}
		evs, err := g.Apply(a)
		if err != nil {
			t.Fatalf("t%d: %v", turn, err)
		}
		why := ""
		if p != 0 {
			why = bots[p].s.why
		}
		fmt.Printf("t%-3d %-34s %s\n", turn, evStr(evs[0], names), why)
	}
}

func parseCode(t *testing.T, code string) game.Action {
	t.Helper()
	switch {
	case len(code) == 2 && code[0] == 'p':
		return game.Play(int(code[1]-'0') - 1)
	case len(code) == 2 && code[0] == 'd':
		return game.Discard(int(code[1]-'0') - 1)
	case len(code) == 3 && code[0] == 'h':
		target := int(code[1] - '0')
		if c, ok := game.ParseColor(code[2:]); ok {
			return game.HintColorTo(target, c)
		}
		return game.HintRankTo(target, game.Rank(code[2]-'0'))
	}
	t.Fatalf("看不懂行動代碼 %q", code)
	return game.Action{}
}

func evHint(h game.Hint) string {
	if h.Kind == game.HintColor {
		return h.Color.Letter()
	}
	return fmt.Sprint(h.Rank)
}

// TestDebugLevelDiff：同樣的牌序分別用 Lv(L-1) 與 Lv(L) 自我對局，列出分數差最多的局（評估新等級用）。
// HLDIFF="人數 L"，例如 HLDIFF="4 4" 比較 4 人局的 Lv3 與 Lv4。
func TestDebugLevelDiff(t *testing.T) {
	if os.Getenv("HLDIFF") == "" {
		t.Skip()
	}
	n, lv := 4, MaxLevel
	fmt.Sscan(os.Getenv("HLDIFF"), &n, &lv)
	type r struct {
		seed     uint64
		old, new int
	}
	var rs []r
	sum := 0
	for seed := range uint64(1000) {
		a := playGame(t, n, envVariant(), lv-1, seed).Score()
		b := playGame(t, n, envVariant(), lv, seed).Score()
		sum += b - a
		if a != b {
			rs = append(rs, r{seed, a, b})
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].new-rs[i].old < rs[j].new-rs[j].old })
	t.Logf("%d 人 1000 局：分數不同 %d 局，Lv%d 比 Lv%d 總共多 %d 分", n, len(rs), lv, lv-1, sum)
	for _, x := range rs[:min(15, len(rs))] {
		t.Logf("seed %d：Lv%d %d → Lv%d %d", x.seed, lv-1, x.old, lv, x.new)
	}
}

// TestDebugChopMoveAgreement：自我對局中，每張被 Chop Move 的牌是否所有玩家（含持有者）都這樣記
// （Chop Move 必須是大家都看得懂的共同資訊）。HCMAGREE=人數，吃 HLEVEL（預設 1，要用 HLEVEL=4 以上）。
func TestDebugChopMoveAgreement(t *testing.T) {
	if os.Getenv("HCMAGREE") == "" {
		t.Skip()
	}
	n := 4
	fmt.Sscan(os.Getenv("HCMAGREE"), &n)
	total, dis := 0, 0
	for seed := range uint64(300) {
		g, _ := game.NewVariant(n, envVariant(), seed)
		bots := make([]*Bot, n)
		for i := range bots {
			bots[i] = New(fmt.Sprint(i), debugLevel())
		}
		seen := map[int]bool{}
		for !g.Over {
			p := g.Current
			a := bots[p].Act(g.View(p))
			why := bots[p].s.why
			g.Apply(a)
			for o, b := range bots {
				if b.s == nil {
					b.s = newState(g.View(o), b.level)
				}
				b.s.sync(g.View(o))
			}
			for q, h := range g.Hands {
				for slot, hc := range h {
					id := hc.Card.ID
					var who []int
					for o, b := range bots {
						if nt := b.s.notes[id]; nt != nil && nt.cm {
							who = append(who, o)
						}
					}
					if len(who) == 0 || seen[id] {
						continue
					}
					seen[id] = true
					total++
					if len(who) < n {
						dis++
						t.Logf("seed %d t%d（%s %+v）後 P%d slot%d %v：只有 %v 認為被 Chop Move", seed, g.Turn-1, why, a, q, slot+1, hc.Card, who)
					}
				}
			}
		}
	}
	t.Logf("被 Chop Move 的牌 %d 張，各家看法不一致 %d 張", total, dis)
}
