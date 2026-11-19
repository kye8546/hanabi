// Package web 是花火的網頁介面：HTTP 伺服器（房間、指令、用 Server-Sent Events 推送畫面，
// 串流被代理伺服器卡住時改用長輪詢）與內嵌的前端。
// 遊戲狀態只在伺服器上，每個人只收到自己座位的 PlayerView，所以看不到自己的手牌。
// 一個房間可以有好幾位真人（房主開房、朋友用邀請連結坐進空位），其他座位由 AI 坐。
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
	rooms map[string]*room
}

// New 建立伺服器：對局紀錄寫到 logDir，AI 每一手至少間隔 aiDelay。
func New(logDir string, aiDelay time.Duration) *Server {
	s := &Server{logDir: logDir, aiDelay: aiDelay, rooms: map[string]*room{}}
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
	mux.HandleFunc("GET /api/room", s.roomInfo)
	mux.HandleFunc("POST /api/join", s.join)
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
		for id, r := range s.rooms {
			r.mu.Lock()
			idle := time.Since(r.active)
			expired := idle > idleTimeout || (r.done && idle > doneTimeout)
			r.mu.Unlock()
			if expired {
				r.cancel()
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

// roomCode 是隨機四位數的房號（朋友在首頁輸入它加入）。房間最多 maxRooms 個，撞號就重抽。
func roomCode() string {
	var b [2]byte
	rand.Read(b[:])
	return fmt.Sprintf("%04d", (int(b[0])<<8|int(b[1]))%10000)
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
		Name    string   `json:"name"`
		Players int      `json:"players"`
		Level   int      `json:"level"`
		Variant string   `json:"variant"`
		Seed    string   `json:"seed"`  // 空的 = 隨機
		Seats   []string `json:"seats"` // 座位 1 起：ai / human（等朋友加入）；沒給 = 全是 AI
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
	if in.Players < game.MinPlayers || in.Players > game.MaxPlayers {
		writeJSON(w, http.StatusBadRequest, reply{Error: fmt.Sprintf("人數只能是 %d–%d", game.MinPlayers, game.MaxPlayers)})
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
	cfg := roomConfig{n: in.Players, level: in.Level, va: va, seed: seed,
		kinds: make([]seatKind, in.Players), names: make([]string, in.Players), tokens: map[string]int{}}
	cfg.kinds[0] = seatHuman
	solo := true
	for i := 1; i < in.Players; i++ {
		cfg.kinds[i] = seatAI
		if i-1 < len(in.Seats) && in.Seats[i-1] == "human" {
			cfg.kinds[i] = seatOpen
			solo = false
		}
	}
	if solo {
		cfg.names[0] = cleanName(in.Name, "你")
	} else {
		cfg.names[0] = cleanName(in.Name, "房主")
	}
	token := randomHex(16)
	cfg.tokens[token] = 0
	r, err := s.open(cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: err.Error()})
		return
	}
	if solo {
		if err := r.start(); err != nil {
			writeJSON(w, http.StatusInternalServerError, reply{Error: err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"room": r.id, "token": token})
}

// open 開一個新房間（還沒開始）。
func (s *Server) open(cfg roomConfig) (*room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rooms) >= maxRooms {
		return nil, fmt.Errorf("伺服器目前太忙，請稍後再試")
	}
	id := roomCode()
	for s.rooms[id] != nil {
		id = roomCode()
	}
	r, err := newRoom(id, cfg, s.logDir, s.aiDelay)
	if err != nil {
		return nil, err
	}
	s.rooms[id] = r
	return r, nil
}

func (s *Server) find(id string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rooms[id]
}

const notFound = "找不到這局（可能已過期或伺服器重新啟動過）"

// roomInfo 是輸入房號（或打開邀請連結）時看到的資訊，不用通行碼：設定與座位。
// 帶著這個瀏覽器之前在這間房的通行碼時，seat 是他的座位（-1 = 沒有，例如房號已經換成別的房間）。
func (s *Server) roomInfo(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	r := s.find(q.Get("room"))
	if r == nil {
		writeJSON(w, http.StatusNotFound, reply{Error: "沒有這個房號（可能打錯，或那局已經關閉）"})
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	info := lobbySnapshot(r.va, r.level, r.seed)
	info.Seed = ""
	info.Seat = -1
	if seat, ok := r.tokens[q.Get("token")]; ok {
		info.Seat = seat
	}
	if r.started {
		info.Phase = "game"
	}
	for s := range r.n {
		info.Seats = append(info.Seats, seatJSON{Name: r.names[s], Kind: kindNames[r.kinds[s]], Online: r.online(s)})
	}
	writeJSON(w, http.StatusOK, info)
}

// join 讓朋友用邀請連結坐進空位。
func (s *Server) join(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Room string `json:"room"`
		Name string `json:"name"`
		Seat int    `json:"seat"` // -1 = 第一個空位
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4096)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: "看不懂"})
		return
	}
	r := s.find(in.Room)
	if r == nil {
		writeJSON(w, http.StatusNotFound, reply{Error: notFound})
		return
	}
	token, seat, err := r.join(in.Seat, in.Name)
	if err != nil {
		writeJSON(w, http.StatusConflict, reply{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": r.id, "token": token, "seat": seat})
}

// lookup 依網址參數 room、token 找房間與座位；找不到時已回應錯誤。
func (s *Server) lookup(w http.ResponseWriter, req *http.Request) (*room, int) {
	q := req.URL.Query()
	r := s.find(q.Get("room"))
	if r == nil {
		writeJSON(w, http.StatusNotFound, reply{Error: notFound})
		return nil, -1
	}
	seat := r.seatOf(q.Get("token"))
	if seat < 0 {
		writeJSON(w, http.StatusNotFound, reply{Error: "你已經不在這局的座位上了"})
		return nil, -1
	}
	return r, seat
}

// current 回傳座位現在的畫面與版本；座位已經不屬於這個通行碼（離開、被房主請出去）時 ok 為 false。
func (r *room) current(token string, seat int) (b []byte, ver int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, in := r.tokens[token]; !in || s != seat {
		return nil, r.ver, false
	}
	return r.snaps[seat], r.ver, true
}

// stream 用 Server-Sent Events 推送畫面：連上時先送目前的畫面，之後每次變化都送一次完整畫面。
func (s *Server) stream(w http.ResponseWriter, req *http.Request) {
	r, seat := s.lookup(w, req)
	if r == nil {
		return
	}
	token := req.URL.Query().Get("token")
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支援串流", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // 反向代理（nginx 等）不要緩衝

	ch := make(chan struct{}, 1)
	r.connect(seat, ch)
	defer r.disconnect(seat, ch)

	sent := -1
	send := func() bool {
		b, ver, ok := r.current(token, seat)
		if !ok {
			fmt.Fprint(w, "event: gone\ndata: {\"reason\":\"left\"}\n\n")
			fl.Flush()
			return false
		}
		if b != nil && ver != sent {
			sent = ver
			fmt.Fprintf(w, "event: state\ndata: %s\n\n", b)
			fl.Flush()
		}
		return true
	}
	if !send() {
		return
	}
	ping := time.NewTicker(20 * time.Second) // 讓代理伺服器不要因為閒置切斷連線
	defer ping.Stop()
	for {
		select {
		case <-req.Context().Done():
			return
		case <-r.ctx.Done():
			fmt.Fprint(w, "event: gone\ndata: {\"reason\":\"closed\"}\n\n")
			fl.Flush()
			return
		case <-ch:
			if !send() {
				return
			}
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
// 回傳 {"ver":N,"state":畫面}；房間關閉時回傳 {"gone":"closed"}，離開座位時 {"gone":"left"}。
func (s *Server) poll(w http.ResponseWriter, req *http.Request) {
	r, seat := s.lookup(w, req)
	if r == nil {
		return
	}
	token := req.URL.Query().Get("token")
	since, _ := strconv.Atoi(req.URL.Query().Get("since"))
	ch := make(chan struct{}, 1)
	r.connect(seat, ch)
	defer r.disconnect(seat, ch)

	timeout := time.NewTimer(pollWait)
	defer timeout.Stop()
	for {
		b, ver, ok := r.current(token, seat)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]string{"gone": "left"})
			return
		}
		if ver != since && b != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprintf(w, `{"ver":%d,"state":%s}`, ver, b)
			return
		}
		select {
		case <-ch:
		case <-r.ctx.Done():
			writeJSON(w, http.StatusOK, map[string]string{"gone": "closed"})
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
	r, seat := s.lookup(w, req)
	if r == nil {
		return
	}
	var c command
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4096)).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: "看不懂指令"})
		return
	}
	r.mu.Lock()
	started := r.started
	r.mu.Unlock()
	switch {
	case c.Cmd == "rematch":
		writeJSON(w, http.StatusOK, s.rematch(r, seat, c))
	case !started:
		writeJSON(w, http.StatusOK, r.lobby(seat, c))
	case c.Cmd == "takeover":
		writeJSON(w, http.StatusOK, r.takeover(seat, c))
	default:
		writeJSON(w, http.StatusOK, r.send(seat, c))
	}
}

// rematch：多人局結束後，房主替同一群人（同樣的座位、設定）開下一局的等待室，
// 大家的通行碼照用，畫面自動換過去。c.On = 用同一副牌。
func (s *Server) rematch(r *room, seat int, c command) reply {
	if seat != 0 {
		return reply{Error: "只有房主能開下一局"}
	}
	r.mu.Lock()
	if !r.done {
		r.mu.Unlock()
		return reply{Error: "這局還沒結束"}
	}
	if r.next != "" {
		r.mu.Unlock()
		return reply{OK: true, Room: r.next}
	}
	cfg := roomConfig{n: r.n, level: r.level, va: r.va, seed: r.seed,
		kinds: append([]seatKind(nil), r.kinds...), names: append([]string(nil), r.names...), tokens: map[string]int{}}
	for tok, st := range r.tokens {
		cfg.tokens[tok] = st
	}
	r.mu.Unlock()
	if !c.On {
		cfg.seed = uint64(time.Now().UnixNano())
	}
	nr, err := s.open(cfg)
	if err != nil {
		return reply{Error: err.Error()}
	}
	r.mu.Lock()
	r.next = nr.id
	r.render()
	r.mu.Unlock()
	return reply{OK: true, Room: nr.id}
}

func (s *Server) downloadLog(w http.ResponseWriter, req *http.Request) {
	r, _ := s.lookup(w, req)
	if r == nil {
		return
	}
	r.mu.Lock()
	gl := r.log
	r.mu.Unlock()
	if gl == nil {
		writeJSON(w, http.StatusNotFound, reply{Error: "還沒開始，沒有紀錄"})
		return
	}
	b, err := os.ReadFile(gl.Path())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, reply{Error: "讀不到對局紀錄"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hanabi-%s.log"`, r.id))
	w.Write(b)
}
