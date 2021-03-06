package slack

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/fizmo"
	"github.com/JaredReisinger/xyzzybot/games"
)

// Config for Slack components...
type Config struct {
	BotToken           string
	Admins             []string
	Logger             log.FieldLogger
	Games              games.Repository
	InterpreterFactory fizmo.InterpreterFactory
	WorkingRoot        string
}

// Manager ...
type Manager struct {
	// config   *util.Config
	config   *Config
	logger   log.FieldLogger
	slackRTM *slack.RTM
	authInfo slack.AuthTestResponse
	self     *slack.UserDetails
	selfLink string
	rooms    roomMap
	quit     chan bool
}

type roomMap map[string]*Room

// StartManager ...
func StartManager(config *Config) (manager *Manager, err error) {
	logger := config.Logger.WithField("component", "slack.manager")

	client := slack.New(config.BotToken)

	resp, err := client.AuthTest()
	if err != nil {
		logger.WithField("token", config.BotToken).WithError(err).Error("auth test")
		return
	}

	logger.WithField("response", resp).Debug("auth good!")

	manager = &Manager{
		config:   config,
		logger:   logger,
		slackRTM: client.NewRTM(),
		authInfo: *resp,
		selfLink: fmt.Sprintf("<@%s>", resp.UserID),
		rooms:    make(roomMap, 0),
		quit:     make(chan bool),
	}

	go manager.handleEvents()
	go manager.slackRTM.ManageConnection()

	return
}

// Disconnect ...
func (manager *Manager) Disconnect() {
	close(manager.quit)
}

func (manager *Manager) getActiveRoomLinks() (typeNames map[roomType][]string) {
	typeNames = make(map[roomType][]string, 0)
	for _, r := range manager.rooms {
		typeNames[r.roomType] = append(typeNames[r.roomType], r.link)
	}

	// Really, we want to be able to sort by display string (a.k.a r.name), but
	// that's currently lost when we aggregate the links.  We could either
	// custom-sort by the string *after* the `|`, or track both the link and the
	// name in the map.

	// TODO: fix sorting...
	// for _, names := range typeNames {
	// 	sort.Sort(sort.StringSlice(names))
	// }

	return
}

func (manager *Manager) handleEvents() {
	defer manager.slackRTM.Disconnect()

	for {
		select {
		case managerEvent := <-manager.slackRTM.IncomingEvents:
			// Always process events in a goroutine to keep this handler loop
			// unblocked and responsive!
			go manager.processEvent(managerEvent)

		case _, ok := <-manager.quit:
			manager.logger.WithField("ok", ok).Debug("Got quit")
			break
		}
	}

}

func createChannelLink(c *slack.Channel) string {
	return fmt.Sprintf("<#%s|%s>", c.ID, c.Name)
}

func createGroupLink(g *slack.Group) string {
	return fmt.Sprintf("<#%s|%s>", g.ID, g.Name)
}

func createUserLink(u *slack.User) string {
	return fmt.Sprintf("<@%s|%s>", u.ID, u.Name)
}

func (manager *Manager) processEvent(managerEvent slack.RTMEvent) {
	// manager.logger.WithFields(log.Fields{
	// 	"eventName": managerEvent.Type,
	// 	"eventData": managerEvent.Data,
	// }).Debug("Got event")

	switch t := managerEvent.Data.(type) {

	case *slack.ConnectedEvent:
		manager.handleConnectedEvent(t)

	case *slack.ChannelJoinedEvent:
		manager.logger.WithField("channel", t.Channel).Info("joined channel")
		manager.addRoom(t.Channel.ID, channelRoom, t.Channel.Name, createChannelLink(&t.Channel), false)

	case *slack.ChannelLeftEvent:
		manager.logger.WithFields(log.Fields{
			"channel": t.Channel,
			"user":    t.User,
		}).Info("left channel")

	case *slack.ChannelRenameEvent:
		// manager.logger.WithFields(log.Fields{
		// 	"id":   t.Channel.ID,
		// 	"name": t.Channel.Name,
		// }).Info("rename channel")
		manager.renameRoom(t.Channel.ID, t.Channel.Name)

	case *slack.FileSharedEvent:
		manager.logger.WithField("id", t.FileID).Info("file shared")
		manager.handleFileEvent(t)

	case *slack.GroupJoinedEvent:
		manager.logger.WithField("channel", t.Channel).Info("joined group (private channel)")
		manager.addRoom(t.Channel.ID, groupRoom, t.Channel.Name, createChannelLink(&t.Channel), false)

	case *slack.GroupLeftEvent:
		manager.logger.WithFields(log.Fields{
			"channel": t.Channel,
			"user":    t.User,
		}).Info("left group (private channel)")

	case *slack.GroupRenameEvent:
		// manager.logger.WithFields(log.Fields{
		// 	"id":   t.Channel.ID,
		// 	"name": t.Channel.Name,
		// }).Info("rename channel")
		manager.renameRoom(t.Group.ID, t.Group.Name)

	case *slack.MessageEvent:
		manager.handleMessageEvent(t)

		// default:
		// 	manager.logger.WithFields(log.Fields{
		// 		"eventName":     managerEvent.Type,
		// 		"eventDataType": fmt.Sprintf("%T", t),
		// 		"eventData":     t,
		// 	}).Debug("unhandled event")
	}
}

func (manager *Manager) handleConnectedEvent(connEvent *slack.ConnectedEvent) {
	info := connEvent.Info

	// manager.logger.WithField("info", info).Info("connected")
	manager.logger.Info("connected")

	// connEvent.Info.User includes details that may be better than those from
	// authtest...
	manager.self = info.User

	// info.Channels includes *all* public channels... we only care about the
	// ones of which we're a member.
	for _, c := range info.Channels {
		if c.IsMember {
			manager.logger.WithFields(log.Fields{
				"channel": c.Name,
				// "open":    c.IsOpen,
			}).Debug("adding channel")
			manager.addRoom(c.ID, channelRoom, c.Name, createChannelLink(&c), true)
		}
	}

	// info.Groups includes *only* groups (private channels) of which we're
	// already a member.
	for _, g := range info.Groups {
		manager.logger.WithFields(log.Fields{
			"group": g.Name,
			// "open":  g.IsOpen,
		}).Debug("adding group (private channel)")
		manager.addRoom(g.ID, groupRoom, g.Name, createGroupLink(&g), true)
	}

	for _, d := range info.IMs {
		user, err := manager.slackRTM.GetUserInfo(d.User)
		if err != nil {
			manager.logger.WithField("id", d.User).WithError(err).Error("getting user")
			continue
		}
		manager.logger.WithFields(log.Fields{
			"id": d.ID,
			// "open": d.IsOpen,
			"user": d.User,
			"name": user.Name,
			"bot":  user.IsBot,
		}).Debug("direct message (im) info")
		if d.User != "USLACKBOT" {
			manager.addRoom(d.ID, directRoom, user.Name, createUserLink(user), true)
		}
	}
}

func (manager *Manager) addRoom(id string, roomType roomType, name string, link string, initialStartup bool) {
	_, ok := manager.rooms[id]

	if ok {
		manager.logger.WithField("id", id).Warn("attempting to add existing room")
		// REVIEW: post a message in this case?
		return
	}

	manager.logger.WithField("id", id).Info("adding room")
	r := newRoom(manager.config, manager, id, roomType, name, link)
	manager.rooms[id] = r
	r.sendIntro(initialStartup)
}

func (manager *Manager) renameRoom(id string, name string) {
	r, ok := manager.rooms[id]

	if !ok {
		return
	}
	manager.logger.WithFields(log.Fields{
		"id":   id,
		"name": name,
	}).Info("renaming room")
	r.name = name
}

func (manager *Manager) removeRoom(channel string) {
	r, ok := manager.rooms[channel]

	if !ok {
		manager.logger.WithField("channel", channel).Warn("attempting to remove non-tracked channel")
		// REVIEW: post a message in this case?
		return
	}

	manager.logger.WithField("channel", channel).Info("removing channel")
	r.killGame()
	delete(manager.rooms, channel)
}

func (manager *Manager) handleFileEvent(fileEvent *slack.FileSharedEvent) {
	file, _, _, err := manager.slackRTM.Client.GetFileInfo(fileEvent.FileID, 0, 0)
	if err != nil {
		manager.logger.WithField("fileID", fileEvent.FileID).WithError(err).Error("getting file info")
		return
	}

	user, admin := manager.getUser(file.User)

	logger := manager.logger.WithFields(log.Fields{
		"file":   file.Name,
		"userID": file.User,
		"user":   user.Name,
	})

	// only allow from admins...
	if !admin {
		logger.Info("ignoring file from non-admin")
		return
	}

	if !manager.isExplicitlyToMe(file.InitialComment.Comment) &&
		!strings.Contains(file.InitialComment.Comment, "upload") {
		logger.Info("ignoring file without upload comment")
		return
	}

	logger.WithFields(log.Fields{
		"comment":            file.InitialComment.Comment,
		"commentUser":        file.InitialComment.User,
		"channels":           file.Channels,
		"groups":             file.Groups,
		"ims":                file.IMs,
		"filetype":           file.Filetype,
		"mimetype":           file.Mimetype,
		"url":                file.URL,
		"urldownload":        file.URLDownload,
		"urlprivate":         file.URLPrivate,
		"urlprivatedownload": file.URLPrivateDownload,
		"size":               file.Size,
		// "DUMP":               file,
	}).Info("file info")

	err = manager.downloadGame(file.URLPrivate, file.Name)
	if err != nil {
		manager.sendMessage(file.User, err.Error())
		return
	}

	manager.sendMessage(file.User, fmt.Sprintf("Upload complete!  The game *%s* is now available!", file.Name))
}

func (manager *Manager) downloadGame(uri string, filename string) error {
	logger := manager.logger.WithFields(log.Fields{
		"url":  uri,
		"name": filename,
	})

	logger.Debug("downloading game")
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		logger.WithError(err).Error("creating request")
		return fmt.Errorf("I wasn’t able to download %s... %s", uri, err.Error())
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", manager.config.BotToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Error("downloading game")
		return fmt.Errorf("I wasn’t able to download %s... %s", uri, err.Error())
	}
	defer resp.Body.Close()

	err = manager.config.Games.AddGameFile(filename, resp.Body)
	if err != nil {
		logger.WithField("game", filename).WithError(err).Error("saving game")
		return fmt.Errorf("I wasn't able to save %s to %s... %s", uri, filename, err.Error())
	}

	return nil
}

func (manager *Manager) deleteGame(filename string) error {
	logger := manager.logger.WithFields(log.Fields{
		"name": filename,
	})

	logger.Debug("deleting game")
	err := manager.config.Games.DeleteGameFile(filename)
	if err != nil {
		logger.WithError(err).Error("deleting game")
		return err
	}

	return nil
}

func (manager *Manager) handleMessageEvent(msgEvent *slack.MessageEvent) {
	// TODO: only turn on with increased verbosity?
	// manager.logger.WithFields(log.Fields{
	// 	"channel":  msgEvent.Channel,
	// 	"user":     msgEvent.User,
	// 	"username": msgEvent.Username,
	// 	"text":     msgEvent.Text,
	// }).Debug("message")

	if msgEvent.User == "" || msgEvent.User == manager.authInfo.UserID {
		// manager.logger.Debug("ignoring message from ourself...")
		return
	}

	// It's more efficient to perform the minimal evalution needed to determine
	// whether the message represents a command or not... but it's much harder
	// to read the logic that way.
	text := msgEvent.Text
	forSomeoneElse := manager.isForSomeoneElse(text)
	toUs := manager.isExplicitlyToMe(text)
	// Ensure any direct-address (to us) is stripped first...
	command := strings.TrimPrefix(text, manager.selfLink)
	command = strings.TrimSpace(command)
	looksLike := manager.looksLikeCommand(command)

	if forSomeoneElse || (!toUs && !looksLike) {
		return
	}

	r, ok := manager.rooms[msgEvent.Channel]
	if !ok {
		// Can this ever happen?
		manager.handleCommand(msgEvent, msgEvent.Channel, command)
	} else {
		r.handleCommand(msgEvent, command)
	}
}

func (manager *Manager) looksLikeCommand(text string) bool {
	words := strings.Fields(text)
	// If it's more than 4 words, it's *probably* not a command
	return len(words) <= 4
}

var userLink = regexp.MustCompile("<@([^>]+)>")

// a message is for someone else if they are directly mentioned
func (manager *Manager) isForSomeoneElse(text string) bool {
	matches := userLink.FindAllStringSubmatch(text, -1)

	// If *any* match is for not-us, we can be definite...
	for _, match := range matches {
		if len(match) > 1 && match[1] != manager.authInfo.UserID {
			return true
		}
	}

	return false
}

func (manager *Manager) getUser(userID string) (*slack.User, bool) {
	user, err := manager.slackRTM.GetUserInfo(userID)
	if err == nil {
		for _, a := range manager.config.Admins {
			if strings.EqualFold(user.Name, a) {
				return user, true
			}
		}
	}
	return user, false
}

func (manager *Manager) isExplicitlyToMe(text string) bool {
	// TODO: handle direct messages differently?  What about low-member-count
	// channels?  Should this be configurable?
	return strings.HasPrefix(text, manager.selfLink)
}

func (manager *Manager) handleCommand(msgEvent *slack.MessageEvent, channel string, command string) {
	reply := fmt.Sprintf("It looks like you want me to try doing `%s`... but for some reason I don’t already know about this channel (%s)!", command, channel)
	manager.sendMessage(channel, reply)
}

func (manager *Manager) sendMessage(channel string, text string) {
	manager.sendMessageWithStatus(channel, text, "")
}

func (manager *Manager) sendMessageWithStatus(channel string, text string, status string) {
	manager.sendMessageWithNameContext(channel, text, status, "")
}

func (manager *Manager) sendMessageWithNameContext(channel string, text string, status string, nameContext string) {
	// All of the message-posting/sending APIs are gross, each in their own way.
	// You'd think there'd just be one that took a message object and sent it,
	// but they all take pieces and parts and cram them together.
	params := slack.NewPostMessageParameters()
	params.AsUser = false
	if nameContext != "" {
		nameContext = fmt.Sprintf(" (%s)", nameContext)
	}
	params.Username = fmt.Sprintf("%s%s", manager.authInfo.User, nameContext)
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
	_, _, err := manager.slackRTM.PostMessage(channel, text, params)
	if err != nil {
		manager.logger.WithError(err).Error("posting message")
		return
	}
}
