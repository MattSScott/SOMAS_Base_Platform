package server

import (
	"context"
	"fmt"
	"time"

	"github.com/MattSScott/basePlatformSOMAS/v2/internal/diagnosticsEngine"
	"github.com/MattSScott/basePlatformSOMAS/v2/pkg/agent"
	"github.com/MattSScott/basePlatformSOMAS/v2/pkg/message"
	"github.com/google/uuid"
)

type BaseServer[T agent.IAgent[T]] struct {
	// map of agentid -> agent struct
	agentMap map[uuid.UUID]T
	// hashset of agent IDs
	agentIdSet map[uuid.UUID]struct{}
	// channel a server goroutine will send to in order to signal messaging completion
	agentFinishedMessaging chan uuid.UUID
	// duration after which messaging phase forcefully ends during turns
	turnTimeout time.Duration
	// interface which allows overridable turns
	gameRunner GameRunner
	// number of iterations for server
	iterations int
	// number of turns for server
	turns int
	// closable channel to signify that messaging is complete
	endNotifyAgentDone chan struct{}
	//the max number of sent messages the server will process concurrently from each agent at one time. Anymore sent will be dropped
	agentMessagingBandwidth int
	// diagnostic engine
	diagnosticsEngine diagnosticsEngine.IDiagnosticsEngine
	//flag which controls whether diagnostics are reported
	reportMessagingDiagnostics bool
}

func (server *BaseServer[T]) ReportMessagingDiagnostics() {
	server.reportMessagingDiagnostics = true
}

func (server *BaseServer[T]) handleStartOfTurn() {
	server.agentFinishedMessaging = make(chan uuid.UUID)
	server.endNotifyAgentDone = make(chan struct{})
}

func (serv *BaseServer[T]) endAgentListeningSession() bool {
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
	serv.diagnosticsEngine.ReportEndMessagingStatus(len(agentStoppedTalkingMap))
	close(serv.endNotifyAgentDone)
	return status
}

func (server *BaseServer[T]) reportDiagnostics() {
	msgSuccessRate := server.diagnosticsEngine.GetMessagingSuccessRate()
	numMsgSuccess := server.diagnosticsEngine.GetNumberMessageSuccesses()
	numMsgDrops := server.diagnosticsEngine.GetNumberMessageDrops()
	fmt.Printf("%f%% of messages successfully sent (%d delivered, %d dropped)\n", msgSuccessRate, numMsgSuccess, numMsgDrops)
	numAgents := len(server.GetAgentMap())
	numEndMsg := server.diagnosticsEngine.GetNumberEndMessagings()
	endMsgSuccess := server.diagnosticsEngine.GetEndMessagingSuccessRate(numAgents)
	fmt.Printf("%f%% of agents successfully ended messaging (%d ended, %d total)\n", endMsgSuccess, numEndMsg, numAgents)

}

func (server *BaseServer[T]) handleEndOfTurn() {
	server.endAgentListeningSession()
	if server.reportMessagingDiagnostics {
		server.reportDiagnostics()
	}
	server.diagnosticsEngine.ResetRoundDiagnostics()
}

func (server *BaseServer[T]) DeliverMessage(msg message.IMessage[T], recipient uuid.UUID) {
	msg.InvokeMessageHandler(server.agentMap[recipient])
}

func (serv *BaseServer[T]) AddAgent(agent T) {
	serv.agentMap[agent.GetID()] = agent
	serv.agentIdSet[agent.GetID()] = struct{}{}
}

func (serv *BaseServer[T]) ViewAgentIdSet() map[uuid.UUID]struct{} {
	return serv.agentIdSet
}

func (serv *BaseServer[T]) AccessAgentByID(id uuid.UUID) T {
	return serv.agentMap[id]
}

func (serv *BaseServer[T]) Start() {
	serv.checkGameRunner()
	for i := 0; i < serv.iterations; i++ {
		serv.gameRunner.RunStartOfIteration(i)
		for j := 0; j < serv.turns; j++ {
			serv.handleStartOfTurn()
			serv.gameRunner.RunTurn(i, j)
			serv.handleEndOfTurn()
		}
		serv.gameRunner.RunEndOfIteration(i)
	}
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

func (serv *BaseServer[T]) checkGameRunner() {
	if serv.gameRunner == nil {
		panic("Handler for running turn has not been set. Have you called SetGameRunner?")
	}
}

func (serv *BaseServer[T]) RunTurn(turn, iteration int) {
	panic("RunTurn not defined in server.")
}

func (serv *BaseServer[T]) RunStartOfIteration(iteration int) {
	panic("RunStartOfIteration not defined in server.")
}

func (serv *BaseServer[T]) RunEndOfIteration(iteration int) {
	panic("RunEndOfIteration not defined in server.")
}

func (serv *BaseServer[T]) GetTurns() int {
	return serv.turns
}

func (serv *BaseServer[T]) GetIterations() int {
	return serv.iterations
}

func (serv *BaseServer[T]) RemoveAgent(agentToRemove T) {
	delete(serv.agentMap, agentToRemove.GetID())
	delete(serv.agentIdSet, agentToRemove.GetID())
}

func (serv *BaseServer[T]) GetAgentMessagingBandwidth() int {
	return serv.agentMessagingBandwidth
}

func (serv *BaseServer[T]) GetDiagnosticEngine() diagnosticsEngine.IDiagnosticsEngine {
	return serv.diagnosticsEngine
}

// generate a server instance based on a mapping function and number of iterations
func CreateBaseServer[T agent.IAgent[T]](iterations, turns int, turnMaxDuration time.Duration, messageBandwidth int) *BaseServer[T] {
	return &BaseServer[T]{
		agentMap:                   make(map[uuid.UUID]T),
		agentIdSet:                 make(map[uuid.UUID]struct{}),
		turnTimeout:                turnMaxDuration,
		gameRunner:                 nil,
		iterations:                 iterations,
		turns:                      turns,
		agentFinishedMessaging:     make(chan uuid.UUID),
		endNotifyAgentDone:         make(chan struct{}),
		agentMessagingBandwidth:    messageBandwidth,
		diagnosticsEngine:          diagnosticsEngine.CreateDiagnosticsEngine(),
		reportMessagingDiagnostics: false,
	}
}
