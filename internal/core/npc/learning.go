package npc

// Memory & Learning components (minimal stubs)

// MemoryManager wraps the Memory interface for extended operations
// like trimming history or exporting metrics.
type MemoryManager interface {
	Append(rec DecisionRecord)
	All() []DecisionRecord
	Reset()
}

type memoryManager struct{ mem Memory }

func NewMemoryManager(mem Memory) MemoryManager    { return &memoryManager{mem: mem} }
func (m *memoryManager) Append(rec DecisionRecord) { m.mem.AppendDecision(rec) }
func (m *memoryManager) All() []DecisionRecord     { return m.mem.History() }
func (m *memoryManager) Reset()                    { m.mem.Reset() }

// StateManager persists agent state (blackboard and memory).
// The Agent already provides SaveState/LoadState, so this wraps those.
type StateManager interface {
	Save(a Agent) ([]byte, error)
	Load(a Agent, data []byte) error
}

type stateManager struct{}

func NewStateManager() StateManager                  { return stateManager{} }
func (stateManager) Save(a Agent) ([]byte, error)    { return a.SaveState() }
func (stateManager) Load(a Agent, data []byte) error { return a.LoadState(data) }

// Analytics computes basic statistics over decision history.
type Analytics interface {
	SuccessRate() float64
	Total() int
}

type analytics struct{ mem Memory }

func NewAnalytics(mem Memory) Analytics { return &analytics{mem: mem} }

func (a *analytics) SuccessRate() float64 {
	h := a.mem.History()
	if len(h) == 0 {
		return 0
	}
	s := 0
	for _, r := range h {
		if r.Status == StatusSuccess {
			s++
		}
	}
	return float64(s) / float64(len(h))
}

func (a *analytics) Total() int { return len(a.mem.History()) }
