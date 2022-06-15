package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/xanzy/go-gitlab"
)

type GitLabMR struct {
	namespace string
	project   string
	number    int
}

type apis struct {
	slackClient  *slack.Client
	gitlabClient *gitlab.Client
}

func main() {
	authToken := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	// slack client
	slackClient := slack.New(authToken, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	socketClient := socketmode.New(
		slackClient,
		socketmode.OptionDebug(true),
		// Option to set a custom logger
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// gitlab client
	gitlabToken := os.Getenv("GITLAB_ACCESS_TOKEN")
	customGitlabUrl := fmt.Sprintf("%s/api/v4", os.Getenv("CUSTOM_GITLAB_URL"))
	gitlabClient, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(customGitlabUrl))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	a := &apis{slackClient, gitlabClient}

	// Create a context that can be used to cancel goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context, socketClient *socketmode.Client) {
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
					err := a.handleEventMessage(eventsAPIEvent)
					if err != nil {
						log.Fatalf("Failed to handle message event: %v", err)
					}
				}

			}
		}
	}(ctx, socketClient)

	socketClient.Run()
}

func (a *apis) handleEventMessage(event slackevents.EventsAPIEvent) error {
	switch event.Type {
	// First we check if this is an CallbackEvent
	case slackevents.CallbackEvent:

		innerEvent := event.InnerEvent
		// Yet Another Type switch on the actual Data to see if its an AppMentionEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			if containsGitlabMR(ev.Text) {
				gitLabMRs, err := pluckUrls(ev.Text)
				if err != nil {
					return err
				}
				for _, mr := range gitLabMRs {
					fmt.Println("=======")
					// interact with GitLab API
					// add Reaction
					a.addReaction(ev.Channel, ev.TimeStamp)
					// fmt.Printf("%+v\n", mr)
					url := fmt.Sprintf("%s/%s", mr.namespace, mr.project)
					getOpts := gitlab.GetMergeRequestsOptions{}
					// get MR
					mergeRequest, _, mrErr := a.gitlabClient.MergeRequests.GetMergeRequest(url, mr.number, &getOpts)
					if mrErr != nil {
						return mrErr
					}
					fmt.Printf("%+v\n", mergeRequest)
					fmt.Println("=======")
				}
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func (a *apis) addReaction(channelID, timestamp string) error {
	msgRef := slack.NewRefToMessage(channelID, timestamp)
	err := a.slackClient.AddReaction("one", msgRef)
	if err != nil {
		return err
	}
	return nil
}

func containsGitlabMR(text string) bool {
	return strings.Contains(text, "gitlab") && strings.Contains(text, "-/merge_requests/")
}

func pluckUrls(text string) ([]*GitLabMR, error) {
	var urls []string
	var mrs []*GitLabMR
	textAry := strings.Split(text, "<")
	for _, v := range textAry {
		if containsGitlabMR(v) {
			ary := strings.Split(v, ">")
			urls = append(urls, ary[0])
		}
	}
	for _, url := range urls {
		mr, err := processUrl(url)
		if err != nil {
			return nil, err
		}
		mrs = append(mrs, mr)
	}
	return mrs, nil
}

func processUrl(url string) (*GitLabMR, error) {
	ary := strings.Split(url, "/")
	namespace := ary[len(ary)-5]
	project := ary[len(ary)-4]
	mrNumber := ary[len(ary)-1]
	number, err := strconv.Atoi(mrNumber)
	if err != nil {
		return nil, err
	}

	return &GitLabMR{
		namespace,
		project,
		number,
	}, nil
}
