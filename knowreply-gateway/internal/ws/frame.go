package ws

// 帧类型，对应 README §5.1 的 WS 消息协议。
const (
	TypeUserMessage    = "user_message"
	TypeAssistantDelta = "assistant_delta"
	TypeAssistantDone  = "assistant_done"
	TypeTransferHuman  = "transfer_human"
	TypeError          = "error"
)

// Frame 是 WS 上双向传输的 JSON 帧，字段按场景按需填充。
type Frame struct {
	Type      string   `json:"type"`
	SessionID string   `json:"sessionId,omitempty"`
	MsgID     string   `json:"msgId,omitempty"`
	Content   string   `json:"content,omitempty"`
	Delta     string   `json:"delta,omitempty"`
	Citations []string `json:"citations,omitempty"`
	NeedHuman bool     `json:"needHuman,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	TicketID  string   `json:"ticketId,omitempty"`
	Code      string   `json:"code,omitempty"`
	Message   string   `json:"message,omitempty"`
}
