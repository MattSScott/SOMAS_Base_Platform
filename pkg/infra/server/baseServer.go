package infra

import (
	"fmt"

	baseAgent "github.com/MattSScott/basePlatformSOMAS/pkg/agents/BaseAgent"
)

type BaseServer[T baseAgent.Agent] struct {
	NumAgents int
	NumTurns  int
	Agents    []T
}

func (bs *BaseServer[T]) RunGameLoop() {
	fmt.Printf("%d agents initialised: \n", bs.NumAgents)
	for index, agent := range bs.Agents {
		fmt.Printf("agent %d  \n", index)
		fmt.Printf("_____________________________________________ \n")
		agent.UpdateAgent()
		agent.Activity()
		//TO DO: add the function for stages

	}

}

func (bs *BaseServer[T]) Start() {
	//LOOPS
	for i := 0; i < bs.NumTurns; i++ {
		fmt.Printf("Game Loop %d Running \n", i)
		bs.RunGameLoop()
	}

}

type AgentGenerator[T baseAgent.Agent] func() T

type AgentGeneratorCountPair[T baseAgent.Agent] struct {
	generator AgentGenerator[T]
	count     int
}

func MakeAgentGeneratorCountPair[T baseAgent.Agent](generatorFunction AgentGenerator[T], count int) AgentGeneratorCountPair[T] {
	return AgentGeneratorCountPair[T]{
		generator: generatorFunction,
		count:     count,
	}
}

func (bs *BaseServer[T]) initialiseAgents(m []AgentGeneratorCountPair[T]) {

	agents := make([]T, getNumAgents(m))
	agentCount := 0

	for _, pair := range m {
		for i := 0; i < pair.count; i++ {
			agents[agentCount] = pair.generator()
			agentCount++
		}
	}

	bs.Agents = agents
	bs.NumAgents = len(agents)
}

func getNumAgents[T baseAgent.Agent](pairs []AgentGeneratorCountPair[T]) int {

	numAgents := 0

	for _, pair := range pairs {
		numAgents += pair.count
	}

	return numAgents
}

func CreateServer[T baseAgent.Agent](mapper []AgentGeneratorCountPair[T], numTurns int) *BaseServer[T] {
	// generate the server and return it
	serv := &BaseServer[T]{
		NumTurns: numTurns,
	}
	serv.initialiseAgents(mapper)
	return serv
}
