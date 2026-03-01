package messaging

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
)

// LineMessenger implements the Messenger interface using LINE webhook + push messages.
type LineMessenger struct {
	bot          *messaging_api.MessagingApiAPI
	channelSecret string
	notifyUserID  string
	port          int
	handler       CommandHandler
}

func NewLineMessenger(channelSecret, channelToken, notifyUserID string, port int) (*LineMessenger, error) {
	bot, err := messaging_api.NewMessagingApiAPI(channelToken)
	if err != nil {
		return nil, fmt.Errorf("creating LINE bot: %w", err)
	}
	if port == 0 {
		port = 8080
	}
	return &LineMessenger{
		bot:           bot,
		channelSecret: channelSecret,
		notifyUserID:  notifyUserID,
		port:          port,
	}, nil
}

func (l *LineMessenger) OnCommand(handler CommandHandler) {
	l.handler = handler
}

func (l *LineMessenger) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.handleWebhook)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", l.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	log.Printf("LINE webhook server listening on :%d", l.port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (l *LineMessenger) handleWebhook(w http.ResponseWriter, r *http.Request) {
	cb, err := webhook.ParseRequest(l.channelSecret, r)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	for _, event := range cb.Events {
		switch e := event.(type) {
		case webhook.MessageEvent:
			switch msg := e.Message.(type) {
			case webhook.TextMessageContent:
				if l.handler != nil {
					reply := l.handler(msg.Text)
					replyToken := e.ReplyToken
					l.bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
						ReplyToken: replyToken,
						Messages: []messaging_api.MessageInterface{
							&messaging_api.TextMessage{Text: reply},
						},
					})
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (l *LineMessenger) SendNotification(msg string) error {
	if l.notifyUserID == "" {
		return nil
	}
	_, err := l.bot.PushMessage(&messaging_api.PushMessageRequest{
		To: l.notifyUserID,
		Messages: []messaging_api.MessageInterface{
			&messaging_api.TextMessage{Text: msg},
		},
	}, "")
	return err
}
