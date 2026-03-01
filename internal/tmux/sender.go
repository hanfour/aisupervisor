package tmux

import "fmt"

type Sender struct {
	client TmuxClient
}

func NewSender(client TmuxClient) *Sender {
	return &Sender{client: client}
}

func (s *Sender) Send(session string, window, pane int, keys string) error {
	return s.client.SendKeys(session, window, pane, keys)
}

func (s *Sender) SendWithEnter(session string, window, pane int, keys string) error {
	return s.client.SendKeys(session, window, pane, fmt.Sprintf("%s Enter", keys))
}

// SendLiteral sends literal text to a pane using tmux send-keys -l.
func (s *Sender) SendLiteral(session string, window, pane int, text string) error {
	return s.client.SendLiteralKeys(session, window, pane, text)
}
