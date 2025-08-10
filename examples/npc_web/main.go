package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	npc "github.com/zeusync/zeusync/internal/core/npc"
)

type Tile int

const (
	Empty Tile = iota
	Trap
	Artifact
	Exit
	Fog
)

func (t Tile) String() string {
	switch t {
	case Empty:
		return "empty"
	case Trap:
		return "trap"
	case Artifact:
		return "artifact"
	case Exit:
		return "exit"
	case Fog:
		return "fog"
	default:
		return "empty"
	}
}

type World struct {
	W, H int
	Grid [][]Tile
}

func NewWorldRandom(w, h int, traps, artifacts, fog int) *World {
	wr := &World{W: w, H: h, Grid: make([][]Tile, h)}
	for y := 0; y < h; y++ {
		wr.Grid[y] = make([]Tile, w)
	}
	// exit
	ex := rand.Intn(w)
	ey := rand.Intn(h)
	wr.Grid[ey][ex] = Exit
	place := func(t Tile, n int) {
		for i := 0; i < n; i++ {
			for {
				x := rand.Intn(w)
				y := rand.Intn(h)
				if wr.Grid[y][x] == Empty {
					wr.Grid[y][x] = t
					break
				}
			}
		}
	}
	place(Trap, traps)
	place(Artifact, artifacts)
	place(Fog, fog)
	return wr
}

func (w *World) InBounds(x, y int) bool { return x >= 0 && y >= 0 && x < w.W && y < w.H }
func (w *World) TileAt(x, y int) Tile {
	if !w.InBounds(x, y) {
		return Empty
	}
	return w.Grid[y][x]
}

func (w *World) ClearTile(x, y int) {
	if w.InBounds(x, y) {
		w.Grid[y][x] = Empty
	}
}

func getInt(bb npc.Blackboard, key string) int {
	v, _ := bb.Get(key)
	switch vv := v.(type) {
	case int:
		return vv
	case int64:
		return int(vv)
	case float64:
		return int(vv)
	default:
		return 0
	}
}
func getBool(bb npc.Blackboard, key string) bool { v, _ := bb.Get(key); b, _ := v.(bool); return b }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

type VisionSensor struct {
	world  *World
	radius int
}

func (s *VisionSensor) Name() string { return "VisionSensor" }

func (s *VisionSensor) Update(ctx context.Context, bb npc.Blackboard) error {
	xi := getInt(bb, "pos_x")
	yi := getInt(bb, "pos_y")
	tile := s.world.TileAt(xi, yi)
	bb.Set("trap_here", tile == Trap)
	bb.Set("artifact_here", tile == Artifact)
	bb.Set("fog_here", tile == Fog)
	exitVisible := false
	nTrap := false
	nArt := false
	nearestArtDist := 1 << 30
	ax, ay := -1, -1
	r := s.radius
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			x := xi + dx
			y := yi + dy
			if !s.world.InBounds(x, y) {
				continue
			}
			switch s.world.TileAt(x, y) {
			case Exit:
				exitVisible = true
				bb.Set("exit_x", x)
				bb.Set("exit_y", y)
			case Trap:
				nTrap = true
			case Artifact:
				nArt = true
				// track nearest artifact position
				d := abs(dx) + abs(dy)
				if d < nearestArtDist {
					nearestArtDist = d
					ax, ay = x, y
				}
			}
		}
	}
	bb.Set("exit_visible", exitVisible)
	bb.Set("nearby_trap", nTrap)
	bb.Set("nearby_artifact", nArt)
	if nArt && ax >= 0 {
		bb.Set("artifact_visible", true)
		bb.Set("artifact_x", ax)
		bb.Set("artifact_y", ay)
	} else {
		bb.Set("artifact_visible", false)
	}
	bb.Set("speed_mod", 1.0+float64(getInt(bb, "energy"))*0.1)
	return nil
}

type ActionWrap struct {
	name string
	fn   func(t npc.TickContext) (npc.Status, error)
}

func (a ActionWrap) Name() string                               { return a.name }
func (a ActionWrap) Tick(t npc.TickContext) (npc.Status, error) { return a.fn(t) }

type CondWrap struct {
	name string
	fn   func(t npc.TickContext) (bool, error)
}

func (c CondWrap) Name() string { return c.name }
func (c CondWrap) Tick(t npc.TickContext) (npc.Status, error) {
	ok, err := c.fn(t)
	if err != nil {
		return npc.StatusFailure, err
	}
	if ok {
		return npc.StatusSuccess, nil
	}
	return npc.StatusFailure, nil
}

func MoveSmartAction(name string, world *World, tx, ty string) npc.Action {
	return ActionWrap{name: name, fn: func(t npc.TickContext) (npc.Status, error) {
		cx := getInt(t.BB, "pos_x")
		cy := getInt(t.BB, "pos_y")
		gx := getInt(t.BB, tx)
		gy := getInt(t.BB, ty)
		if cx == gx && cy == gy {
			return npc.StatusSuccess, nil
		}
		// consider 4 neighbors; prefer reducing distance; avoid remembered danger and fog when possible
		type cand struct {
			x, y  int
			score int
		}
		best := cand{x: cx, y: cy, score: 1 << 30}
		dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if !world.InBounds(nx, ny) {
				continue
			}
			if getBool(t.BB, fmt.Sprintf("danger_%d_%d", nx, ny)) {
				continue // don't step into known danger
			}
			dist := abs(gx-nx) + abs(gy-ny)
			fogPenalty := 0
			if world.TileAt(nx, ny) == Fog {
				fogPenalty = 3 // avoid fog when possible
			}
			s := dist*10 + fogPenalty
			if s < best.score {
				best = cand{x: nx, y: ny, score: s}
			}
		}
		// if no neighbor chosen (all blocked), fall back to staying or random
		if best.score == 1<<30 {
			// try random non-danger in-bounds
			for _, d := range dirs {
				nx, ny := cx+d[0], cy+d[1]
				if world.InBounds(nx, ny) && !getBool(t.BB, fmt.Sprintf("danger_%d_%d", nx, ny)) {
					best = cand{x: nx, y: ny, score: 0}
					break
				}
			}
		}
		// slow down additionally if in fog (cautious steps)
		if getBool(t.BB, "fog_here") && rand.Float64() < 0.5 {
			return npc.StatusRunning, nil
		}
		t.BB.Set("pos_x", best.x)
		t.BB.Set("pos_y", best.y)
		return npc.StatusRunning, nil
	}}
}

func ExploreAction(name string, world *World) npc.Action {
	return ActionWrap{name: name, fn: func(t npc.TickContext) (npc.Status, error) {
		x := getInt(t.BB, "pos_x")
		y := getInt(t.BB, "pos_y")
		v, _ := t.BB.Get("visited")
		vm, _ := v.(map[[2]int]bool)
		if vm == nil {
			vm = map[[2]int]bool{}
		}
		vm[[2]int{x, y}] = true
		// BFS to nearest unvisited and non-danger cell. First pass avoids fog cells if possible.
		dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		var target [2]int
		found := false
		parents := make(map[[2]int][2]int)
		start := [2]int{x, y}
		bfs := func(allowFog bool) bool {
			parents = make(map[[2]int][2]int)
			seen := make(map[[2]int]bool)
			queue := make([][2]int, 0)
			queue = append(queue, start)
			seen[start] = true
			for qi := 0; qi < len(queue); qi++ {
				cur := queue[qi]
				for _, d := range dirs {
					nx, ny := cur[0]+d[0], cur[1]+d[1]
					np := [2]int{nx, ny}
					if !world.InBounds(nx, ny) || seen[np] {
						continue
					}
					if getBool(t.BB, fmt.Sprintf("danger_%d_%d", nx, ny)) {
						continue
					}
					if world.TileAt(nx, ny) == Fog && !allowFog {
						continue
					}
					seen[np] = true
					parents[np] = cur
					if !vm[np] {
						target = np
						return true
					}
					queue = append(queue, np)
				}
			}
			return false
		}
		if bfs(false) || bfs(true) {
			found = true
		}
		// if not found, fall back to any valid neighbor to keep moving
		var next [2]int
		if found {
			// backtrack one step from target to start
			curr := target
			for {
				prev, ok := parents[curr]
				if !ok || prev == start {
					next = curr
					break
				}
				curr = prev
			}
		} else {
			for _, d := range dirs {
				nx, ny := x+d[0], y+d[1]
				if world.InBounds(nx, ny) && !getBool(t.BB, fmt.Sprintf("danger_%d_%d", nx, ny)) {
					next = [2]int{nx, ny}
					break
				}
			}
		}
		x, y = next[0], next[1]
		t.BB.Set("visited", vm)
		t.BB.Set("pos_x", x)
		t.BB.Set("pos_y", y)
		return npc.StatusRunning, nil
	}}
}

func PickupArtifactAction(world *World) npc.Action {
	return ActionWrap{name: "PickupArtifact", fn: func(t npc.TickContext) (npc.Status, error) {
		x := getInt(t.BB, "pos_x")
		y := getInt(t.BB, "pos_y")
		if world.TileAt(x, y) != Artifact {
			return npc.StatusFailure, nil
		}
		energy := getInt(t.BB, "energy") + 1
		t.BB.Set("energy", energy)
		world.ClearTile(x, y)
		return npc.StatusSuccess, nil
	}}
}

func RememberTrapAction(world *World) npc.Action {
	return ActionWrap{name: "RememberTrap", fn: func(t npc.TickContext) (npc.Status, error) {
		x := getInt(t.BB, "pos_x")
		y := getInt(t.BB, "pos_y")
		if world.TileAt(x, y) == Trap {
			hp := getInt(t.BB, "hp")
			if hp <= 0 {
				return npc.StatusFailure, nil
			}
			t.BB.Set("hp", hp-1)
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if world.InBounds(nx, ny) && world.TileAt(nx, ny) == Trap {
					t.BB.Set(fmt.Sprintf("danger_%d_%d", nx, ny), true)
				}
			}
		}
		return npc.StatusSuccess, nil
	}}
}

func ConditionKeyBool(name, key string) npc.Condition {
	return CondWrap{name: name, fn: func(t npc.TickContext) (bool, error) { return getBool(t.BB, key), nil }}
}

func IsAtExitCondition(world *World) npc.Condition {
	return CondWrap{name: "AtExit", fn: func(t npc.TickContext) (bool, error) {
		return world.TileAt(getInt(t.BB, "pos_x"), getInt(t.BB, "pos_y")) == Exit, nil
	}}
}

type simpleTree struct{ r npc.BehaviorNode }

func (t simpleTree) Root() npc.BehaviorNode { return t.r }
func (t simpleTree) Tick(tc npc.TickContext) (npc.Status, error) {
	if t.r == nil {
		return npc.StatusSuccess, nil
	}
	return t.r.Tick(tc)
}

type Game struct {
	mu             sync.RWMutex
	world          *World
	agent          npc.Agent
	bb             npc.Blackboard
	mem            npc.Memory
	startTime      time.Time
	pendingRestart bool
}

func (g *Game) build() {
	w := NewWorldRandom(22, 14, 30, 7, 24)
	bb := npc.NewBlackboard()
	mem := npc.NewMemory()
	eb := npc.NewEventBus()
	bb.Set("pos_x", 0)
	bb.Set("pos_y", 0)
	bb.Set("hp", 5)
	bb.Set("energy", 0)
	sensors := []npc.Sensor{&VisionSensor{world: w, radius: 4}}
	isExit := ConditionKeyBool("ExitVisible", "exit_visible")
	atExit := IsAtExitCondition(w)
	moveExit := MoveSmartAction("MoveToExit", w, "exit_x", "exit_y")
	moveArtifact := MoveSmartAction("MoveToArtifact", w, "artifact_x", "artifact_y")
	pickup := PickupArtifactAction(w)
	remember := RememberTrapAction(w)
	explore := ExploreAction("Explore", w)
	// cooldowns slowed to 1000ms
	cd := npc.NewCooldown("MoveCD", 1000*time.Millisecond, false)
	cd.SetChild(moveExit)
	ec := npc.NewCooldown("ExploreCD", 1000*time.Millisecond, false)
	ec.SetChild(explore)
	// behavior branches
	toExit := npc.NewSequence("ToExit", isExit, npc.NewSelector("MoveOrDone",
		ActionWrap{name: "CheckAtExit", fn: func(t npc.TickContext) (npc.Status, error) {
			if st, _ := atExit.Tick(t); st == npc.StatusSuccess {
				return npc.StatusSuccess, nil
			}
			return npc.StatusFailure, nil
		}},
		cd,
	))
	artifactVisible := ConditionKeyBool("ArtifactVisible", "artifact_visible")
	toArtifact := npc.NewSequence("ToArtifact", artifactVisible, moveArtifact)
	artifactHere := ConditionKeyBool("ArtifactHere", "artifact_here")
	nearTrap := ConditionKeyBool("NearTrap", "nearby_trap")
	trapHere := ConditionKeyBool("TrapHere", "trap_here")
	collect := npc.NewSequence("Collect", artifactHere, pickup)
	trapImpact := npc.NewSequence("TrapImpact", trapHere, remember)
	safety := npc.NewSequence("Safety", nearTrap, remember)
	root := npc.NewSelector("Root", toExit, trapImpact, collect, toArtifact, ec, safety)
	g.world = w
	g.bb = bb
	g.mem = mem
	g.agent = npc.NewAgent(bb, mem, eb, simpleTree{r: root}, sensors)
}

func (g *Game) Restart() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.build()
	g.startTime = time.Now()
	g.pendingRestart = false
}

// Websocket hub

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}
type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func newHub() *wsHub                { return &wsHub{clients: map[*wsClient]struct{}{}} }
func (h *wsHub) add(c *wsClient)    { h.mu.Lock(); h.clients[c] = struct{}{}; h.mu.Unlock() }
func (h *wsHub) remove(c *wsClient) { h.mu.Lock(); delete(h.clients, c); h.mu.Unlock() }
func (h *wsHub) broadcast(b []byte) {
	h.mu.RLock()
	for c := range h.clients {
		select {
		case c.send <- b:
		default:
		}
	}
	h.mu.RUnlock()
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type TickFrame struct {
	Tick       int        `json:"tick"`
	Elapsed    int        `json:"elapsed_ticks"`
	Status     string     `json:"status"`
	Outcome    string     `json:"outcome,omitempty"`
	PosX       int        `json:"pos_x"`
	PosY       int        `json:"pos_y"`
	HP         int        `json:"hp"`
	Energy     int        `json:"energy"`
	ExitVis    bool       `json:"exit_visible"`
	Remembered int        `json:"remembered_traps"`
	Events     []string   `json:"events,omitempty"`
	Grid       [][]string `json:"grid"`
}

func main() {
	rand.Seed(time.Now().UnixNano())
	game := &Game{}
	game.Restart()
	hub := newHub()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		cl := &wsClient{conn: c, send: make(chan []byte, 64)}
		hub.add(cl)
		defer func() { hub.remove(cl); c.Close() }()
		go func() {
			for {
				_, msg, err := c.ReadMessage()
				if err != nil {
					return
				}
				if string(msg) == "restart" {
					game.Restart()
				}
			}
		}()
		for b := range cl.send {
			if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		}
	})

	http.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		game.Restart()
		w.WriteHeader(204)
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("examples/npc_web/static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "examples/npc_web/static/index.html")
	})

	go func() {
		tick := 0
		var prevHP, prevEnergy int
		var lastStart time.Time
		for {
			game.mu.RLock()
			bb := game.bb
			world := game.world
			agent := game.agent
			start := game.startTime
			game.mu.RUnlock()
			st, err := agent.Step(context.Background())
			if err != nil {
				log.Println("step:", err)
			}
			grid := make([][]string, world.H)
			for y := 0; y < world.H; y++ {
				grid[y] = make([]string, world.W)
				for x := 0; x < world.W; x++ {
					grid[y][x] = world.Grid[y][x].String()
				}
			}
			// compute metrics
			keys := bb.Keys()
			rem := 0
			for _, k := range keys {
				if strings.HasPrefix(k, "danger_") {
					rem++
				}
			}
			px := getInt(bb, "pos_x")
			py := getInt(bb, "pos_y")
			hp := getInt(bb, "hp")
			energy := getInt(bb, "energy")
			// reset prev on restart
			if start != lastStart {
				prevHP = hp
				prevEnergy = energy
				lastStart = start
			}
			events := make([]string, 0, 2)
			if dh := prevHP - hp; dh > 0 {
				events = append(events, fmt.Sprintf("-%d HP (trap)", dh))
			}
			if de := energy - prevEnergy; de > 0 {
				events = append(events, fmt.Sprintf("+%d energy (artifact)", de))
			}
			prevHP = hp
			prevEnergy = energy
			outcome := ""
			if world.TileAt(px, py) == Exit {
				outcome = "success"
			} else if hp <= 0 {
				outcome = "dead"
				// schedule auto-restart once
				game.mu.Lock()
				if !game.pendingRestart {
					game.pendingRestart = true
					go func() {
						time.Sleep(1 * time.Second)
						game.Restart()
					}()
				}
				game.mu.Unlock()
			}
			elapsed := int(time.Since(start) / (80 * time.Millisecond))
			frame := TickFrame{Tick: tick, Elapsed: elapsed, Status: fmt.Sprintf("%v", st), Outcome: outcome, PosX: px, PosY: py, HP: hp, Energy: energy, ExitVis: getBool(bb, "exit_visible"), Remembered: rem, Events: events, Grid: grid}
			b, _ := json.Marshal(frame)
			hub.broadcast(b)
			tick++
			time.Sleep(80 * time.Millisecond)
		}
	}()

	log.Println("NPC web demo: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
