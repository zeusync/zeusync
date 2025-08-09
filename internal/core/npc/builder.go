package npc

import (
	"fmt"
	"sync"
)

// BehaviorTreeBuilderImpl implements the BehaviorTreeBuilder interface
type BehaviorTreeBuilderImpl struct {
	mu            sync.RWMutex
	nodeFactories map[string]NodeFactory
}

// NewBehaviorTreeBuilder creates a new behavior tree builder
func NewBehaviorTreeBuilder() *BehaviorTreeBuilderImpl {
	builder := &BehaviorTreeBuilderImpl{
		nodeFactories: make(map[string]NodeFactory),
	}

	// Register default node types
	builder.registerDefaultFactories()

	return builder
}

// registerDefaultFactories registers the built-in node factories
func (btb *BehaviorTreeBuilderImpl) registerDefaultFactories() {
	btb.nodeFactories["Sequence"] = &SequenceNodeFactory{}
	btb.nodeFactories["Selector"] = &SelectorNodeFactory{}
	btb.nodeFactories["Parallel"] = &ParallelNodeFactory{}
	btb.nodeFactories["Inverter"] = &InverterNodeFactory{}
	btb.nodeFactories["Repeater"] = &RepeaterNodeFactory{}
	btb.nodeFactories["RandomSelector"] = &RandomSelectorNodeFactory{}
	btb.nodeFactories["Condition"] = &ConditionNodeFactory{}
	btb.nodeFactories["Action"] = &ActionNodeFactory{}
	btb.nodeFactories["Wait"] = &WaitActionNodeFactory{}
	btb.nodeFactories["Log"] = &LogActionNodeFactory{}
}

// BuildFromConfig creates a behavior tree from configuration
func (btb *BehaviorTreeBuilderImpl) BuildFromConfig(config *BehaviorTreeConfig) (Node, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return btb.buildNode(config.Root)
}

// buildNode recursively builds a node from configuration
func (btb *BehaviorTreeBuilderImpl) buildNode(config *NodeConfig) (Node, error) {
	if config == nil {
		return nil, fmt.Errorf("node config cannot be nil")
	}

	if !config.Enabled {
		// Return a no-op node for disabled nodes
		return NewBasicActionNode(config.Name, func(ctx *ExecutionContext) NodeStatus {
			return StatusSuccess
		}), nil
	}

	btb.mu.RLock()
	factory, exists := btb.nodeFactories[config.Type]
	btb.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown node type: %s", config.Type)
	}

	// Create the node
	node, err := factory.CreateNode(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create node %s: %w", config.Name, err)
	}

	// Handle composite nodes (nodes with multiple children)
	if compositeNode, ok := node.(CompositeNode); ok && len(config.Children) > 0 {
		for _, childConfig := range config.Children {
			childNode, err := btb.buildNode(childConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to build child node: %w", err)
			}
			compositeNode.AddChild(childNode)
		}
	}

	// Handle decorator nodes (nodes with a single child)
	if decoratorNode, ok := node.(DecoratorNode); ok && config.Child != nil {
		childNode, err := btb.buildNode(config.Child)
		if err != nil {
			return nil, fmt.Errorf("failed to build child node: %w", err)
		}
		decoratorNode.SetChild(childNode)
	}

	return node, nil
}

// RegisterNodeType registers a custom node type
func (btb *BehaviorTreeBuilderImpl) RegisterNodeType(nodeType string, factory NodeFactory) {
	btb.mu.Lock()
	defer btb.mu.Unlock()

	btb.nodeFactories[nodeType] = factory
}

// GetRegisteredTypes returns all registered node types
func (btb *BehaviorTreeBuilderImpl) GetRegisteredTypes() []string {
	btb.mu.RLock()
	defer btb.mu.RUnlock()

	types := make([]string, 0, len(btb.nodeFactories))
	for nodeType := range btb.nodeFactories {
		types = append(types, nodeType)
	}
	return types
}

// Node Factories

// SequenceNodeFactory creates sequence nodes
type SequenceNodeFactory struct{}

func (f *SequenceNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	return NewSequenceNode(config.Name), nil
}

func (f *SequenceNodeFactory) GetNodeType() string {
	return "Sequence"
}

// SelectorNodeFactory creates selector nodes
type SelectorNodeFactory struct{}

func (f *SelectorNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	return NewSelectorNode(config.Name), nil
}

func (f *SelectorNodeFactory) GetNodeType() string {
	return "Selector"
}

// ParallelNodeFactory creates parallel nodes
type ParallelNodeFactory struct{}

func (f *ParallelNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	successPolicy, _ := config.GetIntParameter("success_policy")
	if successPolicy <= 0 {
		successPolicy = 1 // Default: at least one child must succeed
	}

	failurePolicy, _ := config.GetIntParameter("failure_policy")
	if failurePolicy <= 0 {
		failurePolicy = 1 // Default: at least one child must fail
	}

	return NewParallelNode(config.Name, successPolicy, failurePolicy), nil
}

func (f *ParallelNodeFactory) GetNodeType() string {
	return "Parallel"
}

// InverterNodeFactory creates inverter nodes
type InverterNodeFactory struct{}

func (f *InverterNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	return NewInverterNode(config.Name), nil
}

func (f *InverterNodeFactory) GetNodeType() string {
	return "Inverter"
}

// RepeaterNodeFactory creates repeater nodes
type RepeaterNodeFactory struct{}

func (f *RepeaterNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	maxRepeats, _ := config.GetIntParameter("max_repeats")
	if maxRepeats < 0 {
		maxRepeats = -1 // Infinite repeats
	}

	return NewRepeaterNode(config.Name, maxRepeats), nil
}

func (f *RepeaterNodeFactory) GetNodeType() string {
	return "Repeater"
}

// RandomSelectorNodeFactory creates random selector nodes
type RandomSelectorNodeFactory struct{}

func (f *RandomSelectorNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	return NewRandomSelectorNode(config.Name), nil
}

func (f *RandomSelectorNodeFactory) GetNodeType() string {
	return "RandomSelector"
}

// ConditionNodeFactory creates condition nodes
type ConditionNodeFactory struct{}

func (f *ConditionNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	conditionType, exists := config.GetStringParameter("condition_type")
	if !exists {
		return nil, fmt.Errorf("condition_type parameter is required")
	}

	switch conditionType {
	case "blackboard_check":
		return f.createBlackboardCondition(config)
	case "compare":
		return f.createCompareCondition(config)
	case "has_key":
		return f.createHasKeyCondition(config)
	default:
		return nil, fmt.Errorf("unknown condition type: %s", conditionType)
	}
}

func (f *ConditionNodeFactory) createBlackboardCondition(config *NodeConfig) (Node, error) {
	key, exists := config.GetStringParameter("key")
	if !exists {
		return nil, fmt.Errorf("key parameter is required for blackboard_check condition")
	}

	expectedValue, exists := config.GetParameter("expected_value")
	if !exists {
		return nil, fmt.Errorf("expected_value parameter is required for blackboard_check condition")
	}

	return NewBasicConditionNode(config.Name, func(ctx *ExecutionContext) bool {
		value, exists := ctx.Blackboard.Get(key)
		return exists && value == expectedValue
	}), nil
}

func (f *ConditionNodeFactory) createCompareCondition(config *NodeConfig) (Node, error) {
	key, exists := config.GetStringParameter("key")
	if !exists {
		return nil, fmt.Errorf("key parameter is required for compare condition")
	}

	operator, exists := config.GetStringParameter("operator")
	if !exists {
		return nil, fmt.Errorf("operator parameter is required for compare condition")
	}

	value, exists := config.GetParameter("value")
	if !exists {
		return nil, fmt.Errorf("value parameter is required for compare condition")
	}

	return NewBasicConditionNode(config.Name, func(ctx *ExecutionContext) bool {
		bbValue, exists := ctx.Blackboard.Get(key)
		if !exists {
			return false
		}

		return compareValues(bbValue, operator, value)
	}), nil
}

func (f *ConditionNodeFactory) createHasKeyCondition(config *NodeConfig) (Node, error) {
	key, exists := config.GetStringParameter("key")
	if !exists {
		return nil, fmt.Errorf("key parameter is required for has_key condition")
	}

	return NewBasicConditionNode(config.Name, func(ctx *ExecutionContext) bool {
		return ctx.Blackboard.Has(key)
	}), nil
}

func (f *ConditionNodeFactory) GetNodeType() string {
	return "Condition"
}

// ActionNodeFactory creates action nodes
type ActionNodeFactory struct{}

func (f *ActionNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	actionType, exists := config.GetStringParameter("action_type")
	if !exists {
		return nil, fmt.Errorf("action_type parameter is required")
	}

	switch actionType {
	case "set_blackboard":
		return f.createSetBlackboardAction(config)
	case "increment":
		return f.createIncrementAction(config)
	case "custom":
		return f.createCustomAction(config)
	default:
		return nil, fmt.Errorf("unknown action type: %s", actionType)
	}
}

func (f *ActionNodeFactory) createSetBlackboardAction(config *NodeConfig) (Node, error) {
	key, exists := config.GetStringParameter("key")
	if !exists {
		return nil, fmt.Errorf("key parameter is required for set_blackboard action")
	}

	value, exists := config.GetParameter("value")
	if !exists {
		return nil, fmt.Errorf("value parameter is required for set_blackboard action")
	}

	return NewBasicActionNode(config.Name, func(ctx *ExecutionContext) NodeStatus {
		ctx.Blackboard.Set(key, value)
		return StatusSuccess
	}), nil
}

func (f *ActionNodeFactory) createIncrementAction(config *NodeConfig) (Node, error) {
	key, exists := config.GetStringParameter("key")
	if !exists {
		return nil, fmt.Errorf("key parameter is required for increment action")
	}

	increment, exists := config.GetFloatParameter("increment")
	if !exists {
		increment = 1.0 // Default increment
	}

	return NewBasicActionNode(config.Name, func(ctx *ExecutionContext) NodeStatus {
		currentValue, exists := ctx.Blackboard.GetFloat(key)
		if !exists {
			currentValue = 0.0
		}

		ctx.Blackboard.Set(key, currentValue+increment)
		return StatusSuccess
	}), nil
}

func (f *ActionNodeFactory) createCustomAction(config *NodeConfig) (Node, error) {
	// This would be extended to support custom action registration
	return NewBasicActionNode(config.Name, func(ctx *ExecutionContext) NodeStatus {
		// Custom action implementation would go here
		return StatusSuccess
	}), nil
}

func (f *ActionNodeFactory) GetNodeType() string {
	return "Action"
}

// WaitActionNodeFactory creates wait action nodes
type WaitActionNodeFactory struct{}

func (f *WaitActionNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	duration, exists := config.GetDurationParameter("duration")
	if !exists {
		return nil, fmt.Errorf("duration parameter is required for wait action")
	}

	return NewWaitActionNode(config.Name, duration), nil
}

func (f *WaitActionNodeFactory) GetNodeType() string {
	return "Wait"
}

// LogActionNodeFactory creates log action nodes
type LogActionNodeFactory struct{}

func (f *LogActionNodeFactory) CreateNode(config *NodeConfig) (Node, error) {
	message, exists := config.GetStringParameter("message")
	if !exists {
		message = "Log action executed"
	}

	return NewLogActionNode(config.Name, message), nil
}

func (f *LogActionNodeFactory) GetNodeType() string {
	return "Log"
}

// Helper functions

// compareValues compares two values using the specified operator
func compareValues(a interface{}, operator string, b interface{}) bool {
	switch operator {
	case "==", "eq":
		return a == b
	case "!=", "ne":
		return a != b
	case ">", "gt":
		return compareNumeric(a, b, func(x, y float64) bool { return x > y })
	case ">=", "gte":
		return compareNumeric(a, b, func(x, y float64) bool { return x >= y })
	case "<", "lt":
		return compareNumeric(a, b, func(x, y float64) bool { return x < y })
	case "<=", "lte":
		return compareNumeric(a, b, func(x, y float64) bool { return x <= y })
	default:
		return false
	}
}

// compareNumeric compares two values numerically
func compareNumeric(a, b interface{}, compareFn func(float64, float64) bool) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if !aOk || !bOk {
		return false
	}

	return compareFn(aFloat, bFloat)
}

// toFloat64 converts a value to float64
func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}
