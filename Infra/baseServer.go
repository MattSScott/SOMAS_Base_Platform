package infra

import (
	"fmt"

	"github.com/google/uuid"
)

type BaseServer[T IAgent[T]] struct {
	agentMap    map[uuid.UUID]T
	roundRunner RoundRunner
	iterations  int
}

func (bs *BaseServer[T]) GetAgentMap() map[uuid.UUID]T {
	return bs.agentMap
}

func (bs *BaseServer[T]) AddAgent(agentToAdd T) {
	bs.agentMap[agentToAdd.GetID()] = agentToAdd
}

func (bs *BaseServer[T]) RemoveAgent(agentToAdd T) {
	delete(bs.agentMap, agentToAdd.GetID())
}

func (bs *BaseServer[T]) GetIterations() int {
	return bs.iterations
}

func (bs *BaseServer[T]) RunGameLoop() {
	for id, agent := range bs.agentMap {
		fmt.Printf("Agent %s updating state \n", id)
		agent.UpdateAgentInternalState()
	}
}

func (bs *BaseServer[T]) Start() {
	fmt.Printf("Server initialised with %d agents \n", len(bs.agentMap))
	fmt.Print("\n")
	//LOOPS
	for i := 0; i < bs.iterations; i++ {
		fmt.Printf("Game Loop %d running... \n \n", i)
		fmt.Printf("Main game loop running... \n \n")
		bs.roundRunner.RunRound()
		fmt.Printf("\nMain game loop finished. \n \n")
		fmt.Printf("Messaging session started... \n \n")
		fmt.Printf("\nMessaging session completed \n \n")
		fmt.Printf("Game Loop %d completed. \n", i)
	}

}

type AgentGenerator[T IAgent[T]] func() T

type AgentGeneratorCountPair[T IAgent[T]] struct {
	generator AgentGenerator[T]
	count     int
}

func MakeAgentGeneratorCountPair[T IAgent[T]](generatorFunction AgentGenerator[T], count int) AgentGeneratorCountPair[T] {
	return AgentGeneratorCountPair[T]{
		generator: generatorFunction,
		count:     count,
	}
}

func (bs *BaseServer[T]) GenerateAgentArrayFromMap() []T {

	agentMapToArray := make([]T, len(bs.agentMap))

	i := 0
	for _, ag := range bs.agentMap {
		agentMapToArray[i] = ag
		i++
	}
	return agentMapToArray
}

func (bs *BaseServer[T]) SendSynchronousMessage(msg IMessage, recipients []uuid.UUID) {
	for _, recip := range recipients {
		if msg.GetSender() == recip {
			continue
		}
		msg.InvokeMessageHandler(recip)
	}

}

func (bs *BaseServer[T]) RunSynchronousMessagingSession() {
	for _, agent := range bs.agentMap {
		agent.RunSynchronousMessaging()
	}
}

// func (bs *BaseServer[T]) RunSynchronousMessagingSession() {
// 	for _, agent := range bs.agentMap {
// 		allMessages := agent.GetAllMessages()
// 		for _, msg := range allMessages {
// 			recipients := msg.GetRecipients()
// 			for _, recip := range recipients {
// 				if agent.GetID() == recip.GetID() {
// 					continue
// 				}
// 				msg.InvokeMessageHandler(recip)
// 			}
// 		}
// 	}
// }

func (bs *BaseServer[T]) initialiseAgents(m []AgentGeneratorCountPair[T]) {

	for _, pair := range m {
		for i := 0; i < pair.count; i++ {
			agent := pair.generator()
			bs.AddAgent(agent)
		}
	}

}

// generate a server instance based on a mapping function and number of iterations
func CreateServer[T IAgent[T]](generatorArray []AgentGeneratorCountPair[T], iterations int) *BaseServer[T] {
	serv := &BaseServer[T]{
		agentMap:   make(map[uuid.UUID]T),
		iterations: iterations,
	}
	serv.initialiseAgents(generatorArray)
	return serv
}
