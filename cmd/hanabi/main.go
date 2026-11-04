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
	"hanabi/session"
)

func main() {
	seed := flag.Uint64("seed", 0, "洗牌種子；0 = 開局時詢問（直接按 Enter 為隨機）。相同種子可重玩同一副牌")
	name := flag.String("name", "你", "你的名字")
	noColor := flag.Bool("nocolor", false, "不使用終端顏色")
	showNeg := flag.Bool("showneg", false, "開局就顯示反向提示資訊（例如提示紅色時，沒被點到的牌排除紅色；遊戲中可用 n 指令切換）")
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
	r := console.NewRenderer(os.Stdout, names, color, *showNeg)

	gl, err := session.NewLog("logs", names, set.Level, set.Variant, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "無法建立對局紀錄：", err)
		exit(1)
	}
	human := console.NewHuman(names[0], in, os.Stdout, r, names)
	// 輸入 q 離開時也要把紀錄寫完（Human 之後會直接結束程式，所以在這裡等 Enter）
	human.OnQuit = func() {
		gl.Close(g)
		fmt.Printf("本局紀錄：%s\n", gl.Path())
		console.WaitEnter(in, os.Stdout)
	}
	human.Peek = func() []game.Card {
		gl.Note(g.Turn, fmt.Sprintf("t%d 偷看了自己的手牌", g.Turn))
		var cs []game.Card
		for _, hc := range g.Hands[0] {
			cs = append(cs, hc.Card)
		}
		return cs
	}
	// 代打用的 AI：和其他 AI 一樣每個回合跟上局面（seat0.Observe），輸入 a 時才替你決定
	var helper *hgroup.Bot
	human.Auto = func(v game.PlayerView) (game.Action, string) {
		a := helper.Act(v)
		gl.SetWhy(v.Turn, "AI 代打："+helper.Why())
		return a, helper.Why()
	}
	newPlayers := func() []player.Player {
		helper = hgroup.New(names[0], set.Level)
		ps := []player.Player{session.Helped{Player: session.LoggedHuman{Logged: session.Logged{Player: human, L: gl}, H: human}, Helper: helper}}
		for i := 1; i < n; i++ {
			ps = append(ps, session.Logged{Player: hgroup.New(names[i], set.Level), L: gl})
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
			ng, ps, turn, ok := session.Rewind(g, *seed, 0, newPlayers)
			if !ok {
				fmt.Println("✗ 還沒有可以回溯的行動")
				continue
			}
			g, players = ng, ps
			gl.Rewind(g, turn)
			human.Rewound(g.View(0))
			continue
		}
		if err != nil {
			gl.Close(g)
			fmt.Fprintln(os.Stderr, err)
			exit(1)
		}
		break
	}
	human.Final(g.View(0))
	human.OnQuit()
}
