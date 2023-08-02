package agent2

import (
	"fmt"
	baseagent "somas_base_platform/pkg/agents/BaseAgent"
)

type Agent2 struct {
	*baseagent.BaseAgent
}

func (a2 *Agent2) Activity() {
	fmt.Println("Agent2's Activity")
	a2.BaseAgent.Activity()
}

func GetAgent() *Agent2 {
	return &Agent2{
		baseagent.NewAgent("A2"),
	}
}
