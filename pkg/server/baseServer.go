package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/MattSScott/basePlatformSOMAS/pkg/agent"
	"github.com/MattSScott/basePlatformSOMAS/pkg/message"
	"github.com/google/uuid"
)

type BaseServer[T agent.IAgent[T]] struct {
	//Max number of messages a single agent can send to others.
	messageLimit int
	// map of agentid -> agent struct
	agentMap map[uuid.UUID]T
	// map of agentid -> empty struct so that agents cannot access each others agent structs
	agentIdSet map[uuid.UUID]struct{}
	// channel a server goroutine will send to in order to signal messaging completion
	agentFinishedMessaging chan uuid.UUID
	// duration after which messaging phase forcefully ends during rounds
	turnTimeout time.Duration
	// interface which holds extended methods for round running and turn running
	gameRunner GameRunner
	// number of iterations for server
	iterations int
	// number of turns for server
	turns int
	// closable channel to signify that messaging is complete
	endNotifyAgentDone chan struct{}
	//map that stores channels which agents send messages to.
	//Note that sending/recieving to a channel is a read op on the map itself so no need to use sync.Map
	agentMessageChannels map[uuid.UUID]chan message.IMessage[T]
	//map that holds counters that track number of messages sent by each agent. Map is concurrently read/written so need to use sync.Map
	agentMessagesSent sync.Map
	//WaitGroup which makes the main thread wait before setting up all agent listening spinners
	//flag representing whether channels are closed and messages can be sent
	allowMessageSend          bool
	agentListenerSetupStaller sync.WaitGroup
	allowMessageLock          sync.RWMutex
}

func (server *BaseServer[T]) HandleStartOfTurn(iter, turn int) {
	server.agentFinishedMessaging = make(chan uuid.UUID)
	server.endNotifyAgentDone = make(chan struct{})
	fmt.Printf("Iteration %d, Turn %d starting...\n", iter, turn)
}

func (serv *BaseServer[T]) EndAgentListeningSession() bool {
	status := true
	ctx, cancel := context.WithTimeout(context.Background(), serv.turnTimeout)
	defer cancel()
	agentStoppedTalkingMap := make(map[uuid.UUID]struct{})
awaitSessionEnd:
	for len(agentStoppedTalkingMap) != len(serv.agentMap) {
		select {
		case id := <-serv.agentFinishedMessaging:
			agentStoppedTalkingMap[id] = struct{}{}

		case <-ctx.Done():
			status = false
			break awaitSessionEnd
		}
	}
	close(serv.endNotifyAgentDone)
	return status
}

func (server *BaseServer[T]) HandleEndOfTurn(iter, turn int) {
	if server.EndAgentListeningSession() {
		fmt.Println("All agents notified that they have finished messaging")
	} else {
		fmt.Println("All agents didn't notify that they have finished messaging, exited on timeout")
	}
	fmt.Printf("Iteration %d, Turn %d finished.\n", iter, turn)
}

func (server *BaseServer[T]) SendMessage(msg message.IMessage[T], receivers []uuid.UUID) {
	// sentMsgsUncast, _ := server.agentMessagesSent.Load(msg.GetSender())
	// sentMessageCount := sentMsgsUncast.(*int)
	// if *sentMessageCount >= server.messageLimit {
	// 	return
	// }
	for _, receiver := range receivers {
		server.allowMessageLock.RLock()
		if server.allowMessageSend {
			server.agentMessageChannels[receiver] <- msg
		}
		server.allowMessageLock.RUnlock()
	}
}

func (server *BaseServer[T]) BroadcastMessage(msg message.IMessage[T]) {
	agSet := server.ViewAgentIdSet()
	arrayRec := make([]uuid.UUID, len(agSet)-1)
	i := 0
	for id := range agSet {
		if id != msg.GetSender() {
			arrayRec[i] = id
			i++
		}
	}
	server.SendMessage(msg, arrayRec)
}

func (serv *BaseServer[T]) AddAgent(agent T) {
	msgCounter := 0
	serv.agentMap[agent.GetID()] = agent
	serv.agentIdSet[agent.GetID()] = struct{}{}
	serv.agentMessageChannels[agent.GetID()] = make(chan message.IMessage[T])
	serv.agentMessagesSent.Store(agent.GetID(), &msgCounter)
	serv.agentListenerSetupStaller.Add(1)
	go serv.agentMessageListener(agent.GetID())
}

func (serv *BaseServer[T]) ViewAgentIdSet() map[uuid.UUID]struct{} {
	return serv.agentIdSet
}

func (serv *BaseServer[T]) AccessAgentByID(id uuid.UUID) T {
	return serv.agentMap[id]
}

func (serv *BaseServer[T]) Start() {
	serv.checkHandler()
	for i := 0; i < serv.iterations; i++ {
		for j := 0; j < serv.turns; j++ {
			serv.HandleStartOfTurn(i+1, j+1)
			serv.gameRunner.RunTurn()
			serv.HandleEndOfTurn(i+1, j+1)
		}
	}
	serv.closeMessageChannels()
}

func (serv *BaseServer[T]) GetAgentMap() map[uuid.UUID]T {
	return serv.agentMap
}

func (serv *BaseServer[T]) AgentStoppedTalking(id uuid.UUID) {
	select {
	case serv.agentFinishedMessaging <- id:
		return
	case <-serv.endNotifyAgentDone:
		return

	}
}

func (serv *BaseServer[T]) SetGameRunner(handler GameRunner) {
	serv.gameRunner = handler
}

func (serv *BaseServer[T]) checkHandler() {
	if serv.gameRunner == nil {
		panic("round running handler has not been set. Have you run SetRunHandler?")
	}
}

func (serv *BaseServer[T]) RunTurn() {
	serv.gameRunner.RunTurn()
}

func (serv *BaseServer[T]) GetTurns() int {
	return serv.turns
}

func (serv *BaseServer[T]) RunIteration() {
	serv.gameRunner.RunIteration()
}

func (serv *BaseServer[T]) GetIterations() int {
	return serv.iterations
}

func (serv *BaseServer[T]) RemoveAgent(agentToRemove T) {
	delete(serv.agentMap, agentToRemove.GetID())
	delete(serv.agentIdSet, agentToRemove.GetID())
	delete(serv.agentMessageChannels, agentToRemove.GetID())
	serv.agentMessagesSent.Delete(agentToRemove.GetID())
}

func (serv *BaseServer[T]) GenerateAgentArrayFromMap() []T {
	agentMapToArray := make([]T, len(serv.agentMap))
	i := 0
	for _, ag := range serv.agentMap {
		agentMapToArray[i] = ag
		i++
	}
	return agentMapToArray
}

func (serv *BaseServer[T]) SendSynchronousMessage(msg message.IMessage[T], recipients []uuid.UUID) {
	for _, recip := range recipients {
		if msg.GetSender() == recip {
			continue
		}
		msg.InvokeMessageHandler(serv.agentMap[recip])
	}
}

func (serv *BaseServer[T]) RunSynchronousMessagingSession() {
	for _, agent := range serv.agentMap {
		agent.RunSynchronousMessaging()
	}
}

func (serv *BaseServer[T]) initialiseAgents(m []agent.AgentGeneratorCountPair[T]) {
	for _, pair := range m {
		for i := 0; i < pair.Count; i++ {
			agent := pair.Generator(serv)
			serv.AddAgent(agent)
		}
	}
}

// generate a server instance based on a mapping function and number of iterations
func CreateServer[T agent.IAgent[T]](generatorArray []agent.AgentGeneratorCountPair[T], iterations, turns int, turnMaxDuration time.Duration) *BaseServer[T] {
	serv := &BaseServer[T]{
		agentMap:               make(map[uuid.UUID]T),
		agentIdSet:             make(map[uuid.UUID]struct{}),
		turnTimeout:            turnMaxDuration,
		gameRunner:             nil,
		iterations:             iterations,
		turns:                  turns,
		agentFinishedMessaging: make(chan uuid.UUID),
		endNotifyAgentDone:     make(chan struct{}),
		agentMessageChannels:   make(map[uuid.UUID]chan message.IMessage[T]),
		messageLimit:           100,
		allowMessageSend:       true,
		allowMessageLock:       sync.RWMutex{},
	}
	fmt.Println("Initiliasing agents")
	serv.initialiseAgents(generatorArray)
	serv.agentListenerSetupStaller.Wait()
	return serv
}

func (server *BaseServer[T]) agentMessageListener(id uuid.UUID) {
	server.agentListenerSetupStaller.Done()

	for msg := range server.agentMessageChannels[id] {
		msg.InvokeMessageHandler(server.AccessAgentByID(id))
	}
}

func (server *BaseServer[T]) closeMessageChannels() {
	server.allowMessageLock.Lock()
	server.allowMessageSend = false
	for _, channel := range server.agentMessageChannels {
		close(channel)
	}
	server.allowMessageLock.Unlock()
}
