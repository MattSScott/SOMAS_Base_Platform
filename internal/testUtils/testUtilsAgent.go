package testUtils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MattSScott/basePlatformSOMAS/v2/pkg/agent"
)

type ITestBaseAgent interface {
	agent.IAgent[ITestBaseAgent]
	CreateTestMessage() *TestMessage
	HandleTestMessage()
	ReceivedMessage() bool
	GetCounter() int32
	SetCounter(int32)
	GetGoal() int32
	SetGoal(int32)
	FinishedMessaging()
	SignalMessagingCompleteUnthreaded(*sync.WaitGroup, *uint32)
	GetAgentStoppedTalking() int
	HandleTimeoutTestMessage(msg TestTimeoutMessage)
	HandleInfiniteLoopMessage(msg TestMessagingBandwidthLimiter)
}

type TestServerFunctionsAgent struct {
	Counter        int32
	Goal           int32
	StoppedTalking int
	*agent.BaseAgent[ITestBaseAgent]
}

func (ta *TestServerFunctionsAgent) FinishedMessaging() {
	ta.StoppedTalking++
	ta.SignalMessagingComplete()
}

func (ta *TestServerFunctionsAgent) CreateTestMessage() *TestMessage {
	return &TestMessage{
		ta.CreateBaseMessage(),
		5,
	}
}

func (ta *TestServerFunctionsAgent) SignalMessagingCompleteUnthreaded(wg *sync.WaitGroup, counter *uint32) {
	defer wg.Done()
	ta.AgentStoppedTalking(ta.GetID())
	atomic.AddUint32(counter, 1)
}

func (ta TestServerFunctionsAgent) GetAgentStoppedTalking() int {
	return ta.StoppedTalking
}

func (ta *TestServerFunctionsAgent) HandleTestMessage() {
	newCounterValue := atomic.AddInt32(&ta.Counter, 1)
	if newCounterValue == atomic.LoadInt32(&ta.Goal) {
		ta.SignalMessagingComplete()
	}
}

func (ta *TestServerFunctionsAgent) ReceivedMessage() bool {
	return ta.Counter == ta.Goal
}

func NewTestAgent(serv agent.IExposedServerFunctions[ITestBaseAgent]) ITestBaseAgent {
	return &TestServerFunctionsAgent{
		BaseAgent:      agent.CreateBaseAgent(serv),
		Counter:        0,
		Goal:           0,
		StoppedTalking: 0,
	}
}

func (ta *TestServerFunctionsAgent) GetCounter() int32 {
	return ta.Counter
}

func (ta *TestServerFunctionsAgent) RunSynchronousMessaging() {
	newMsg := ta.CreateTestMessage()
	for id := range ta.ViewAgentIdSet() {
		ta.SendSynchronousMessage(newMsg, id)
	}
}

func (ta *TestServerFunctionsAgent) SetCounter(count int32) {
	ta.Counter = count
}

func (ta *TestServerFunctionsAgent) SetGoal(goal int32) {
	ta.Goal = goal
}
func (ta *TestServerFunctionsAgent) GetGoal() int32 {
	return ta.Goal
}

func (ta *TestServerFunctionsAgent) HandleTimeoutTestMessage(msg TestTimeoutMessage) {
	start := time.Now()
	time.Sleep(msg.Workload) // simulate long work
	fmt.Println("work has been completed, took ", time.Since(start), "notifying finished messaging")
	ta.SignalMessagingComplete()
}

func (ta *TestServerFunctionsAgent) HandleInfiniteLoopMessage(msg TestMessagingBandwidthLimiter) {
	// two or more agents sending to each other repeatedly will cause infinite recursive calls
	originalSender := msg.GetSender()
	// msg.SetSender(ta.GetID())
	ta.SendMessage(&msg, originalSender)
}
