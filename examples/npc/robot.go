package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	npc "github.com/zeusync/zeusync/internal/core/npc"
)

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

type Tile int

const (
	Empty Tile = iota
	Trap
	Artifact
	Exit
	Fog
)

type World struct {
	W, H int
	Grid [][]Tile
}

func NewWorld() *World {
	w := &World{W: 10, H: 8, Grid: make([][]Tile, 8)}
	for y := 0; y < w.H; y++ {
		w.Grid[y] = make([]Tile, w.W)
	}
	// place some traps
	w.Grid[2][3] = Trap
	w.Grid[2][4] = Trap
	w.Grid[5][2] = Trap
	// artifacts
	w.Grid[1][7] = Artifact
	w.Grid[6][6] = Artifact
	// fog areas
	w.Grid[3][3] = Fog
	w.Grid[3][4] = Fog
	w.Grid[4][3] = Fog
	w.Grid[4][4] = Fog
	// exit
	w.Grid[7][9] = Exit
	return w
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

// VisionSensor updates what the robot can see and writes to blackboard
// Keys written:
// - exit_visible (bool), exit_x, exit_y (int)
// - trap_here (bool), artifact_here (bool), fog_here (bool)
// - nearby_trap (bool), nearby_artifact (bool)
// - speed_mod (float64) based on energy

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

// Movement and actions

func MoveTowardsAction(name, targetXKey, targetYKey string) npc.Action {
	return ActionWrap{name: name, fn: func(t npc.TickContext) (npc.Status, error) {
		curX := getInt(t.BB, "pos_x")
		curY := getInt(t.BB, "pos_y")
		gx := getInt(t.BB, targetXKey)
		gy := getInt(t.BB, targetYKey)
		if curX == gx && curY == gy {
			return npc.StatusSuccess, nil
		}
		// simple step towards target
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
		// cautious in fog: sometimes skip movement
		if getBool(t.BB, "fog_here") {
			if rand.Float64() < 0.4 { // skip move this tick
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
		// visited map
		v, _ := t.BB.Get("visited")
		vm, _ := v.(map[[2]int]bool)
		if vm == nil {
			vm = make(map[[2]int]bool)
		}
		vm[[2]int{x, y}] = true
		// candidates
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
			// reset visited to escape deadlocks
			vm = map[[2]int]bool{{x, y}: true}
			// random step
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
			// lose HP and remember
			hp := getInt(t.BB, "hp")
			if hp <= 0 {
				return npc.StatusFailure, nil
			}
			hp -= 1
			t.BB.Set("hp", hp)
		}
		// mark current and neighbors as dangerous to avoid
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

type simpleTree struct{ r npc.BehaviorNode }

func (t simpleTree) Root() npc.BehaviorNode { return t.r }
func (t simpleTree) Tick(tc npc.TickContext) (npc.Status, error) {
	if t.r == nil {
		return npc.StatusSuccess, nil
	}
	return t.r.Tick(tc)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	world := NewWorld()

	reg := npc.NewRegistry()
	npc.RegisterBuiltins(reg)
	npc.RegisterSensors(reg)

	bb := npc.NewBlackboard()
	mem := npc.NewMemory()
	eb := npc.NewEventBus()
	// initial state
	bb.Set("pos_x", 0)
	bb.Set("pos_y", 0)
	bb.Set("hp", 5)
	bb.Set("energy", 0)

	// sensors
	sensors := []npc.Sensor{&VisionSensor{world: world, radius: 3}}

	// build tree programmatically for clarity
	// Root: Selector(
	//   Sequence(IsExitVisible, MoveToExit until AtExit),
	//   Sequence(artifact_here? -> Pickup, MoveToArtifact?),
	//   Sequence(nearby_trap? -> RememberTrap),
	//   Explore
	// )

	isExitVisible := ConditionKeyBool("ExitVisible", "exit_visible")
	atExit := IsAtExitCondition(world)
	moveToExit := MoveTowardsAction("MoveToExit", "exit_x", "exit_y")
	pickup := PickupArtifactAction(world)
	rememberTrap := RememberTrapAction(world)
	explore := ExploreAction("Explore", world)

	// decorate move with Cooldown to avoid jitter
	cd := npc.NewCooldown("MoveCooldown", 200*time.Millisecond, false)
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

	collect := npc.NewSequence("Collect",
		artifactHere,
		pickup,
	)

	safety := npc.NewSequence("Safety",
		nearTrap,
		rememberTrap,
	)

	root := npc.NewSelector("Root",
		toExit,
		collect,
		safety,
		explore,
	)

	tree := simpleTree{r: root}
	agent := npc.NewAgent(bb, mem, eb, tree, sensors)

	ctx := context.Background()
	for tick := 0; tick < 100; tick++ {
		st, err := agent.Step(ctx)
		if err != nil {
			panic(err)
		}
		x := getInt(bb, "pos_x")
		y := getInt(bb, "pos_y")
		hp := getInt(bb, "hp")
		energy := getInt(bb, "energy")
		exitVisible := getBool(bb, "exit_visible")
		fmt.Printf("tick=%02d pos=(%d,%d) hp=%d energy=%d status=%v exitVisible=%v\n", tick, x, y, hp, energy, st, exitVisible)
		if world.TileAt(x, y) == Exit {
			fmt.Println("Robot escaped the ruins!")
			break
		}
		if hp <= 0 {
			fmt.Println("Robot broke in traps.")
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}
