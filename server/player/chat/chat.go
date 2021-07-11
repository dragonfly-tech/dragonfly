package chat

import (
	"sync"
)

// Global represents a global chat. Players will write in this chat by default when they send any message in
// the chat.
var Global = New()

// Chat represents the in-game chat. Messages may be written to it to send a message to all subscribers. The
// zero value of Chat is a chat ready to use.
// Methods on Chat may be called from multiple goroutines concurrently.
// Chat implements the io.Writer and io.StringWriter interfaces. fmt.Fprintf and fmt.Fprint may be used to write
// formatted messages to the chat.
type Chat struct {
	m           sync.Mutex
	subscribers map[Subscriber]struct{}
}

// New returns a new chat.
func New() *Chat {
	return &Chat{subscribers: map[Subscriber]struct{}{}}
}

// Write writes the byte slice p as a string to the chat. It is equivalent to calling
// Chat.WriteString(string(p)).
func (chat *Chat) Write(p []byte) (n int, err error) {
	return chat.WriteString(string(p))
}

// WriteString writes a string s to the chat.
func (chat *Chat) WriteString(s string) (n int, err error) {
	chat.m.Lock()
	defer chat.m.Unlock()
	for subscriber := range chat.subscribers {
		subscriber.Message(s)
	}
	return len(s), nil
}

// Subscribe adds a subscriber to the chat, sending it every message written to the chat. In order to remove
// it again, use Chat.Unsubscribe().
func (chat *Chat) Subscribe(s Subscriber) {
	chat.m.Lock()
	defer chat.m.Unlock()
	chat.subscribers[s] = struct{}{}
}

// Subscribed checks if a subscriber is currently subscribed to the chat.
func (chat *Chat) Subscribed(s Subscriber) bool {
	chat.m.Lock()
	defer chat.m.Unlock()
	_, ok := chat.subscribers[s]
	return ok
}

// Unsubscribe removes a subscriber from the chat, so that messages written to the chat will no longer be
// sent to it.
func (chat *Chat) Unsubscribe(s Subscriber) {
	chat.m.Lock()
	defer chat.m.Unlock()
	delete(chat.subscribers, s)
}

// Close closes the chat, removing all subscribers from it.
func (chat *Chat) Close() error {
	chat.m.Lock()
	chat.subscribers = nil
	chat.m.Unlock()
	return nil
}
