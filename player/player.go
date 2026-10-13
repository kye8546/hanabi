// Package player 定義真人與 AI 共用的玩家介面，以及驅動一局遊戲的主迴圈。
package player

import (
	"errors"
	"fmt"

	"hanabi/game"
)

// Player 是一個座位的決策者。Act 只會拿到自己的 PlayerView，看不到自己的手牌。
type Player interface {
	Name() string
	Act(v game.PlayerView) game.Action
}

// Rejecter 是選用介面：Act 回傳不合法行動時，主迴圈會呼叫 Reject 再重新 Act。
// 真人玩家應實作它（讓使用者重輸）；AI 不實作，回傳不合法行動視為程式錯誤。
type Rejecter interface {
	Reject(a game.Action, err error)
}

// UndoAction 是真人「回溯上一動」的特殊行動（不是遊戲規則裡的行動）：Run 收到時回傳 ErrUndo，
// 由呼叫端重建局面後再繼續呼叫 Run。
var UndoAction = game.Action{Type: -1}

// ErrUndo 表示真人要求回溯。
var ErrUndo = errors.New("回溯")

// Run 讓 players 依座位輪流行動直到遊戲結束（或有人要求回溯，回傳 ErrUndo）。
func Run(g *game.Game, players []Player) error {
	if len(players) != g.NumPlayers {
		return fmt.Errorf("需要 %d 位玩家，收到 %d 位", g.NumPlayers, len(players))
	}
	for !g.Over {
		p := players[g.Current]
		for {
			a := p.Act(g.View(g.Current))
			if a == UndoAction {
				return ErrUndo
			}
			_, err := g.Apply(a)
			if err == nil {
				break
			}
			r, ok := p.(Rejecter)
			if !ok {
				return fmt.Errorf("%s 做出不合法行動 %+v: %w", p.Name(), a, err)
			}
			r.Reject(a, err)
		}
	}
	return nil
}
