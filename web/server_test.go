package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPlayThrough 從網頁 API 開一局，每一手都用 AI 代打（中間試一次回溯），確認能打完、紀錄可以下載。
func TestPlayThrough(t *testing.T) {
	srv := New(t.TempDir(), 0)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	post := func(path, body string, out any) {
		t.Helper()
		res, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	var sess struct{ Room, Token, Error string }
	post("/api/new", `{"name":"測試","players":3,"level":4,"variant":"rainbow6","seed":"7"}`, &sess)
	if sess.Error != "" {
		t.Fatal(sess.Error)
	}
	q := "?room=" + sess.Room + "&token=" + sess.Token

	res, err := http.Get(ts.URL + "/api/stream" + q)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	states := make(chan snapshot, 64)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok && data != "{}" {
				var s snapshot
				json.Unmarshal([]byte(data), &s)
				states <- s
			}
		}
		close(states)
	}()

	undone := false
	acted := -1 // 上線等也會推畫面，同一回合可能收到好幾次
	deadline := time.After(60 * time.Second)
	for {
		var s snapshot
		select {
		case s = <-states:
		case <-deadline:
			t.Fatal("一分鐘內沒打完")
		}
		if s.Error != "" {
			t.Fatal(s.Error)
		}
		if s.Over {
			t.Logf("結束：%d 分，%d 回合", s.Score, s.Turn)
			break
		}
		if !s.YourTurn || s.Turn == acted {
			continue
		}
		acted = s.Turn
		var rep reply
		if s.Turn > 10 && !undone {
			undone = true
			post("/api/act"+q, `{"cmd":"undo"}`, &rep)
		} else {
			post("/api/act"+q, `{"cmd":"auto"}`, &rep)
		}
		if rep.Error != "" {
			t.Fatalf("第 %d 回合：%s", s.Turn, rep.Error)
		}
	}

	var rep reply
	post("/api/act"+q, `{"cmd":"auto"}`, &rep)
	if rep.Error == "" {
		t.Error("遊戲結束後還能下指令")
	}
	logRes, err := http.Get(ts.URL + "/api/log" + q)
	if err != nil {
		t.Fatal(err)
	}
	defer logRes.Body.Close()
	b := new(strings.Builder)
	bufio.NewReader(logRes.Body).WriteTo(b)
	for _, want := range []string{"seed=7 players=3 level=4 variant=rainbow6", "AI 代打：", "回溯", "end"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("紀錄裡沒有 %q", want)
		}
	}
	if _, err := os.Stat(srv.logDir); err != nil {
		t.Error(err)
	}
}

// TestBadToken 錯的通行碼看不到畫面。
func TestBadToken(t *testing.T) {
	srv := New(t.TempDir(), 0)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	res, err := http.Post(ts.URL+"/api/new", "application/json", strings.NewReader(`{"players":2,"level":1,"variant":"none"}`))
	if err != nil {
		t.Fatal(err)
	}
	var sess struct{ Room string }
	json.NewDecoder(res.Body).Decode(&sess)
	res.Body.Close()
	res, err = http.Get(ts.URL + "/api/stream?room=" + sess.Room + "&token=wrong")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("錯的通行碼回傳 %d，應該是 404", res.StatusCode)
	}
}

// TestPoll 長輪詢（SSE 的備用方案）：版本不同立刻回傳；版本相同時等到畫面變化。
func TestPoll(t *testing.T) {
	srv := New(t.TempDir(), 0)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	res, err := http.Post(ts.URL+"/api/new", "application/json", strings.NewReader(`{"players":2,"level":1,"variant":"none","seed":"3"}`))
	if err != nil {
		t.Fatal(err)
	}
	var sess struct{ Room, Token string }
	json.NewDecoder(res.Body).Decode(&sess)
	res.Body.Close()
	q := "?room=" + sess.Room + "&token=" + sess.Token

	type pollOut struct {
		Ver   int
		State snapshot
		Gone  string
	}
	poll := func(since int) pollOut {
		t.Helper()
		res, err := http.Get(ts.URL + "/api/poll" + q + "&since=" + strconv.Itoa(since))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("輪詢回傳 %d", res.StatusCode)
		}
		var out pollOut
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	// 等到輪到真人（開局時 AI 不會先動，但 run 的 goroutine 要先 publish）
	out := poll(0)
	for !out.State.YourTurn {
		out = poll(out.Ver)
	}

	// 版本相同時要等：送出指令後才回傳新的畫面
	got := make(chan pollOut, 1)
	go func() { got <- poll(out.Ver) }()
	select {
	case <-got:
		t.Fatal("畫面沒變卻立刻回傳")
	case <-time.After(200 * time.Millisecond):
	}
	r, err := http.Post(ts.URL+"/api/act"+q, "application/json", strings.NewReader(`{"cmd":"auto"}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	select {
	case next := <-got:
		if next.Ver == out.Ver || next.State.Turn == 0 && next.State.YourTurn {
			t.Errorf("輪詢沒拿到新的畫面：ver %d → %d", out.Ver, next.Ver)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("送出指令後輪詢沒有回傳")
	}

	res, err = http.Get(ts.URL + "/api/poll?room=" + sess.Room + "&token=wrong")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("錯的通行碼回傳 %d，應該是 404", res.StatusCode)
	}
}

// TestMultiplayer：房主開 3 人局（座位 1 等朋友、座位 2 是 AI），朋友用邀請連結加入；開始後兩人各自
// 輪流行動（AI 代打），中途讓 AI 接手朋友的座位再取消；多人局不能回溯、偷看，也看不到種子。
// 結束後房主替同一群人開下一局，朋友的畫面收到新房間、照原本的通行碼進去。
func TestMultiplayer(t *testing.T) {
	srv := New(t.TempDir(), 0)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close) // 在串流的 Cleanup 之後才關（Close 會等所有連線結束）
	post := func(path, body string, out any) {
		t.Helper()
		res, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	act := func(q, body string) reply {
		t.Helper()
		var rep reply
		post("/api/act"+q, body, &rep)
		return rep
	}
	stream := func(q string) <-chan snapshot {
		res, err := http.Get(ts.URL + "/api/stream" + q)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		ch := make(chan snapshot, 256)
		go func() {
			sc := bufio.NewScanner(res.Body)
			sc.Buffer(nil, 1<<20)
			for sc.Scan() {
				if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok && strings.HasPrefix(data, `{"phase"`) {
					var s snapshot
					json.Unmarshal([]byte(data), &s)
					ch <- s
				}
			}
			close(ch)
		}()
		return ch
	}

	var host struct{ Room, Token, Error string }
	post("/api/new", `{"name":"小明","players":3,"level":5,"variant":"none","seed":"11","seats":["human","ai"]}`, &host)
	if host.Error != "" {
		t.Fatal(host.Error)
	}
	var info snapshot
	res, err := http.Get(ts.URL + "/api/room?room=" + host.Room)
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(res.Body).Decode(&info)
	res.Body.Close()
	if info.Phase != "lobby" || len(info.Seats) != 3 || info.Seats[1].Kind != "open" || info.Seats[2].Kind != "ai" || info.Seed != "" {
		t.Fatalf("邀請連結看到的資訊不對：%+v", info)
	}

	var friend struct {
		Room, Token, Error string
		Seat               int
	}
	post("/api/join", `{"room":"`+host.Room+`","name":"小明","seat":-1}`, &friend)
	if friend.Error != "" || friend.Seat != 1 {
		t.Fatalf("加入失敗：%+v", friend)
	}
	var full reply
	post("/api/join", `{"room":"`+host.Room+`","name":"第三人","seat":-1}`, &full)
	if full.Error == "" {
		t.Error("座位滿了還能加入")
	}
	hq := "?room=" + host.Room + "&token=" + host.Token
	fq := "?room=" + host.Room + "&token=" + friend.Token
	hs, fs := stream(hq), stream(fq)

	rep := act(fq, `{"cmd":"start"}`)
	if rep.Error == "" {
		t.Error("不是房主也能開始")
	}
	rep = act(hq, `{"cmd":"start"}`)
	if rep.Error != "" {
		t.Fatal(rep.Error)
	}

	const (
		wantTakeover = iota
		tookOver
		resumed
	)
	phase := wantTakeover
	acted := map[int]int{0: -1, 1: -1} // 座位 → 已經行動過的回合
	triedUndo := false
	deadline := time.After(60 * time.Second)
	var last snapshot
	for done := false; !done; {
		var s snapshot
		var q string
		select {
		case s = <-hs:
			q = hq
		case s = <-fs:
			q = fq
		case <-deadline:
			t.Fatal("一分鐘內沒打完")
		}
		if s.Error != "" {
			t.Fatal(s.Error)
		}
		if s.Phase != "game" {
			continue
		}
		if !s.Multi || (s.Seed != "" && !s.Over) {
			t.Fatalf("多人局的畫面不對：multi=%v seed=%q", s.Multi, s.Seed)
		}
		if s.Names[0] != "小明" || s.Names[1] != "小明（1）" || s.Names[2] != "AI-2" {
			t.Fatalf("名字不對：%v", s.Names)
		}
		if s.Over {
			if s.Seat == 1 {
				last, done = s, true
			}
			continue
		}
		if s.Seat == 1 && phase == tookOver && !s.Takeover {
			continue // 設定 AI 接手之前的畫面
		}
		if !s.YourTurn || acted[s.Seat] >= s.Turn {
			continue
		}
		switch {
		case s.Seat == 0 && !triedUndo:
			triedUndo = true
			for _, cmd := range []string{"undo", "peek"} {
				rep = act(q, `{"cmd":"`+cmd+`"}`)
				if rep.Error == "" {
					t.Errorf("多人局可以 %s", cmd)
				}
			}
		case s.Seat == 1 && phase == wantTakeover && s.Turn >= 4:
			rep = act(hq, `{"cmd":"takeover","target":1,"on":true}`)
			if rep.Error != "" {
				t.Fatal(rep.Error)
			}
			phase = tookOver
			continue
		}
		acted[s.Seat] = s.Turn
		rep = act(q, `{"cmd":"auto"}`)
		if rep.Error != "" {
			t.Fatalf("座位 %d 第 %d 回合：%s", s.Seat, s.Turn, rep.Error)
		}
		if phase == tookOver && s.Seat == 0 {
			// AI 已經替朋友行動過；朋友自己下指令會被擋，按「我回來了」後恢復
			rep = act(fq, `{"cmd":"play","slot":0}`)
			if !strings.Contains(rep.Error, "AI 正在接手") {
				t.Errorf("AI 接手時朋友還能下指令：%+v", rep)
			}
			rep = act(fq, `{"cmd":"takeover","target":1,"on":false}`)
			if rep.Error != "" {
				t.Fatal(rep.Error)
			}
			phase = resumed
		}
	}
	if phase != resumed {
		t.Fatal("沒測到 AI 接手")
	}
	taken := false
	for _, e := range last.Log {
		if e.Player == 1 && e.Note == "AI 接手" {
			taken = true
		}
	}
	if !taken {
		t.Error("畫面紀錄沒有標出 AI 接手的那一手")
	}
	if last.Seed != "11" {
		t.Errorf("結束後應該公開種子，得到 %q", last.Seed)
	}

	res, err = http.Get(ts.URL + "/api/log" + fq)
	if err != nil {
		t.Fatal(err)
	}
	b := new(strings.Builder)
	bufio.NewReader(res.Body).WriteTo(b)
	res.Body.Close()
	for _, want := range []string{"seed=11 players=3 level=5 variant=none humans=0,1", "AI 接手：", "AI 代打：", "end"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("紀錄裡沒有 %q", want)
		}
	}

	// 同一群人再來一局
	rep = act(fq, `{"cmd":"rematch"}`)
	if rep.Error == "" {
		t.Error("不是房主也能開下一局")
	}
	rep = act(hq, `{"cmd":"rematch","on":true}`)
	if rep.Error != "" || rep.Room == "" {
		t.Fatalf("開下一局失敗：%+v", rep)
	}
	for s := range fs {
		if s.Next != "" {
			if s.Next != rep.Room {
				t.Errorf("朋友收到的下一局 %q，應該是 %q", s.Next, rep.Room)
			}
			break
		}
	}
	ns := stream("?room=" + rep.Room + "&token=" + friend.Token)
	s := <-ns
	if s.Phase != "lobby" || s.Seat != 1 || s.Seats[1].Kind != "human" || s.Seats[2].Kind != "ai" {
		t.Errorf("下一局的等待室不對：%+v", s)
	}
}
