package messaging

import (
	"context"
	"log"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/slack-go/slack/slackevents"
)

// SlackMessenger implements the Messenger interface using Slack Socket Mode.
type SlackMessenger struct {
	api       *slack.Client
	socket    *socketmode.Client
	channelID string
	handler   CommandHandler
}

func NewSlackMessenger(botToken, appToken, channelID string) *SlackMessenger {
	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	socket := socketmode.New(api)
	return &SlackMessenger{
		api:       api,
		socket:    socket,
		channelID: channelID,
	}
}

func (s *SlackMessenger) OnCommand(handler CommandHandler) {
	s.handler = handler
}

func (s *SlackMessenger) Start(ctx context.Context) error {
	go func() {
		for evt := range s.socket.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				s.socket.Ack(*evt.Request)

				if eventsAPI.Type == slackevents.CallbackEvent {
					switch innerEvt := eventsAPI.InnerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						if s.handler != nil {
							reply := s.handler(innerEvt.Text)
							s.api.PostMessage(innerEvt.Channel, slack.MsgOptionText(reply, false))
						}
					}
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		// socket.Run doesn't have a clean shutdown, context cancel suffices
	}()

	if err := s.socket.RunContext(ctx); err != nil {
		if ctx.Err() != nil {
			return nil // context cancelled
		}
		log.Printf("Slack socket error: %v", err)
		return err
	}
	return nil
}

func (s *SlackMessenger) SendNotification(msg string) error {
	if s.channelID == "" {
		return nil
	}
	_, _, err := s.api.PostMessage(s.channelID, slack.MsgOptionText(msg, false))
	return err
}
