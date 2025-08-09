package npc

import (
	"math/rand"
	"time"
)

// BaseNode provides common functionality for all nodes
type BaseNode struct {
	name        string
	nodeType    string
	status      NodeStatus
	lastExecute time.Time
}

// NewBaseNode creates a new base node
func NewBaseNode(name, nodeType string) *BaseNode {
	return &BaseNode{
		name:     name,
		nodeType: nodeType,
		status:   StatusInvalid,
	}
}

// GetName returns the node name
func (bn *BaseNode) GetName() string {
	return bn.name
}

// GetType returns the node type
func (bn *BaseNode) GetType() string {
	return bn.nodeType
}

// Reset resets the node status
func (bn *BaseNode) Reset() {
	bn.status = StatusInvalid
}

// GetStatus returns the current status
func (bn *BaseNode) GetStatus() NodeStatus {
	return bn.status
}

// SetStatus sets the node status
func (bn *BaseNode) SetStatus(status NodeStatus) {
	bn.status = status
	bn.lastExecute = time.Now()
}

// Sequence Node - executes children in order, fails on first failure
type SequenceNode struct {
	*BaseNode
	children     []Node
	currentChild int
}

// NewSequenceNode creates a new sequence node
func NewSequenceNode(name string) *SequenceNode {
	return &SequenceNode{
		BaseNode:     NewBaseNode(name, "Sequence"),
		children:     make([]Node, 0),
		currentChild: 0,
	}
}

// Execute runs the sequence logic
func (sn *SequenceNode) Execute(ctx *ExecutionContext) NodeStatus {
	if len(sn.children) == 0 {
		sn.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	for sn.currentChild < len(sn.children) {
		child := sn.children[sn.currentChild]
		status := child.Execute(ctx)

		switch status {
		case StatusFailure:
			sn.Reset()
			sn.SetStatus(StatusFailure)
			return StatusFailure
		case StatusRunning:
			sn.SetStatus(StatusRunning)
			return StatusRunning
		case StatusSuccess:
			sn.currentChild++
			continue
		}
	}

	// All children succeeded
	sn.Reset()
	sn.SetStatus(StatusSuccess)
	return StatusSuccess
}

// AddChild adds a child node
func (sn *SequenceNode) AddChild(child Node) {
	sn.children = append(sn.children, child)
}

// GetChildren returns all children
func (sn *SequenceNode) GetChildren() []Node {
	return sn.children
}

// RemoveChild removes a child node
func (sn *SequenceNode) RemoveChild(child Node) bool {
	for i, c := range sn.children {
		if c == child {
			sn.children = append(sn.children[:i], sn.children[i+1:]...)
			return true
		}
	}
	return false
}

// Reset resets the sequence node
func (sn *SequenceNode) Reset() {
	sn.BaseNode.Reset()
	sn.currentChild = 0
	for _, child := range sn.children {
		child.Reset()
	}
}

// Clone creates a copy of the sequence node
func (sn *SequenceNode) Clone() Node {
	clone := NewSequenceNode(sn.name)
	for _, child := range sn.children {
		clone.AddChild(child.Clone())
	}
	return clone
}

// Selector Node - executes children until one succeeds
type SelectorNode struct {
	*BaseNode
	children     []Node
	currentChild int
}

// NewSelectorNode creates a new selector node
func NewSelectorNode(name string) *SelectorNode {
	return &SelectorNode{
		BaseNode:     NewBaseNode(name, "Selector"),
		children:     make([]Node, 0),
		currentChild: 0,
	}
}

// Execute runs the selector logic
func (sn *SelectorNode) Execute(ctx *ExecutionContext) NodeStatus {
	if len(sn.children) == 0 {
		sn.SetStatus(StatusFailure)
		return StatusFailure
	}

	for sn.currentChild < len(sn.children) {
		child := sn.children[sn.currentChild]
		status := child.Execute(ctx)

		switch status {
		case StatusSuccess:
			sn.Reset()
			sn.SetStatus(StatusSuccess)
			return StatusSuccess
		case StatusRunning:
			sn.SetStatus(StatusRunning)
			return StatusRunning
		case StatusFailure:
			sn.currentChild++
			continue
		}
	}

	// All children failed
	sn.Reset()
	sn.SetStatus(StatusFailure)
	return StatusFailure
}

// AddChild adds a child node
func (sn *SelectorNode) AddChild(child Node) {
	sn.children = append(sn.children, child)
}

// GetChildren returns all children
func (sn *SelectorNode) GetChildren() []Node {
	return sn.children
}

// RemoveChild removes a child node
func (sn *SelectorNode) RemoveChild(child Node) bool {
	for i, c := range sn.children {
		if c == child {
			sn.children = append(sn.children[:i], sn.children[i+1:]...)
			return true
		}
	}
	return false
}

// Reset resets the selector node
func (sn *SelectorNode) Reset() {
	sn.BaseNode.Reset()
	sn.currentChild = 0
	for _, child := range sn.children {
		child.Reset()
	}
}

// Clone creates a copy of the selector node
func (sn *SelectorNode) Clone() Node {
	clone := NewSelectorNode(sn.name)
	for _, child := range sn.children {
		clone.AddChild(child.Clone())
	}
	return clone
}

// Parallel Node - executes all children simultaneously
type ParallelNode struct {
	*BaseNode
	children       []Node
	successPolicy  int // number of children that must succeed
	failurePolicy  int // number of children that must fail
	childrenStatus []NodeStatus
}

// NewParallelNode creates a new parallel node
func NewParallelNode(name string, successPolicy, failurePolicy int) *ParallelNode {
	return &ParallelNode{
		BaseNode:       NewBaseNode(name, "Parallel"),
		children:       make([]Node, 0),
		successPolicy:  successPolicy,
		failurePolicy:  failurePolicy,
		childrenStatus: make([]NodeStatus, 0),
	}
}

// Execute runs the parallel logic
func (pn *ParallelNode) Execute(ctx *ExecutionContext) NodeStatus {
	if len(pn.children) == 0 {
		pn.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	// Initialize status array if needed
	if len(pn.childrenStatus) != len(pn.children) {
		pn.childrenStatus = make([]NodeStatus, len(pn.children))
	}

	successCount := 0
	failureCount := 0
	runningCount := 0

	// Execute all children
	for i, child := range pn.children {
		if pn.childrenStatus[i] == StatusRunning || pn.childrenStatus[i] == StatusInvalid {
			pn.childrenStatus[i] = child.Execute(ctx)
		}

		switch pn.childrenStatus[i] {
		case StatusSuccess:
			successCount++
		case StatusFailure:
			failureCount++
		case StatusRunning:
			runningCount++
		}
	}

	// Check policies
	if successCount >= pn.successPolicy {
		pn.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	if failureCount >= pn.failurePolicy {
		pn.SetStatus(StatusFailure)
		return StatusFailure
	}

	pn.SetStatus(StatusRunning)
	return StatusRunning
}

// AddChild adds a child node
func (pn *ParallelNode) AddChild(child Node) {
	pn.children = append(pn.children, child)
	pn.childrenStatus = append(pn.childrenStatus, StatusInvalid)
}

// GetChildren returns all children
func (pn *ParallelNode) GetChildren() []Node {
	return pn.children
}

// RemoveChild removes a child node
func (pn *ParallelNode) RemoveChild(child Node) bool {
	for i, c := range pn.children {
		if c == child {
			pn.children = append(pn.children[:i], pn.children[i+1:]...)
			pn.childrenStatus = append(pn.childrenStatus[:i], pn.childrenStatus[i+1:]...)
			return true
		}
	}
	return false
}

// Reset resets the parallel node
func (pn *ParallelNode) Reset() {
	pn.BaseNode.Reset()
	for i := range pn.childrenStatus {
		pn.childrenStatus[i] = StatusInvalid
	}
	for _, child := range pn.children {
		child.Reset()
	}
}

// Clone creates a copy of the parallel node
func (pn *ParallelNode) Clone() Node {
	clone := NewParallelNode(pn.name, pn.successPolicy, pn.failurePolicy)
	for _, child := range pn.children {
		clone.AddChild(child.Clone())
	}
	return clone
}

// Inverter Node - inverts the result of its child
type InverterNode struct {
	*BaseNode
	child Node
}

// NewInverterNode creates a new inverter node
func NewInverterNode(name string) *InverterNode {
	return &InverterNode{
		BaseNode: NewBaseNode(name, "Inverter"),
	}
}

// Execute runs the inverter logic
func (in *InverterNode) Execute(ctx *ExecutionContext) NodeStatus {
	if in.child == nil {
		in.SetStatus(StatusFailure)
		return StatusFailure
	}

	status := in.child.Execute(ctx)

	switch status {
	case StatusSuccess:
		in.SetStatus(StatusFailure)
		return StatusFailure
	case StatusFailure:
		in.SetStatus(StatusSuccess)
		return StatusSuccess
	case StatusRunning:
		in.SetStatus(StatusRunning)
		return StatusRunning
	default:
		in.SetStatus(StatusFailure)
		return StatusFailure
	}
}

// SetChild sets the child node
func (in *InverterNode) SetChild(child Node) {
	in.child = child
}

// GetChild returns the child node
func (in *InverterNode) GetChild() Node {
	return in.child
}

// Reset resets the inverter node
func (in *InverterNode) Reset() {
	in.BaseNode.Reset()
	if in.child != nil {
		in.child.Reset()
	}
}

// Clone creates a copy of the inverter node
func (in *InverterNode) Clone() Node {
	clone := NewInverterNode(in.name)
	if in.child != nil {
		clone.SetChild(in.child.Clone())
	}
	return clone
}

// Repeater Node - repeats its child a specified number of times
type RepeaterNode struct {
	*BaseNode
	child       Node
	maxRepeats  int
	currentRuns int
}

// NewRepeaterNode creates a new repeater node
func NewRepeaterNode(name string, maxRepeats int) *RepeaterNode {
	return &RepeaterNode{
		BaseNode:    NewBaseNode(name, "Repeater"),
		maxRepeats:  maxRepeats,
		currentRuns: 0,
	}
}

// Execute runs the repeater logic
func (rn *RepeaterNode) Execute(ctx *ExecutionContext) NodeStatus {
	if rn.child == nil {
		rn.SetStatus(StatusFailure)
		return StatusFailure
	}

	if rn.maxRepeats > 0 && rn.currentRuns >= rn.maxRepeats {
		rn.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	status := rn.child.Execute(ctx)

	switch status {
	case StatusSuccess, StatusFailure:
		rn.currentRuns++
		rn.child.Reset()

		if rn.maxRepeats > 0 && rn.currentRuns >= rn.maxRepeats {
			rn.SetStatus(StatusSuccess)
			return StatusSuccess
		}

		rn.SetStatus(StatusRunning)
		return StatusRunning
	case StatusRunning:
		rn.SetStatus(StatusRunning)
		return StatusRunning
	default:
		rn.SetStatus(StatusFailure)
		return StatusFailure
	}
}

// SetChild sets the child node
func (rn *RepeaterNode) SetChild(child Node) {
	rn.child = child
}

// GetChild returns the child node
func (rn *RepeaterNode) GetChild() Node {
	return rn.child
}

// Reset resets the repeater node
func (rn *RepeaterNode) Reset() {
	rn.BaseNode.Reset()
	rn.currentRuns = 0
	if rn.child != nil {
		rn.child.Reset()
	}
}

// Clone creates a copy of the repeater node
func (rn *RepeaterNode) Clone() Node {
	clone := NewRepeaterNode(rn.name, rn.maxRepeats)
	if rn.child != nil {
		clone.SetChild(rn.child.Clone())
	}
	return clone
}

// RandomSelector Node - randomly selects one child to execute
type RandomSelectorNode struct {
	*BaseNode
	children []Node
	rng      *rand.Rand
}

// NewRandomSelectorNode creates a new random selector node
func NewRandomSelectorNode(name string) *RandomSelectorNode {
	return &RandomSelectorNode{
		BaseNode: NewBaseNode(name, "RandomSelector"),
		children: make([]Node, 0),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute runs the random selector logic
func (rsn *RandomSelectorNode) Execute(ctx *ExecutionContext) NodeStatus {
	if len(rsn.children) == 0 {
		rsn.SetStatus(StatusFailure)
		return StatusFailure
	}

	// Select a random child
	index := rsn.rng.Intn(len(rsn.children))
	child := rsn.children[index]

	status := child.Execute(ctx)
	rsn.SetStatus(status)
	return status
}

// AddChild adds a child node
func (rsn *RandomSelectorNode) AddChild(child Node) {
	rsn.children = append(rsn.children, child)
}

// GetChildren returns all children
func (rsn *RandomSelectorNode) GetChildren() []Node {
	return rsn.children
}

// RemoveChild removes a child node
func (rsn *RandomSelectorNode) RemoveChild(child Node) bool {
	for i, c := range rsn.children {
		if c == child {
			rsn.children = append(rsn.children[:i], rsn.children[i+1:]...)
			return true
		}
	}
	return false
}

// Reset resets the random selector node
func (rsn *RandomSelectorNode) Reset() {
	rsn.BaseNode.Reset()
	for _, child := range rsn.children {
		child.Reset()
	}
}

// Clone creates a copy of the random selector node
func (rsn *RandomSelectorNode) Clone() Node {
	clone := NewRandomSelectorNode(rsn.name)
	for _, child := range rsn.children {
		clone.AddChild(child.Clone())
	}
	return clone
}

// ConditionNode represents a basic condition check
type BasicConditionNode struct {
	*BaseNode
	conditionFunc func(*ExecutionContext) bool
}

// NewBasicConditionNode creates a new basic condition node
func NewBasicConditionNode(name string, conditionFunc func(*ExecutionContext) bool) *BasicConditionNode {
	return &BasicConditionNode{
		BaseNode:      NewBaseNode(name, "Condition"),
		conditionFunc: conditionFunc,
	}
}

// Execute runs the condition check
func (bcn *BasicConditionNode) Execute(ctx *ExecutionContext) NodeStatus {
	if bcn.conditionFunc == nil {
		bcn.SetStatus(StatusFailure)
		return StatusFailure
	}

	if bcn.conditionFunc(ctx) {
		bcn.SetStatus(StatusSuccess)
		return StatusSuccess
	}

	bcn.SetStatus(StatusFailure)
	return StatusFailure
}

// Evaluate evaluates the condition
func (bcn *BasicConditionNode) Evaluate(ctx *ExecutionContext) bool {
	if bcn.conditionFunc == nil {
		return false
	}
	return bcn.conditionFunc(ctx)
}

// Clone creates a copy of the condition node
func (bcn *BasicConditionNode) Clone() Node {
	return NewBasicConditionNode(bcn.name, bcn.conditionFunc)
}

// ActionNode represents a basic action
type BasicActionNode struct {
	*BaseNode
	actionFunc func(*ExecutionContext) NodeStatus
}

// NewBasicActionNode creates a new basic action node
func NewBasicActionNode(name string, actionFunc func(*ExecutionContext) NodeStatus) *BasicActionNode {
	return &BasicActionNode{
		BaseNode:   NewBaseNode(name, "Action"),
		actionFunc: actionFunc,
	}
}

// Execute runs the action
func (ban *BasicActionNode) Execute(ctx *ExecutionContext) NodeStatus {
	if ban.actionFunc == nil {
		ban.SetStatus(StatusFailure)
		return StatusFailure
	}

	status := ban.actionFunc(ctx)
	ban.SetStatus(status)
	return status
}

// CanExecute checks if the action can be executed
func (ban *BasicActionNode) CanExecute(ctx *ExecutionContext) bool {
	return ban.actionFunc != nil
}

// Clone creates a copy of the action node
func (ban *BasicActionNode) Clone() Node {
	return NewBasicActionNode(ban.name, ban.actionFunc)
}
