package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hanabi/console"
	"hanabi/game"
	"hanabi/player"
)

// gameLog 把每一局寫成 logs/ 下的文字檔：開頭是重現這局所需的設定，
// 之後每行一個行動（含可解析的行動代碼與 AI 的決策理由）。
// 相同的 seed、人數、等級、變體與行動序列可以完整重現整局（舊紀錄寫 suits=5/6，沒有就是 5 色）。
type gameLog struct {
	path   string
	header []string
	lines  []logLine
	r      *console.Renderer
	why    map[int]string // 回合 → AI 的決策理由
	done   int            // 已寫入的事件數
}

type logLine struct {
	turn int
	text string
}

func newGameLog(names []string, set console.Settings, seed uint64) (*gameLog, error) {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return nil, err
	}
	l := &gameLog{
		path: filepath.Join("logs", time.Now().Format("20060102-150405")+".log"),
		header: []string{
			fmt.Sprintf("# 花火對局紀錄 %s", time.Now().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("seed=%d players=%d level=%d variant=%s", seed, set.Players, set.Level, set.Variant.Code()),
			"# 每行：回合 座位 行動代碼 | 發生的事 | AI 理由（# 開頭的是註記，例如回溯、偷看）",
		},
		r:   console.NewRenderer(io.Discard, names, false, false),
		why: map[int]string{},
	}
	return l, l.save()
}

func (l *gameLog) Path() string { return l.path }

// save 把整份紀錄重寫到檔案（回溯時要刪掉被撤銷的行，所以每次都整份重寫；檔案很小）。
func (l *gameLog) save() error {
	var b strings.Builder
	for _, h := range l.header {
		b.WriteString(h + "\n")
	}
	for _, ln := range l.lines {
		b.WriteString(ln.text + "\n")
	}
	return os.WriteFile(l.path, []byte(b.String()), 0o644)
}

// flush 寫入尚未寫過的事件（抽牌不寫）。
func (l *gameLog) flush(evs []game.Event) {
	for _, e := range evs[l.done:] {
		code := actionCode(e)
		if code == "" {
			continue
		}
		line := fmt.Sprintf("t%-3d P%d %-4s | %s", e.Turn, e.Player, code, l.r.Describe(e))
		if w := l.why[e.Turn]; w != "" && e.Type != game.EvGameOver {
			line += " | " + w
		}
		l.lines = append(l.lines, logLine{e.Turn, line})
	}
	l.done = len(evs)
	l.save()
}

// note 加一行註記（# 開頭，重播時會被忽略）。
func (l *gameLog) note(turn int, text string) {
	l.lines = append(l.lines, logLine{turn, "# " + text})
	l.save()
}

// rewind 回溯到第 turn 回合之前：刪掉該回合起的行動，留下註記。g 是重建後的局面。
func (l *gameLog) rewind(g *game.Game, turn int) {
	kept := l.lines[:0]
	for _, ln := range l.lines {
		if ln.turn < turn || strings.HasPrefix(ln.text, "#") {
			kept = append(kept, ln)
		}
	}
	l.lines = kept
	l.done = len(g.Log)
	l.note(turn, fmt.Sprintf("t%d 回溯：撤銷第 %d 回合起的行動", turn, turn))
}

func (l *gameLog) close(g *game.Game) {
	l.flush(g.Log)
}

// actionCode 把事件寫成可解析的行動代碼：p3（打出位置 3）、d5（棄位置 5）、h2r / h23（提示 2 號紅色 / 數字 3）。
func actionCode(e game.Event) string {
	switch e.Type {
	case game.EvPlay, game.EvMisplay:
		return "p" + strconv.Itoa(e.Slot+1)
	case game.EvDiscard:
		return "d" + strconv.Itoa(e.Slot+1)
	case game.EvHint:
		if e.Hint.Kind == game.HintColor {
			return fmt.Sprintf("h%d%s", e.Target, e.Hint.Color.Letter())
		}
		return fmt.Sprintf("h%d%d", e.Target, e.Hint.Rank)
	case game.EvGameOver:
		return "end"
	}
	return ""
}

// logged 包住一位玩家：每次輪到他時先把之前的事件寫進紀錄，行動後記下 AI 的理由。
type logged struct {
	player.Player
	l *gameLog
}

func (p logged) Act(v game.PlayerView) game.Action {
	p.l.flush(v.Log)
	a := p.Player.Act(v)
	if w, ok := p.Player.(interface{ Why() string }); ok {
		p.l.why[v.Turn] = w.Why()
	}
	return a
}

// loggedHuman 額外轉送 Reject，讓真人輸入不合法時能重來。
type loggedHuman struct {
	logged
	h player.Rejecter
}

func (p loggedHuman) Reject(a game.Action, err error) { p.h.Reject(a, err) }
