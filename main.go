package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type GitLabInfo struct {
	group    string
	project  string
	mrNumber string
}

type payload struct {
	text string `json:"text"`
}

type data struct {
	payload payload `json:"payload"`
}

func main() {
	authToken := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	// slack client
	client := slack.New(authToken, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		// Option to set a custom logger
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Create a context that can be used to cancel goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
		// Create a for loop that selects either the context cancellation or the events incomming
		for {
			select {
			// inscase context cancel is called exit the goroutine
			case <-ctx.Done():
				log.Println("Shutting down socketmode listener")
				return
			case event := <-socketClient.Events:
				// We have a new Events, let's type switch the event
				// Add more use cases here if you want to listen to other events.
				switch event.Type {
				// handle EventAPI events
				case socketmode.EventTypeEventsAPI:
					// The Event sent on the channel is not the same as the EventAPI events so we need to type cast it
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
						continue
					}
					// We need to send an Acknowledge to the slack server
					socketClient.Ack(*event.Request)
					// Now we have an Events API event, but this event type can in turn be many types, so we actually need another type switch
					// log.Println(eventsAPIEvent)
					handleEventMessage(eventsAPIEvent, client)
				}

			}
		}
	}(ctx, client, socketClient)

	socketClient.Run()
}

func handleEventMessage(event slackevents.EventsAPIEvent, client *slack.Client) error {
	switch event.Type {
	// First we check if this is an CallbackEvent
	case slackevents.CallbackEvent:

		innerEvent := event.InnerEvent
		fmt.Println(innerEvent.Type)
		// Yet Another Type switch on the actual Data to see if its an AppMentionEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			if containsGitlabMR(ev.Text) {
				gitLabMRs := pluckUrls(ev.Text)
				for _, mr := range gitLabMRs {
					fmt.Println("=======")
					fmt.Printf("%+v\n", mr)
					fmt.Println("=======")
				}
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func containsGitlabMR(text string) bool {
	return strings.Contains(text, "gitlab") && strings.Contains(text, "-/merge_requests/")
}

func pluckUrls(text string) []*GitLabInfo {
	var urls []string
	var gitLabInfos []*GitLabInfo
	textAry := strings.Split(text, "<")
	for _, v := range textAry {
		if containsGitlabMR(v) {
			ary := strings.Split(v, ">")
			urls = append(urls, ary[0])
		}
	}
	for _, url := range urls {
		gitLabInfos = append(gitLabInfos, processUrl(url))
	}
	return gitLabInfos
}

func processUrl(url string) *GitLabInfo {
	ary := strings.Split(url, "/")
	group := ary[len(ary)-5]
	project := ary[len(ary)-4]
	mrNumber := ary[len(ary)-1]

	return &GitLabInfo{
		group,
		project,
		mrNumber,
	}
}
