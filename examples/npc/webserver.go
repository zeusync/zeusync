//go:build npc_web_standalone

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	npc "github.com/zeusync/zeusync/internal/core/npc"
)

// --- World model (randomized) ---

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
	// place exit
	ex := rand.Intn(w)
	ey := rand.Intn(h)
	wr.Grid[ey][ex] = Exit
	// place traps
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

// --- Helpers to read from blackboard ---

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

func getBool(bb npc.Blackboard, key string) bool {
	v, _ := bb.Get(key)
	b, _ := v.(bool)
	return b
}

// --- Sensor ---

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
	// look around
	exitVisible := false
	nTrap := false
	nArt := false
	r := s.radius
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			x := xi + dx
			y := yi + dy
			if !s.world.InBounds(x, y) {
				continue
			}
			t := s.world.TileAt(x, y)
			switch t {
			case Exit:
				exitVisible = true
				bb.Set("exit_x", x)
				bb.Set("exit_y", y)
			case Trap:
				nTrap = true
			case Artifact:
				nArt = true
			}
		}
	}
	bb.Set("exit_visible", exitVisible)
	bb.Set("nearby_trap", nTrap)
	bb.Set("nearby_artifact", nArt)
	// speed modifier from energy
	energy := getInt(bb, "energy")
	bb.Set("speed_mod", 1.0+float64(energy)*0.1)
	return nil
}

// --- Actions and Conditions ---

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

func MoveTowardsAction(name, targetXKey, targetYKey string) npc.Action {
	return ActionWrap{name: name, fn: func(t npc.TickContext) (npc.Status, error) {
		curX := getInt(t.BB, "pos_x")
		curY := getInt(t.BB, "pos_y")
		gx := getInt(t.BB, targetXKey)
		gy := getInt(t.BB, targetYKey)
		if curX == gx && curY == gy {
			return npc.StatusSuccess, nil
		}
		if gx > curX {
			curX++
		} else if gx < curX {
			curX--
		}
		if gy > curY {
			curY++
		} else if gy < curY {
			curY--
		}
		if getBool(t.BB, "fog_here") {
			if rand.Float64() < 0.4 {
				return npc.StatusRunning, nil
			}
		}
		t.BB.Set("pos_x", curX)
		t.BB.Set("pos_y", curY)
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
			vm = make(map[[2]int]bool)
		}
		vm[[2]int{x, y}] = true
		dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		cand := make([][2]int, 0, 4)
		for _, d := range dirs {
			nx, ny := x+d[0], y+d[1]
			if !world.InBounds(nx, ny) {
				continue
			}
			if getBool(t.BB, fmt.Sprintf("danger_%d_%d", nx, ny)) {
				continue
			}
			if vm[[2]int{nx, ny}] {
				continue
			}
			cand = append(cand, [2]int{nx, ny})
		}
		if len(cand) == 0 {
			vm = map[[2]int]bool{{x, y}: true}
			d := dirs[rand.Intn(len(dirs))]
			nx, ny := x+d[0], y+d[1]
			if world.InBounds(nx, ny) {
				x, y = nx, ny
			}
		} else {
			c := cand[rand.Intn(len(cand))]
			x, y = c[0], c[1]
		}
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
		energy := getInt(t.BB, "energy")
		energy++
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
			hp -= 1
			t.BB.Set("hp", hp)
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
	return CondWrap{name: name, fn: func(t npc.TickContext) (bool, error) {
		return getBool(t.BB, key), nil
	}}
}

func IsAtExitCondition(world *World) npc.Condition {
	return CondWrap{name: "AtExit", fn: func(t npc.TickContext) (bool, error) {
		x := getInt(t.BB, "pos_x")
		y := getInt(t.BB, "pos_y")
		return world.TileAt(x, y) == Exit, nil
	}}
}

// --- Decision tree wrapper ---

type simpleTree struct{ r npc.BehaviorNode }

func (t simpleTree) Root() npc.BehaviorNode { return t.r }
func (t simpleTree) Tick(tc npc.TickContext) (npc.Status, error) {
	if t.r == nil {
		return npc.StatusSuccess, nil
	}
	return t.r.Tick(tc)
}

// --- Runtime (agent + world) with control for restart ---

type Game struct {
	mu      sync.RWMutex
	world   *World
	agent   npc.Agent
	bb      npc.Blackboard
	mem     npc.Memory
	stopCh  chan struct{}
	stopped bool
}

func NewGame() *Game { return &Game{} }

func (g *Game) buildAgentLocked() {
	w := NewWorldRandom(20, 12, 25, 6, 18)
	bb := npc.NewBlackboard()
	mem := npc.NewMemory()
	eb := npc.NewEventBus()
	// init state
	bb.Set("pos_x", 0)
	bb.Set("pos_y", 0)
	bb.Set("hp", 5)
	bb.Set("energy", 0)
	// sensors
	sensors := []npc.Sensor{&VisionSensor{world: w, radius: 4}}
	// tree
	isExitVisible := ConditionKeyBool("ExitVisible", "exit_visible")
	atExit := IsAtExitCondition(w)
	moveToExit := MoveTowardsAction("MoveToExit", "exit_x", "exit_y")
	pickup := PickupArtifactAction(w)
	rememberTrap := RememberTrapAction(w)
	explore := ExploreAction("Explore", w)

	cd := npc.NewCooldown("MoveCooldown", 180*time.Millisecond, false)
	cd.SetChild(moveToExit)

	toExit := npc.NewSequence("ToExit", isExitVisible, npc.NewSelector("MoveOrDone",
		ActionWrap{name: "CheckAtExit", fn: func(t npc.TickContext) (npc.Status, error) {
			if st, _ := atExit.Tick(t); st == npc.StatusSuccess {
				return npc.StatusSuccess, nil
			}
			return npc.StatusFailure, nil
		}},
		cd,
	))

	artifactHere := ConditionKeyBool("ArtifactHere", "artifact_here")
	nearTrap := ConditionKeyBool("NearTrap", "nearby_trap")

	collect := npc.NewSequence("Collect", artifactHere, pickup)
	safety := npc.NewSequence("Safety", nearTrap, rememberTrap)

	root := npc.NewSelector("Root", toExit, collect, safety, explore)
	tree := simpleTree{r: root}
	agent := npc.NewAgent(bb, mem, eb, tree, sensors)

	g.world = w
	g.agent = agent
	g.bb = bb
	g.mem = mem
}

func (g *Game) Restart() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopCh != nil && !g.stopped {
		close(g.stopCh)
	}
	g.stopCh = make(chan struct{})
	g.stopped = false
	g.buildAgentLocked()
}

// --- Websocket broadcasting ---

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func newHub() *wsHub { return &wsHub{clients: make(map[*wsClient]struct{})} }

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

// message model

type TickFrame struct {
	Tick    int        `json:"tick"`
	Status  string     `json:"status"`
	PosX    int        `json:"pos_x"`
	PosY    int        `json:"pos_y"`
	HP      int        `json:"hp"`
	Energy  int        `json:"energy"`
	ExitVis bool       `json:"exit_visible"`
	Grid    [][]string `json:"grid"`
}

// --- HTTP server and game loop ---

func main() {
	rand.Seed(time.Now().UnixNano())
	game := NewGame()
	game.Restart()
	hub := newHub()

	// writer goroutine per client
	serveWS := func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("ws upgrade:", err)
			return
		}
		client := &wsClient{conn: c, send: make(chan []byte, 64)}
		hub.add(client)
		defer func() { hub.remove(client); c.Close() }()
		// reader: accept control messages (e.g., restart)
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
		// writer
		for b := range client.send {
			if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		}
	}

	// HTTP handlers
	http.HandleFunc("/ws", serveWS)
	http.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		game.Restart()
		w.WriteHeader(204)
	})
	// static
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("examples/npc/static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "examples/npc/static/index.html")
	})

	// tick loop
	go func() {
		tick := 0
		for {
			game.mu.RLock()
			stopCh := game.stopCh
			bb := game.bb
			world := game.world
			agent := game.agent
			game.mu.RUnlock()

			select {
			case <-stopCh:
				continue
			default:
			}

			st, err := agent.Step(context.Background())
			if err != nil {
				log.Println("step error:", err)
			}
			// prepare frame
			grid := make([][]string, world.H)
			for y := 0; y < world.H; y++ {
				grid[y] = make([]string, world.W)
				for x := 0; x < world.W; x++ {
					grid[y][x] = world.Grid[y][x].String()
				}
			}
			frame := TickFrame{
				Tick:    tick,
				Status:  fmt.Sprintf("%v", st),
				PosX:    getInt(bb, "pos_x"),
				PosY:    getInt(bb, "pos_y"),
				HP:      getInt(bb, "hp"),
				Energy:  getInt(bb, "energy"),
				ExitVis: getBool(bb, "exit_visible"),
				Grid:    grid,
			}
			b, _ := json.Marshal(frame)
			hub.broadcast(b)

			tick++
			time.Sleep(80 * time.Millisecond)
		}
	}()

	log.Println("NPC web server on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
