package hgroup

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"hanabi/game"
	"hanabi/player"
)

// playGame 讓 n 個指定等級的 AI 打一局變體 va 的牌，回傳遊戲結果。
func playGame(t testing.TB, n int, va game.Variant, level int, seed uint64) *game.Game {
	t.Helper()
	g, err := game.NewVariant(n, va, seed)
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]player.Player, n)
	for i := range ps {
		ps[i] = New(fmt.Sprintf("AI-%d", i), level)
	}
	if err := player.Run(g, ps); err != nil {
		t.Fatalf("%d 人 seed %d: %v", n, seed, err)
	}
	return g
}

// 全 AI 對局：不可以有不合法行動，並印出各人數的平均分數（go test -v 看得到）。
// 預設標準 5 色；HVARIANT=6suits / black6 跑變體（envVariant）。
func TestSelfPlay(t *testing.T) {
	games := 200
	if testing.Short() {
		games = 30
	}
	if n, err := strconv.Atoi(os.Getenv("HGAMES")); err == nil {
		games = n // 例如 HGAMES=1000 跑更多局，減少隨機誤差
	}
	levels := []int{}
	for lv := 1; lv <= MaxLevel; lv++ {
		levels = append(levels, lv)
	}
	if lv, err := strconv.Atoi(os.Getenv("HLEVEL")); err == nil {
		levels = []int{lv} // 例如 HLEVEL=2 只跑 Level 2
	}
	va := envVariant()
	for _, lv := range levels {
		for n := game.MinPlayers; n <= game.MaxPlayers; n++ {
			selfPlay(t, n, va, lv, games)
		}
	}
}

// envVariant 是自我對局與除錯測試的變體：HVARIANT 是紀錄裡的代碼（none / 6suits / black6），
// 舊的 HSUITS=6 也還能用；預設標準 5 色。
func envVariant() game.Variant {
	if va, ok := game.ParseVariant(os.Getenv("HVARIANT")); ok {
		return va
	}
	if os.Getenv("HSUITS") == "6" {
		return game.SixSuits
	}
	return game.NoVariant
}

// 變體：每個等級少量對局，確認沒有不合法行動、分數合理（詳細數字用 HVARIANT 跑 TestSelfPlay）。
// Black 的 2 人局三閃電約 1.2%（critical 牌多，被迫拖延、救不到的情況比較多），50 局時偶爾就超過 5% 的警戒線，所以跑 100 局。
func TestVariantSelfPlay(t *testing.T) {
	games := 100
	if testing.Short() {
		games = 10
	}
	for _, va := range []game.Variant{game.SixSuits, game.BlackSix} {
		for lv := 1; lv <= MaxLevel; lv++ {
			for n := game.MinPlayers; n <= game.MaxPlayers; n++ {
				selfPlay(t, n, va, lv, games)
			}
		}
	}
}

func selfPlay(t *testing.T, n int, va game.Variant, lv, games int) {
	{
		total, bombs, perfect, strikes, maxSum := 0, 0, 0, 0, 0
		for seed := range uint64(games) {
			g := playGame(t, n, va, lv, seed)
			total += g.Score()
			strikes += g.Strikes
			maxSum += maxScore(g)
			if g.EndReason == game.EndStrikes {
				bombs++
			}
			if g.Score() == va.MaxScore() {
				perfect++
			}
		}
		avg := float64(total) / float64(games)
		t.Logf("%s Lv%d %d 人：平均 %.2f 分（棄牌後理論上限 %.2f），平均閃電 %.2f，三閃電 %d 局，滿分 %d 局（共 %d 局）",
			va.Name(), lv, n, avg, float64(maxSum)/float64(games), float64(strikes)/float64(games), bombs, perfect, games)
		// 退步警戒線：自我對局目前約 21–23 分
		if avg < 18 || bombs*20 > games {
			t.Errorf("%s Lv%d %d 人表現退步：平均 %.2f 分、三閃電 %d/%d 局", va.Name(), lv, n, avg, bombs, games)
		}
	}
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
