package message

// // base interface structure used for message passing - can be composed for more complex message structures
type IAgentMessenger interface {
	GetAllMessages() []IMessage
}
