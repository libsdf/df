/*
Definitions of kinds of messages that used by transport.
*/
package message

const (
	KindData            uint8 = 0
	KindRequestDNS      uint8 = 1
	KindResultDNS       uint8 = 2
	KindRequestDial     uint8 = 4
	KindConnectionState uint8 = 8
	KindPingPong        uint8 = 16
)

type Message interface {
	Kind() byte
	MessageId() uint32
	AckMessageId() uint32
}

type MessageHandler func(Message)

type MessageRequestDNS struct {
	Id         uint32 `json:"id"`
	Network    string `json:"network,omitempty"`
	DomainName string `json:"domain_name,omitempty"`
}

func (m *MessageRequestDNS) Kind() byte {
	return KindRequestDNS
}

func (m MessageRequestDNS) MessageId() uint32 {
	return m.Id
}

func (m MessageRequestDNS) AckMessageId() uint32 {
	return 0
}

type MessageResultDNS struct {
	Id        uint32
	Ack       uint32
	Addresses [][]byte
	Error     string
}

func (m *MessageResultDNS) Kind() byte {
	return KindResultDNS
}

func (m MessageResultDNS) MessageId() uint32 {
	return m.Id
}

func (m MessageResultDNS) AckMessageId() uint32 {
	return m.Ack
}

type MessageRequestDial struct {
	Id           uint32 `json:"id"`
	Network      string `json:"network,omitempty"`
	Address      string `json:"address,omitempty"`
	ConnectionId string `json:"connection_id,omitempty"`
}

func (m *MessageRequestDial) Kind() byte {
	return KindRequestDial
}

func (m MessageRequestDial) MessageId() uint32 {
	return m.Id
}

func (m MessageRequestDial) AckMessageId() uint32 {
	return 0
}

type MessageConnectionState struct {
	Id           uint32 `json:"id"`
	Ack          uint32 `json:"ack"`
	ConnectionId string `json:"connection_id,omitempty"`
	State        string `json:"state,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
}

func (m *MessageConnectionState) Kind() byte {
	return KindConnectionState
}

func (m MessageConnectionState) MessageId() uint32 {
	return m.Id
}

func (m MessageConnectionState) AckMessageId() uint32 {
	return m.Ack
}

type MessageData struct {
	Id           uint32
	Ack          uint32
	ConnectionId string
	Data         []byte
}

func (m *MessageData) Kind() byte {
	return KindData
}

func (m *MessageData) MessageId() uint32 {
	return m.Id
}

func (m *MessageData) AckMessageId() uint32 {
	return m.Ack
}

type MessagePing struct {
	Id  uint32
	Ack uint32
}

func (m *MessagePing) Kind() byte {
	return KindPingPong
}

func (m *MessagePing) MessageId() uint32 {
	return m.Id
}

func (m *MessagePing) AckMessageId() uint32 {
	return m.Ack
}
