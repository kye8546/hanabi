package ai

import (
	"fmt"
	"testing"

	"hanabi/game"
	"hanabi/player"
)

// 全 AI 煙霧測試：各種人數、多個 seed 都應正常結束且分數合理。
func TestDiscardBotGamesFinish(t *testing.T) {
	for _, va := range game.Variants {
		for n := game.MinPlayers; n <= game.MaxPlayers; n++ {
			for seed := range uint64(100) {
				g, err := game.NewVariant(n, va, seed)
				if err != nil {
					t.Fatal(err)
				}
				ps := make([]player.Player, n)
				for i := range ps {
					ps[i] = NewDiscardBot(fmt.Sprintf("AI-%d", i))
				}
				if err := player.Run(g, ps); err != nil {
					t.Fatalf("%d 人 seed %d: %v", n, seed, err)
				}
				if !g.Over || g.Score() < 0 || g.Score() > va.MaxScore() {
					t.Fatalf("%s %d 人 seed %d: over=%v score=%d", va.Name(), n, seed, g.Over, g.Score())
				}
			}
		}
	}
}
