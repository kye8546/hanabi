package hgroup

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"hanabi/game"
	"hanabi/player"
)

// playGame 讓 n 個指定等級的 AI 打一局變體 va 的牌，回傳遊戲結果。
func playGame(t testing.TB, n int, va game.Variant, level int, seed uint64) *game.Game {
	t.Helper()
	g, err := runGame(n, va, level, seed)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func runGame(n int, va game.Variant, level int, seed uint64) (*game.Game, error) {
	g, err := game.NewVariant(n, va, seed)
	if err != nil {
		return nil, err
	}
	ps := make([]player.Player, n)
	for i := range ps {
		ps[i] = New(fmt.Sprintf("AI-%d", i), level)
	}
	if err := player.Run(g, ps); err != nil {
		return nil, fmt.Errorf("%s Lv%d %d 人 seed %d: %v", va.Name(), level, n, seed, err)
	}
	return g, nil
}

// 全 AI 對局：不可以有不合法行動，並印出各等級 × 人數的平均分數表（go test -v 看得到）。
// HGAMES 局數（預設 200；比較改動用 1000）、HLEVEL 等級（例如 4 或 1,4；預設全部）、
// HVARIANT 變體（none / 6suits / black6 / rainbow6，可用逗號列多個；預設標準 5 色）。各局平行跑，結果和循序跑完全相同。
func TestSelfPlay(t *testing.T) {
	games := 200
	if testing.Short() {
		games = 30
	}
	if n, err := strconv.Atoi(os.Getenv("HGAMES")); err == nil {
		games = n
	}
	var levels []int
	for _, f := range strings.Split(os.Getenv("HLEVEL"), ",") {
		if lv, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			levels = append(levels, lv)
		}
	}
	if len(levels) == 0 {
		for lv := 1; lv <= MaxLevel; lv++ {
			levels = append(levels, lv)
		}
	}
	selfPlay(t, envVariants(), levels, games)
}

// envVariant 是除錯測試的變體：HVARIANT 是紀錄裡的代碼（none / 6suits / black6），
// 舊的 HSUITS=6 也還能用；預設標準 5 色。列了多個時用第一個。
func envVariant() game.Variant { return envVariants()[0] }

func envVariants() []game.Variant {
	var vas []game.Variant
	for _, f := range strings.Split(os.Getenv("HVARIANT"), ",") {
		if va, ok := game.ParseVariant(strings.TrimSpace(f)); ok {
			vas = append(vas, va)
		}
	}
	if len(vas) == 0 && os.Getenv("HSUITS") == "6" {
		vas = []game.Variant{game.SixSuits}
	}
	if len(vas) == 0 {
		vas = []game.Variant{game.NoVariant}
	}
	return vas
}

// 變體：每個等級少量對局，確認沒有不合法行動、分數合理（詳細數字用 HVARIANT 跑 TestSelfPlay）。
// Black 的 2 人局三閃電約 1.2%（critical 牌多，被迫拖延、救不到的情況比較多），50 局時偶爾就超過 5% 的警戒線，所以跑 100 局。
func TestVariantSelfPlay(t *testing.T) {
	games := 100
	if testing.Short() {
		games = 10
	}
	var levels []int
	for lv := 1; lv <= MaxLevel; lv++ {
		levels = append(levels, lv)
	}
	selfPlay(t, []game.Variant{game.SixSuits, game.BlackSix, game.RainbowSix}, levels, games)
}

// simStats 是一組（變體、等級、人數）自我對局的累計結果。
type simStats struct {
	total, bombs, perfect, strikes, maxSum int
}

// selfPlay 讓每組（變體 × 等級 × 2–5 人）各打 seed 0..games-1，用所有 CPU 平行跑，
// 印出每組的結果與分數表；平均分數低於 18 或三閃電超過 5% 算退步。
func selfPlay(t *testing.T, vas []game.Variant, levels []int, games int) {
	type group struct {
		va    game.Variant
		lv, n int
	}
	var groups []group
	for _, va := range vas {
		for _, lv := range levels {
			for n := game.MinPlayers; n <= game.MaxPlayers; n++ {
				groups = append(groups, group{va, lv, n})
			}
		}
	}
	stats := make([]simStats, len(groups))
	type job struct {
		g    int
		seed uint64
	}
	jobs := make(chan job)
	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				gr := groups[j.g]
				g, err := runGame(gr.n, gr.va, gr.lv, j.seed)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
				} else {
					st := &stats[j.g]
					st.total += g.Score()
					st.strikes += g.Strikes
					st.maxSum += maxScore(g)
					if g.EndReason == game.EndStrikes {
						st.bombs++
					}
					if g.Score() == gr.va.MaxScore() {
						st.perfect++
					}
				}
				mu.Unlock()
			}
		}()
	}
	for i := range groups {
		for seed := range uint64(games) {
			jobs <- job{i, seed}
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		t.Fatal(firstErr)
	}

	f := func(x int) float64 { return float64(x) / float64(games) }
	var table strings.Builder
	fmt.Fprintf(&table, "\n%d 局，平均分數（平均閃電）\n%-22s", games, "")
	for n := game.MinPlayers; n <= game.MaxPlayers; n++ {
		fmt.Fprintf(&table, "%-13s", fmt.Sprintf("%d 人", n)) // 「人」佔兩格寬
	}
	for i, gr := range groups {
		st := stats[i]
		avg := f(st.total)
		t.Logf("%s Lv%d %d 人：平均 %.2f 分（棄牌後理論上限 %.2f），平均閃電 %.2f，三閃電 %d 局，滿分 %d 局（共 %d 局）",
			gr.va.Name(), gr.lv, gr.n, avg, f(st.maxSum), f(st.strikes), st.bombs, st.perfect, games)
		// 退步警戒線：自我對局目前約 21–23 分
		if avg < 18 || st.bombs*20 > games {
			t.Errorf("%s Lv%d %d 人表現退步：平均 %.2f 分、三閃電 %d/%d 局", gr.va.Name(), gr.lv, gr.n, avg, st.bombs, games)
		}
		if gr.n == game.MinPlayers {
			fmt.Fprintf(&table, "\n%-22s", fmt.Sprintf("%s Lv%d", gr.va.Name(), gr.lv))
		}
		fmt.Fprintf(&table, "%-14s", fmt.Sprintf("%.2f (%.2f)", avg, f(st.strikes)))
	}
	t.Log(table.String())
}

// maxScore 回傳依棄牌堆計算、這局理論上還能拿到的最高分。
func maxScore(g *game.Game) int {
	lost := map[game.Card]int{}
	for _, c := range g.Discards {
		lost[game.Card{Color: c.Color, Rank: c.Rank}]++
	}
	total := 0
	for _, col := range g.Variant.Colors() {
		for r := game.Rank(1); r <= game.MaxRank; r++ {
			if lost[game.Card{Color: col, Rank: r}] >= game.Copies(col, r) {
				break
			}
			total++
		}
	}
	return total
}
