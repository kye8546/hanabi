package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"hanabi/ai/hgroup"
	"hanabi/console"
	"hanabi/game"
	"hanabi/player"
)

func main() {
	seed := flag.Uint64("seed", 0, "洗牌種子；0 = 開局時詢問（直接按 Enter 為隨機）。相同種子可重玩同一副牌")
	name := flag.String("name", "你", "你的名字")
	noColor := flag.Bool("nocolor", false, "不使用終端顏色")
	hideNeg := flag.Bool("hideneg", false, "手牌只顯示被提示點到的顏色/數字，不顯示反向提示資訊（預設和 Hanab Live 一樣顯示，例如提示紅色時，沒被點到的牌會排除紅色）")
	flag.Parse()

	// 人數與慣例等級每局開始前都必須選，不做成啟動參數；種子沒用 -seed 指定時也在這裡問
	in := bufio.NewScanner(os.Stdin)
	set := console.AskSetup(in, os.Stdout, hgroup.Levels[:], hgroup.MaxLevel, *seed)
	n := set.Players

	*seed = set.Seed
	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
		set.Seed = *seed
	}
	// 任何結束前都先等玩家按 Enter（Windows 直接執行 exe 時視窗才不會一結束就關掉）
	exit := func(code int) {
		console.WaitEnter(in, os.Stdout)
		os.Exit(code)
	}
	g, err := game.NewVariant(n, set.Variant, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(2)
	}

	names := make([]string, n)
	names[0] = *name
	for i := 1; i < n; i++ {
		names[i] = fmt.Sprintf("AI-%d", i)
	}
	color := !*noColor && os.Getenv("NO_COLOR") == ""
	r := console.NewRenderer(os.Stdout, names, color, !*hideNeg)

	gl, err := newGameLog(names, set, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "無法建立對局紀錄：", err)
		exit(1)
	}
	human := console.NewHuman(names[0], in, os.Stdout, r, names)
	// 輸入 q 離開時也要把紀錄寫完（Human 之後會直接結束程式，所以在這裡等 Enter）
	human.OnQuit = func() {
		gl.close(g)
		fmt.Printf("本局紀錄：%s\n", gl.Path())
		console.WaitEnter(in, os.Stdout)
	}
	human.Peek = func() []game.Card {
		gl.note(g.Turn, fmt.Sprintf("t%d 偷看了自己的手牌", g.Turn))
		var cs []game.Card
		for _, hc := range g.Hands[0] {
			cs = append(cs, hc.Card)
		}
		return cs
	}
	newPlayers := func() []player.Player {
		ps := []player.Player{loggedHuman{logged{human, gl}, human}}
		for i := 1; i < n; i++ {
			ps = append(ps, logged{hgroup.New(names[i], set.Level), gl})
		}
		return ps
	}
	players := newPlayers()

	variant := ""
	if set.Variant != game.NoVariant {
		variant = " · " + set.Variant.Name()
	}
	fmt.Printf("\n花火 · %d 人局%s · H-Group Lv%d %s · seed %d（輸入 ? 看說明）\n",
		n, variant, set.Level, hgroup.Levels[set.Level-1], *seed)
	fmt.Printf("本局紀錄會存到 %s\n", gl.Path())
	for {
		err := player.Run(g, players)
		if errors.Is(err, player.ErrUndo) {
			ng, ps, turn, ok := rewind(g, *seed, newPlayers)
			if !ok {
				fmt.Println("✗ 還沒有可以回溯的行動")
				continue
			}
			g, players = ng, ps
			gl.rewind(g, turn)
			human.Rewound(g.View(0))
			continue
		}
		if err != nil {
			gl.close(g)
			fmt.Fprintln(os.Stderr, err)
			exit(1)
		}
		break
	}
	human.Final(g.View(0))
	human.OnQuit()
}

// rewind 回到真人（座位 0）上一次行動之前：用同一個 seed 重新開局，把那之前的行動照順序重播。
// AI 每一手都重新呼叫 Act（它們依完整紀錄逐步建立筆記），得到的狀態和原本一樣。
// 回傳新局面、新的玩家、被撤銷的第一個回合；沒有可回溯的行動時 ok 為 false。
func rewind(g *game.Game, seed uint64, newPlayers func() []player.Player) (*game.Game, []player.Player, int, bool) {
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
		if e.Player == 0 {
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
		if p != 0 {
			// 讓 AI 照原本的節奏建立筆記（它會選出同一個行動）；直接呼叫 AI，不經過紀錄（那些行已經寫過）
			bot := ps[p]
			if lp, ok := bot.(logged); ok {
				bot = lp.Player
			}
			bot.Act(ng.View(p))
		}
		if _, err := ng.Apply(x.a); err != nil {
			return g, nil, 0, false
		}
	}
	return ng, ps, acts[last].turn, true
}
