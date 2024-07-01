package log

type silentHandler struct {
}

func (h *silentHandler) Write(msg *Message) {
}

func (h *silentHandler) CleanUp() {
}
