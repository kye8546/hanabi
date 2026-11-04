package session

import (
	"hanabi/ai/hgroup"
	"hanabi/game"
	"hanabi/player"
)

// Helped 是真人的座位：每次輪到他時先讓代打用的 AI 跟上局面（AI 代打才用得到它的決定）。
type Helped struct {
	player.Player
	Helper *hgroup.Bot
}

func (p Helped) Act(v game.PlayerView) game.Action {
	p.Helper.Observe(v)
	return p.Player.Act(v)
}

// Reject 轉送給真人（Helped 包的一定是會被退回的真人）。
func (p Helped) Reject(a game.Action, err error) { p.Player.(player.Rejecter).Reject(a, err) }

// Rewind 回到座位 seat 上一次行動之前：用同一個 seed 重新開局，把那之前的行動照順序重播。
// AI 每一手都重新呼叫 Act（它們依完整紀錄逐步建立筆記），得到的狀態和原本一樣；真人的座位（Helped）只讓代打的 AI Observe。
// 回傳新局面、新的玩家、被撤銷的第一個回合；沒有可回溯的行動時 ok 為 false。
func Rewind(g *game.Game, seed uint64, seat int, newPlayers func() []player.Player) (*game.Game, []player.Player, int, bool) {
	type act struct {
		turn int
		a    game.Action
	}
	var acts []act
	last := -1
	for _, e := range g.Log {
		var a game.Action
		switch e.Type {
		case game.EvPlay, game.EvMisplay:
			a = game.Play(e.Slot)
		case game.EvDiscard:
			a = game.Discard(e.Slot)
		case game.EvHint:
			a = game.Action{Type: game.ActHint, Target: e.Target, Hint: e.Hint}
		default:
			continue
		}
		if e.Player == seat {
			last = len(acts)
		}
		acts = append(acts, act{e.Turn, a})
	}
	if last < 0 {
		return g, nil, 0, false
	}
	ng, err := game.NewVariant(g.NumPlayers, g.Variant, seed)
	if err != nil {
		return g, nil, 0, false
	}
	ps := newPlayers()
	for _, x := range acts[:last] {
		p := ng.Current
		if o, ok := ps[p].(Helped); ok {
			o.Helper.Observe(ng.View(p))
		} else {
			// 讓 AI 照原本的節奏建立筆記（它會選出同一個行動）；直接呼叫 AI，不經過紀錄等包裝（那些行已經寫過）
			bot := ps[p]
			for {
				w, ok := bot.(interface{ Unwrap() player.Player })
				if !ok {
					break
				}
				bot = w.Unwrap()
			}
			bot.Act(ng.View(p))
		}
		if _, err := ng.Apply(x.a); err != nil {
			return g, nil, 0, false
		}
	}
	return ng, ps, acts[last].turn, true
}
