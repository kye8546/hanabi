// Package web 是花火的網頁介面：HTTP 伺服器（房間、指令、用 Server-Sent Events 推送畫面，
// 串流被代理伺服器卡住時改用長輪詢）與內嵌的前端。
// 遊戲狀態只在伺服器上，每個人只收到自己的 PlayerView，所以看不到自己的手牌（之後的多人連線也靠這點）。
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"hanabi/ai/hgroup"
	"hanabi/game"
)

//go:embed static
var staticFiles embed.FS

const (
	maxRooms    = 200              // 同時進行的局數上限
	idleTimeout = 3 * time.Hour    // 沒人連線、沒有指令多久之後關掉房間
	doneTimeout = 30 * time.Minute // 結束的局保留多久（看結果、下載紀錄）
)

// Server 管理所有房間。
type Server struct {
	logDir  string
	aiDelay time.Duration

	mu    sync.Mutex
	rooms map[string]*entry
}

type entry struct {
	r     *room
	token string // 真人座位的通行碼（之後多人時每個座位一個）
}

// New 建立伺服器：對局紀錄寫到 logDir，AI 每一手至少間隔 aiDelay。
func New(logDir string, aiDelay time.Duration) *Server {
	s := &Server{logDir: logDir, aiDelay: aiDelay, rooms: map[string]*entry{}}
	go s.janitor()
	return s
}

// Handler 回傳所有路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFiles, "static")
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("POST /api/new", s.newGame)
	mux.HandleFunc("GET /api/stream", s.stream)
	mux.HandleFunc("GET /api/poll", s.poll)
	mux.HandleFunc("POST /api/act", s.act)
	mux.HandleFunc("GET /api/log", s.downloadLog)
	return mux
}

// janitor 定期關掉閒置太久或結束太久的房間。
func (s *Server) janitor() {
	for range time.Tick(5 * time.Minute) {
		s.mu.Lock()
		for id, e := range s.rooms {
			e.r.mu.Lock()
			idle := time.Since(e.r.active)
			expired := idle > idleTimeout || (e.r.done && idle > doneTimeout)
			e.r.mu.Unlock()
			if expired {
				e.r.cancel()
				delete(s.rooms, id)
			}
		}
		s.mu.Unlock()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// variantDescs 是開局選單的說明（順序同 game.Variants）。
var variantDescs = map[game.Variant]string{
	game.NoVariant:  "標準 5 色，滿分 25",
	game.SixSuits:   "多一個青色（T），每色 10 張、共 60 張，滿分 30",
	game.BlackSix:   "多一個黑色（K），每個數字只有 1 張（全都是 critical）、共 55 張，滿分 30",
	game.RainbowSix: "多一個彩虹色（M），每種顏色提示都會點到它、不能用彩虹色提示，共 60 張，滿分 30",
}

func (s *Server) config(w http.ResponseWriter, _ *http.Request) {
	type level struct {
		Level     int    `json:"level"`
		Name      string `json:"name"`
		Available bool   `json:"available"`
	}
	type variant struct {
		Code string `json:"code"`
		Name string `json:"name"`
		Desc string `json:"desc"`
	}
	var cfg struct {
		Levels     []level   `json:"levels"`
		Variants   []variant `json:"variants"`
		MinPlayers int       `json:"minPlayers"`
		MaxPlayers int       `json:"maxPlayers"`
	}
	for i, name := range hgroup.Levels {
		cfg.Levels = append(cfg.Levels, level{i + 1, name, i+1 <= hgroup.MaxLevel})
	}
	for _, v := range game.Variants {
		cfg.Variants = append(cfg.Variants, variant{v.Code(), v.Name(), variantDescs[v]})
	}
	cfg.MinPlayers, cfg.MaxPlayers = game.MinPlayers, game.MaxPlayers
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) newGame(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Players int    `json:"players"`
		Level   int    `json:"level"`
		Variant string `json:"variant"`
		Seed    string `json:"seed"` // 空的 = 隨機
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4096)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: "看不懂開局設定"})
		return
	}
	if in.Level < 1 || in.Level > hgroup.MaxLevel {
		writeJSON(w, http.StatusBadRequest, reply{Error: fmt.Sprintf("慣例等級只能選 Lv1–Lv%d", hgroup.MaxLevel)})
		return
	}
	va, ok := game.ParseVariant(in.Variant)
	if !ok {
		writeJSON(w, http.StatusBadRequest, reply{Error: "沒有這個變體"})
		return
	}
	var seed uint64
	if in.Seed != "" {
		n, err := strconv.ParseUint(in.Seed, 10, 64)
		if err != nil || n == 0 {
			writeJSON(w, http.StatusBadRequest, reply{Error: "種子要是正整數"})
			return
		}
		seed = n
	} else {
		seed = uint64(time.Now().UnixNano())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rooms) >= maxRooms {
		writeJSON(w, http.StatusServiceUnavailable, reply{Error: "伺服器目前太忙，請稍後再試"})
		return
	}
	id := randomHex(6)
	r, err := newRoom(id, in.Players, in.Level, va, seed, cleanName(in.Name), s.logDir, s.aiDelay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: err.Error()})
		return
	}
	e := &entry{r: r, token: randomHex(16)}
	s.rooms[id] = e
	go r.run()
	writeJSON(w, http.StatusOK, map[string]string{"room": id, "token": e.token})
}

// lookup 依網址參數 room、token 找房間；找不到時已回應錯誤。
func (s *Server) lookup(w http.ResponseWriter, req *http.Request) *room {
	q := req.URL.Query()
	s.mu.Lock()
	e := s.rooms[q.Get("room")]
	s.mu.Unlock()
	if e == nil || q.Get("token") != e.token {
		writeJSON(w, http.StatusNotFound, reply{Error: "找不到這局（可能已過期或伺服器重新啟動過）"})
		return nil
	}
	return e.r
}

// stream 用 Server-Sent Events 推送畫面：連上時先送目前的畫面，之後每次變化都送一次完整畫面。
func (s *Server) stream(w http.ResponseWriter, req *http.Request) {
	r := s.lookup(w, req)
	if r == nil {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支援串流", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // 反向代理（nginx 等）不要緩衝

	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.subs[ch] = true
	r.active = time.Now()
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.active = time.Now()
		r.mu.Unlock()
	}()

	send := func() {
		r.mu.Lock()
		b := r.snap
		r.active = time.Now()
		r.mu.Unlock()
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", b)
		fl.Flush()
	}
	send()
	ping := time.NewTicker(20 * time.Second) // 讓代理伺服器不要因為閒置切斷連線
	defer ping.Stop()
	for {
		select {
		case <-req.Context().Done():
			return
		case <-r.ctx.Done():
			fmt.Fprint(w, "event: gone\ndata: {}\n\n")
			fl.Flush()
			return
		case <-ch:
			send()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

// pollWait 是長輪詢最多等多久（要比代理伺服器的逾時短；Cloudflare 是 100 秒）。
const pollWait = 25 * time.Second

// poll 是 SSE 的備用方案（Cloudflare 的免帳號通道等代理伺服器會把串流整個緩衝住）：
// 畫面版本和 since 不同就立刻回傳，否則等到畫面變化或 pollWait 到期。
// 回傳 {"ver":N,"state":畫面}；房間關閉時回傳 {"gone":true}。
func (s *Server) poll(w http.ResponseWriter, req *http.Request) {
	r := s.lookup(w, req)
	if r == nil {
		return
	}
	since, _ := strconv.Atoi(req.URL.Query().Get("since"))
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.active = time.Now()
	r.subs[ch] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}()

	timeout := time.NewTimer(pollWait)
	defer timeout.Stop()
	for {
		r.mu.Lock()
		ver, b := r.ver, r.snap
		r.mu.Unlock()
		if ver != since {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprintf(w, `{"ver":%d,"state":%s}`, ver, b)
			return
		}
		select {
		case <-ch:
		case <-r.ctx.Done():
			writeJSON(w, http.StatusOK, map[string]bool{"gone": true})
			return
		case <-timeout.C:
			w.WriteHeader(http.StatusNoContent)
			return
		case <-req.Context().Done():
			return
		}
	}
}

func (s *Server) act(w http.ResponseWriter, req *http.Request) {
	r := s.lookup(w, req)
	if r == nil {
		return
	}
	var c command
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4096)).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: "看不懂指令"})
		return
	}
	writeJSON(w, http.StatusOK, r.send(c))
}

func (s *Server) downloadLog(w http.ResponseWriter, req *http.Request) {
	r := s.lookup(w, req)
	if r == nil {
		return
	}
	b, err := os.ReadFile(r.log.Path())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, reply{Error: "讀不到對局紀錄"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hanabi-%s.log"`, r.id))
	w.Write(b)
}
