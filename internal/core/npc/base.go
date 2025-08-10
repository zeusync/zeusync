package npc

// Base node and functional wrappers

// baseNode implements common Name storage for nodes

type baseNode struct{ name string }

func (b baseNode) Name() string { return b.name }

// ActionFunc wraps a function as an Action node.

type ActionFunc struct {
	baseNode
	Fn func(t TickContext) (Status, error)
}

func (a ActionFunc) Tick(t TickContext) (Status, error) { return a.Fn(t) }

// ConditionFunc wraps a function as a Condition node.

type ConditionFunc struct {
	baseNode
	Fn func(t TickContext) (bool, error)
}

func (c ConditionFunc) Tick(t TickContext) (Status, error) {
	ok, err := c.Fn(t)
	if err != nil {
		return StatusFailure, err
	}
	if ok {
		return StatusSuccess, nil
	}
	return StatusFailure, nil
}
