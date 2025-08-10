package npc

// Tree is a simple DecisionTree implementation with a root node.

type Tree struct{ root BehaviorNode }

func (t Tree) Root() BehaviorNode { return t.root }

func (t Tree) Tick(tc TickContext) (Status, error) {
	if t.root == nil {
		return StatusSuccess, nil
	}
	return t.root.Tick(tc)
}
