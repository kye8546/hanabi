package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
		if !s.YourTurn {
			continue
		}
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
