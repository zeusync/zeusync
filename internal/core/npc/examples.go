package npc

import (
	"time"
)

// CreateGameNPCConfig creates a configuration for a game NPC
func CreateGameNPCConfig(id, name string) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Game NPC Behavior",
			Root: &NodeConfig{
				Name:    "MainSelector",
				Type:    "Selector",
				Enabled: true,
				Children: []*NodeConfig{
					{
						Name:    "CombatSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsInCombat",
								Type:    "Condition",
								Enabled: true,
								Parameters: map[string]any{
									"condition_type": "has_key",
									"key":            "in_combat",
								},
							},
							{
								Name:    "IsEnemyVisible",
								Type:    "Condition",
								Enabled: true,
								Parameters: map[string]any{
									"condition_type": "compare",
									"key":            "distance_to_enemy",
									"operator":       "<=",
									"value":          10.0,
								},
							},
							{
								Name:    "AttackEnemy",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]any{
									"action_type": "custom",
									"message":     "Attacking enemy!",
								},
							},
						},
					},
					{
						Name:    "PatrolSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsNotInCombat",
								Type:    "Inverter",
								Enabled: true,
								Child: &NodeConfig{
									Name:    "CheckCombat",
									Type:    "Condition",
									Enabled: true,
									Parameters: map[string]any{
										"condition_type": "has_key",
										"key":            "in_combat",
									},
								},
							},
							{
								Name:    "Patrol",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]any{
									"action_type": "custom",
									"message":     "Patrolling area...",
								},
							},
						},
					},
					{
						Name:    "IdleAction",
						Type:    "Action",
						Enabled: true,
						Parameters: map[string]any{
							"action_type": "set_blackboard",
							"key":         "state",
							"value":       "idle",
						},
					},
				},
			},
		},
		Sensors: []*SensorConfig{
			{
				Name:           "HealthSensor",
				Type:           "Health",
				Enabled:        true,
				UpdateInterval: time.Millisecond * 100,
				Parameters: map[string]any{
					"health_key":     "health",
					"max_health_key": "max_health",
					"output_key":     "health_status",
					"low_threshold":  0.3,
				},
			},
			{
				Name:           "ProximitySensor",
				Type:           "Proximity",
				Enabled:        true,
				UpdateInterval: time.Millisecond * 200,
				Parameters: map[string]any{
					"position_key": "position",
					"entities_key": "nearby_entities",
					"output_key":   "proximity",
					"max_range":    15.0,
					"entity_types": []string{"player", "enemy"},
				},
			},
		},
		EventHandlers: []*EventHandlerConfig{
			{
				Name:      "DamageHandler",
				Type:      "damage",
				EventType: "damage",
				Priority:  8,
				Enabled:   true,
			},
			{
				Name:      "CombatHandler",
				Type:      "combat",
				EventType: "combat",
				Priority:  7,
				Enabled:   true,
			},
		},
		InitialData: map[string]interface{}{
			"health":     100.0,
			"max_health": 100.0,
			"position":   Position{X: 0, Y: 0, Z: 0},
			"state":      "idle",
		},
		UpdateRate: time.Millisecond * 100,
		Active:     true,
	}
}

// CreateTradingAgentConfig creates a configuration for a trading agent
func CreateTradingAgentConfig(id, name string) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Trading Agent Behavior",
			Root: &NodeConfig{
				Name:    "TradingSelector",
				Type:    "Selector",
				Enabled: true,
				Children: []*NodeConfig{
					{
						Name:    "HighDemandSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsDemandHigh",
								Type:    "Condition",
								Enabled: true,
								Parameters: map[string]interface{}{
									"condition_type": "compare",
									"key":            "market_demand",
									"operator":       ">",
									"value":          0.8,
								},
							},
							{
								Name:    "IncreasePrice",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]interface{}{
									"action_type": "increment",
									"key":         "price_multiplier",
									"increment":   0.1,
								},
							},
						},
					},
					{
						Name:    "LowDemandSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsDemandLow",
								Type:    "Condition",
								Enabled: true,
								Parameters: map[string]interface{}{
									"condition_type": "compare",
									"key":            "market_demand",
									"operator":       "<",
									"value":          0.3,
								},
							},
							{
								Name:    "DecreasePrice",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]interface{}{
									"action_type": "increment",
									"key":         "price_multiplier",
									"increment":   -0.05,
								},
							},
						},
					},
					{
						Name:    "MonitorMarket",
						Type:    "Action",
						Enabled: true,
						Parameters: map[string]interface{}{
							"action_type": "set_blackboard",
							"key":         "last_action",
							"value":       "monitoring",
						},
					},
				},
			},
		},
		Sensors: []*SensorConfig{
			{
				Name:           "InventorySensor",
				Type:           "Inventory",
				Enabled:        true,
				UpdateInterval: time.Second,
				Parameters: map[string]interface{}{
					"inventory_key": "inventory",
					"output_key":    "inventory_status",
					"item_types":    []string{"goods", "materials", "tools"},
				},
			},
			{
				Name:           "TimerSensor",
				Type:           "Timer",
				Enabled:        true,
				UpdateInterval: time.Millisecond * 100,
				Parameters: map[string]interface{}{
					"output_key": "timer",
				},
			},
		},
		EventHandlers: []*EventHandlerConfig{
			{
				Name:      "ItemHandler",
				Type:      "item",
				EventType: "item",
				Priority:  5,
				Enabled:   true,
			},
		},
		InitialData: map[string]interface{}{
			"market_demand":    0.5,
			"price_multiplier": 1.0,
			"inventory": map[string]int{
				"goods":     10,
				"materials": 5,
				"tools":     2,
			},
			"budget": 1000.0,
		},
		UpdateRate: time.Millisecond * 500,
		Active:     true,
	}
}

// CreatePatrolGuardConfig creates a configuration for a patrol guard
func CreatePatrolGuardConfig(id, name string, waypoints []Position) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Patrol Guard Behavior",
			Root: &NodeConfig{
				Name:    "GuardSelector",
				Type:    "Selector",
				Enabled: true,
				Children: []*NodeConfig{
					{
						Name:    "AlertSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsIntruderDetected",
								Type:    "Condition",
								Enabled: true,
								Parameters: map[string]interface{}{
									"condition_type": "compare",
									"key":            "proximity_player_detected",
									"operator":       "==",
									"value":          true,
								},
							},
							{
								Name:    "InvestigateIntruder",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]interface{}{
									"action_type": "custom",
									"message":     "Investigating intruder!",
								},
							},
						},
					},
					{
						Name:    "PatrolSequence",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "IsNotAlert",
								Type:    "Inverter",
								Enabled: true,
								Child: &NodeConfig{
									Name:    "CheckAlert",
									Type:    "Condition",
									Enabled: true,
									Parameters: map[string]interface{}{
										"condition_type": "has_key",
										"key":            "alert_state",
									},
								},
							},
							{
								Name:    "PatrolWaypoints",
								Type:    "Action",
								Enabled: true,
								Parameters: map[string]interface{}{
									"action_type": "custom",
									"message":     "Patrolling waypoints...",
								},
							},
						},
					},
					{
						Name:    "WaitAction",
						Type:    "Wait",
						Enabled: true,
						Parameters: map[string]interface{}{
							"duration": "2s",
						},
					},
				},
			},
		},
		Sensors: []*SensorConfig{
			{
				Name:           "ProximitySensor",
				Type:           "Proximity",
				Enabled:        true,
				UpdateInterval: time.Millisecond * 100,
				Parameters: map[string]interface{}{
					"position_key": "position",
					"entities_key": "world_entities",
					"output_key":   "proximity",
					"max_range":    8.0,
					"entity_types": []string{"player", "intruder"},
				},
			},
		},
		EventHandlers: []*EventHandlerConfig{
			{
				Name:      "CombatHandler",
				Type:      "combat",
				EventType: "combat",
				Priority:  9,
				Enabled:   true,
			},
		},
		InitialData: map[string]interface{}{
			"position":    Position{X: 0, Y: 0, Z: 0},
			"waypoints":   waypoints,
			"speed":       2.0,
			"alert_range": 8.0,
			"state":       "patrol",
		},
		UpdateRate: time.Millisecond * 100,
		Active:     true,
	}
}

// CreateSimpleTestConfig creates a simple configuration for testing
func CreateSimpleTestConfig(id, name string) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Simple Test Behavior",
			Root: &NodeConfig{
				Name:    "TestSequence",
				Type:    "Sequence",
				Enabled: true,
				Children: []*NodeConfig{
					{
						Name:    "LogStart",
						Type:    "Log",
						Enabled: true,
						Parameters: map[string]interface{}{
							"message": "Starting test sequence",
						},
					},
					{
						Name:    "SetCounter",
						Type:    "Action",
						Enabled: true,
						Parameters: map[string]interface{}{
							"action_type": "set_blackboard",
							"key":         "counter",
							"value":       0,
						},
					},
					{
						Name:    "IncrementCounter",
						Type:    "Action",
						Enabled: true,
						Parameters: map[string]interface{}{
							"action_type": "increment",
							"key":         "counter",
							"increment":   1,
						},
					},
					{
						Name:    "WaitBriefly",
						Type:    "Wait",
						Enabled: true,
						Parameters: map[string]interface{}{
							"duration": "1s",
						},
					},
					{
						Name:    "LogEnd",
						Type:    "Log",
						Enabled: true,
						Parameters: map[string]interface{}{
							"message": "Test sequence completed",
						},
					},
				},
			},
		},
		Sensors: []*SensorConfig{
			{
				Name:           "TimerSensor",
				Type:           "Timer",
				Enabled:        true,
				UpdateInterval: time.Millisecond * 100,
				Parameters: map[string]interface{}{
					"output_key": "timer",
				},
			},
		},
		InitialData: map[string]interface{}{
			"test_value": "hello world",
		},
		UpdateRate: time.Millisecond * 200,
		Active:     true,
	}
}

// CreateRepeatingBehaviorConfig creates a configuration with repeating behavior
func CreateRepeatingBehaviorConfig(id, name string) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Repeating Behavior",
			Root: &NodeConfig{
				Name:    "RepeatingSequence",
				Type:    "Repeater",
				Enabled: true,
				Parameters: map[string]interface{}{
					"max_repeats": 5,
				},
				Child: &NodeConfig{
					Name:    "RepeatedActions",
					Type:    "Sequence",
					Enabled: true,
					Children: []*NodeConfig{
						{
							Name:    "LogIteration",
							Type:    "Log",
							Enabled: true,
							Parameters: map[string]interface{}{
								"message": "Repeating iteration",
							},
						},
						{
							Name:    "IncrementCounter",
							Type:    "Action",
							Enabled: true,
							Parameters: map[string]interface{}{
								"action_type": "increment",
								"key":         "iteration_count",
								"increment":   1,
							},
						},
						{
							Name:    "ShortWait",
							Type:    "Wait",
							Enabled: true,
							Parameters: map[string]interface{}{
								"duration": "500ms",
							},
						},
					},
				},
			},
		},
		InitialData: map[string]interface{}{
			"iteration_count": 0,
		},
		UpdateRate: time.Millisecond * 100,
		Active:     true,
	}
}

// CreateParallelBehaviorConfig creates a configuration with parallel behavior
func CreateParallelBehaviorConfig(id, name string) *AgentConfig {
	return &AgentConfig{
		ID:   id,
		Name: name,
		BehaviorTree: &BehaviorTreeConfig{
			Name: "Parallel Behavior",
			Root: &NodeConfig{
				Name:    "ParallelTasks",
				Type:    "Parallel",
				Enabled: true,
				Parameters: map[string]interface{}{
					"success_policy": 2, // Need 2 children to succeed
					"failure_policy": 1, // 1 child failure = overall failure
				},
				Children: []*NodeConfig{
					{
						Name:    "Task1",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "LogTask1",
								Type:    "Log",
								Enabled: true,
								Parameters: map[string]interface{}{
									"message": "Executing Task 1",
								},
							},
							{
								Name:    "WaitTask1",
								Type:    "Wait",
								Enabled: true,
								Parameters: map[string]interface{}{
									"duration": "1s",
								},
							},
						},
					},
					{
						Name:    "Task2",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "LogTask2",
								Type:    "Log",
								Enabled: true,
								Parameters: map[string]interface{}{
									"message": "Executing Task 2",
								},
							},
							{
								Name:    "WaitTask2",
								Type:    "Wait",
								Enabled: true,
								Parameters: map[string]interface{}{
									"duration": "1.5s",
								},
							},
						},
					},
					{
						Name:    "Task3",
						Type:    "Sequence",
						Enabled: true,
						Children: []*NodeConfig{
							{
								Name:    "LogTask3",
								Type:    "Log",
								Enabled: true,
								Parameters: map[string]interface{}{
									"message": "Executing Task 3",
								},
							},
							{
								Name:    "WaitTask3",
								Type:    "Wait",
								Enabled: true,
								Parameters: map[string]interface{}{
									"duration": "0.8s",
								},
							},
						},
					},
				},
			},
		},
		UpdateRate: time.Millisecond * 50,
		Active:     true,
	}
}
