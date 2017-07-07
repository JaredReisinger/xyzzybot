package slack

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/nlopes/slack"

	"github.com/JaredReisinger/xyzzybot/util"
)

// RTM ...
type RTM struct {
	config   *util.Config
	logger   log.FieldLogger
	slackRTM *slack.RTM
	authInfo slack.AuthTestResponse
	selfLink string
	channels channelMap
	quit     chan bool
}

// StartRTM ...
func StartRTM(config *util.Config) (rtm *RTM, err error) {
	logger := config.Logger.WithField("component", "slack.rtm")

	client := slack.New(config.Slack.BotUserOAuthAccessToken)

	resp, err := client.AuthTest()
	if err != nil {
		logger.WithField("token", config.Slack.BotUserOAuthAccessToken).WithError(err).Error("auth test")
		return
	}

	logger.WithField("response", resp).Debug("auth good!")

	rtm = &RTM{
		config:   config,
		logger:   logger,
		slackRTM: client.NewRTM(),
		authInfo: *resp,
		selfLink: fmt.Sprintf("<@%s>", resp.UserID),
		channels: make(channelMap, 0),
		quit:     make(chan bool),
	}

	go rtm.handleEvents()
	go rtm.slackRTM.ManageConnection()

	return
}

// Disconnect ...
func (rtm *RTM) Disconnect() {
	close(rtm.quit)
}

func (rtm *RTM) handleEvents() {
	defer rtm.slackRTM.Disconnect()

	for {
		select {
		case rtmEvent := <-rtm.slackRTM.IncomingEvents:
			// Always process events in a goroutine to keep this handler loop
			// unblocked and responsive!
			go rtm.processEvent(rtmEvent)

		case _, ok := <-rtm.quit:
			rtm.logger.WithField("ok", ok).Debug("Got quit")
			break
		}
	}

}

func (rtm *RTM) processEvent(rtmEvent slack.RTMEvent) {
	// rtm.logger.WithFields(log.Fields{
	// 	"eventName": rtmEvent.Type,
	// 	"eventData": rtmEvent.Data,
	// }).Debug("Got event")

	switch t := rtmEvent.Data.(type) {

	case *slack.ConnectedEvent:
		rtm.handleConnectedEvent(t)

	case *slack.ChannelJoinedEvent:
		rtm.logger.WithField("channel", t.Channel).Info("joined channel")
		rtm.addChannel(t.Channel.ID, false)

	case *slack.ChannelLeftEvent:
		rtm.logger.WithFields(log.Fields{
			"channel": t.Channel,
			"user":    t.User,
		}).Info("left channel")

	case *slack.GroupJoinedEvent:
		rtm.logger.WithField("channel", t.Channel).Info("joined group (private channel)")
		rtm.addChannel(t.Channel.ID, false)

	case *slack.GroupLeftEvent:
		rtm.logger.WithFields(log.Fields{
			"channel": t.Channel,
			"user":    t.User,
		}).Info("left group (private channel)")

	case *slack.MessageEvent:
		rtm.handleMessageEvent(t)

		// default:
		// 	rtm.logger.WithFields(log.Fields{
		// 		"eventName":     rtmEvent.Type,
		// 		"eventDataType": fmt.Sprintf("%T", t),
		// 		"eventData":     t,
		// 	}).Debug("unhandled event")
	}
}

func (rtm *RTM) handleConnectedEvent(connEvent *slack.ConnectedEvent) {
	// rtm.logger.WithField("info", connEvent.Info).Info("connected")
	rtm.logger.Info("connected")
	for _, c := range connEvent.Info.Channels {
		// rtm.logger.WithFields(log.Fields{
		// 	"channel": c.Name,
		// 	"member":  c.IsMember,
		// 	"open":    c.IsOpen,
		// }).Debug("channel info")
		if c.IsMember {
			rtm.addChannel(c.ID, true)
		}
	}
	for _, g := range connEvent.Info.Groups {
		// rtm.logger.WithFields(log.Fields{
		// 	"channel": g.Name,
		// 	"open":    g.IsOpen,
		// }).Debug("group info")
		rtm.addChannel(g.ID, true)
	}
}

func (rtm *RTM) addChannel(channel string, initialStartup bool) {
	_, ok := rtm.channels[channel]

	if ok {
		rtm.logger.WithField("channel", channel).Warn("attempting to add existing channel")
		// REVIEW: post a message in this case?
		return
	}

	rtm.logger.WithField("channel", channel).Info("adding channel")
	c := NewChannel(rtm.config, rtm, channel, rtm.config.Logger)
	rtm.channels[channel] = c
	c.sendIntro(initialStartup)
}

func (rtm *RTM) removeChannel(channel string) {
	c, ok := rtm.channels[channel]

	if !ok {
		rtm.logger.WithField("channel", channel).Warn("attempting to remove non-tracked channel")
		// REVIEW: post a message in this case?
		return
	}

	rtm.logger.WithField("channel", channel).Info("removing channel")
	c.killGame()
	delete(rtm.channels, channel)
}

func (rtm *RTM) handleMessageEvent(msgEvent *slack.MessageEvent) {
	// TODO: only turn on with increased verbosity?
	// rtm.logger.WithFields(log.Fields{
	// 	"channel":  msgEvent.Channel,
	// 	"user":     msgEvent.User,
	// 	"username": msgEvent.Username,
	// 	"text":     msgEvent.Text,
	// }).Debug("message")

	if msgEvent.User == "" || msgEvent.User == rtm.authInfo.UserID {
		// rtm.logger.Debug("ignoring message from ourself...")
		return
	}

	// It's more efficient to perform the minimal evalution needed to determine
	// whether the message represents a command or not... but it's much harder
	// to read the logic that way.
	text := msgEvent.Text
	forSomeoneElse := rtm.isForSomeoneElse(text)
	toUs := rtm.isExplicitlyToUs(text)
	// Ensure any direct-address (to us) is stripped first...
	command := strings.TrimPrefix(text, rtm.selfLink)
	command = strings.TrimSpace(command)
	looksLike := rtm.looksLikeCommand(command)

	if forSomeoneElse || (!toUs && !looksLike) {
		return
	}

	c, ok := rtm.channels[msgEvent.Channel]
	if !ok {
		// Can this ever happen?
		rtm.handleCommand(msgEvent.Channel, command)
	} else {
		c.handleCommand(command)
	}
}

func (rtm *RTM) looksLikeCommand(text string) bool {
	words := strings.Fields(text)
	// If it's more than 4 words, it's *probably* not a command
	return len(words) <= 4
}

var userLink = regexp.MustCompile("<@([^>]+)>")

// a message is for someone else if they are directly mentioned
func (rtm *RTM) isForSomeoneElse(text string) bool {
	matches := userLink.FindAllStringSubmatch(text, -1)

	// If *any* match is for not-us, we can be definite...
	for _, match := range matches {
		if len(match) > 1 && match[1] != rtm.authInfo.UserID {
			return true
		}
	}

	return false
}

func (rtm *RTM) isExplicitlyToUs(text string) bool {
	// TODO: handle direct messages differently?  What about low-member-count
	// channels?  Should this be configurable?
	return strings.HasPrefix(text, rtm.selfLink)
}

func (rtm *RTM) handleCommand(channel string, command string) {
	reply := fmt.Sprintf("It looks like you want me to try doing `%s`... but for some reason I don’t already know about this channel!", command)
	rtm.sendMessage(channel, reply)
}

func (rtm *RTM) sendMessage(channel string, text string) {
	rtm.sendMessageWithStatus(channel, text, "")
}

func (rtm *RTM) sendMessageWithStatus(channel string, text string, status string) {
	rtm.sendMessageWithNameContext(channel, text, status, "")
}

func (rtm *RTM) sendMessageWithNameContext(channel string, text string, status string, nameContext string) {
	// All of the message-posting/sending APIs are gross, each in their own way.
	// You'd think there'd just be one that took a message object and sent it,
	// but they all take pieces and parts and cram them together.
	params := slack.NewPostMessageParameters()
	params.AsUser = false
	if nameContext != "" {
		nameContext = fmt.Sprintf(" (%s)", nameContext)
	}
	params.Username = fmt.Sprintf("%s%s", rtm.authInfo.User, nameContext)
	params.EscapeText = false

	if status != "" {
		// We represent status window text as an attachment footer, because
		// that's a pretty good analog for the top-of-window placement in a
		// fixed screen.
		params.Attachments = []slack.Attachment{
			slack.Attachment{
				Footer: status,
			},
		}
	}
	// params.Attachments = ...
	_, _, err := rtm.slackRTM.PostMessage(channel, text, params)
	if err != nil {
		rtm.logger.WithError(err).Error("posting message")
		return
	}
}
