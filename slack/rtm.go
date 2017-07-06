package slack

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/nlopes/slack"

	"github.com/JaredReisinger/fizmo-slack/util"
)

type RTM struct {
	config   *util.Config
	logger   log.FieldLogger
	slackRTM *slack.RTM
	authInfo slack.AuthTestResponse
	selfLink string
	channels channelMap
	quit     chan bool
}

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
		// rtm.sendIntro(t.Channel, false)
		rtm.addChannel(t.Channel.ID, false)

	case *slack.ChannelLeftEvent:
		rtm.logger.WithFields(log.Fields{
			"channel": t.Channel,
			"user":    t.User,
		}).Info("left channel")

	case *slack.GroupJoinedEvent:
		rtm.logger.WithField("channel", t.Channel).Info("joined group (private channel)")
		// rtm.sendIntro(t.Channel, false)
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
	rtm.channels[channel] = NewChannel(rtm.config, channel, rtm.config.Logger)
	rtm.sendIntro(channel, initialStartup)
}

func (rtm *RTM) removeChannel(channel string) {
	c, ok := rtm.channels[channel]

	if !ok {
		rtm.logger.WithField("channel", channel).Warn("attempting to remove non-tracked channel")
		// REVIEW: post a message in this case?
		return
	}

	rtm.logger.WithField("channel", channel).Info("removing channel")
	c.Kill()
	delete(rtm.channels, channel)
}

func (rtm *RTM) sendIntro(channel string, initialStartup bool) {
	var format string

	if initialStartup {
		format = "Hi, everyone!  I’ve been asleep for a bit, but I’m awake again.  Just as a reminder, you can address me directly to get more help: `@%s help`"
	} else {
		format = "Hi, everyone!  Thanks for inviting me to the channel!  You can address me directly to get more help: `@%s help`"
	}

	msg := fmt.Sprintf(format, rtm.authInfo.User)
	rtm.sendMessage(channel, msg)
}

func (rtm *RTM) handleMessageEvent(msgEvent *slack.MessageEvent) {
	rtm.logger.WithFields(log.Fields{
		"channel":  msgEvent.Channel,
		"user":     msgEvent.User,
		"username": msgEvent.Username,
		"text":     msgEvent.Text,
	}).Debug("message")

	if msgEvent.User == "" || msgEvent.User == rtm.authInfo.UserID {
		rtm.logger.Debug("ignoring message from ourself...")
		return
	}

	command, ok := rtm.shouldBeCommand(msgEvent.Text)
	if !ok {
		// doesn't look like a command... ignore it!
		return
	}

	reply := fmt.Sprintf("It looks like you want me to try doing `%s`...", command)
	rtm.sendMessage(msgEvent.Channel, reply)
}

func (rtm *RTM) shouldBeCommand(text string) (string, bool) {
	return rtm.directlyAddressed(text)
}

func (rtm *RTM) directlyAddressed(text string) (string, bool) {
	// TODO: handle direct messages differently?  What about low-member-count
	// channels?  Should this be configurable?
	if !strings.HasPrefix(text, rtm.selfLink) {
		return "", false
	}

	command := strings.TrimPrefix(text, rtm.selfLink)
	command = strings.TrimSpace(command)
	return command, true
}

func (rtm *RTM) sendMessage(channel string, text string) {
	// All of the message-posting/sending APIs are gross, each in their own way.
	// You'd think there'd just be one that took a message object and sent it,
	// but they all take pieces and parts and cram them together.
	params := slack.NewPostMessageParameters()
	params.AsUser = false
	params.Username = fmt.Sprintf("%s (no game)", rtm.authInfo.User)
	params.EscapeText = false
	// params.Attachments = ...
	_, _, err := rtm.slackRTM.PostMessage(channel, text, params)
	if err != nil {
		rtm.logger.WithError(err).Error("posting message")
		return
	}
}