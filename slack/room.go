package slack

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/interpreter"
	"github.com/JaredReisinger/xyzzybot/util"
)

const (
	metaCommandPrefix = "!"
)

// The term "channel" has specific meaning for Slack, moreso than just the
// generic "communications channel" it might otherwise.  For Slack, a "channel"
// is public, what would be a private channel is really a "group", and there is
// also "im" (instant- or direct messages) and "mpim" (multi-party instant
// message).  From our standpoint, these are all the same (the IDs can be used
// with PostMessage()), but there are different APIs for getting information
// about them.
//
// To aid in comprehension, we instead use an analog for "communications
// channel"... and keeping in theme with interactive fiction, it's "room".

type roomType int

const (
	channelRoom roomType = iota
	groupRoom
	directRoom
)

// Room represents a Slack channel (public, or private (group), or user
// direct-message) to which we're connected.
type Room struct {
	ID       string
	roomType roomType
	name     string
	link     string // formatted `<#C1234|foo>` or `<@U1234|bob>` link
	config   *util.Config
	rtm      *RTM
	interp   *interpreter.Interpreter
	logger   log.FieldLogger
}

func newRoom(config *util.Config, rtm *RTM, id string, roomType roomType, name string, link string, logger log.FieldLogger) *Room {
	return &Room{
		ID:       id,
		roomType: roomType,
		name:     name,
		link:     link,
		config:   config,
		rtm:      rtm,
		logger: logger.WithFields(log.Fields{
			"component": "slack",
			"room":      name,
			"roomID":    id,
		}),
	}
}

func (r *Room) startGame(name string) error {
	if r.gameInProgress() {
		err := errors.New("game already in progress, ignoring start-game request")
		r.logger.WithError(err).Error("starting game")
		return err
	}

	// Create a new interpreter for the requested game...
	gameFile := path.Join(r.config.GameDirectory, name)
	r.logger.WithField("gameFile", gameFile).Info("starting game")
	i, err := interpreter.NewInterpreter(r.config, gameFile, r.logger)
	if err != nil {
		r.logger.WithError(err).Error("starting interpreter")
		return err
	}

	go r.listenForGameOutput(i.Output)

	err = i.Start()
	if err != nil {
		r.logger.WithError(err).Error("starting interpreter")
		return err
	}

	r.interp = i
	return nil
}

func (r *Room) listenForGameOutput(outchan chan *interpreter.GlkOutput) {
	r.logger.Info("setting up game output handler")
	for {
		output := <-outchan
		if output == nil {
			r.logger.Warn("game output has been closed")
			r.killGame()
			return
		}
		debugOutput := r.debugFormat(output)
		r.logger.WithField("output", debugOutput).Debug("recieved output")

		r.sendOutputMessage(output)
	}
}

func (r *Room) sendOutputMessage(output *interpreter.GlkOutput) {
	lines := []string{}
	status := ""

	for _, w := range output.Windows {
		windowText := formatWindow(w)

		// If the window looks like a status window, save its text separately as
		// status.
		if inferStatusWindow(w) {
			status = windowText
		} else {
			lines = append(lines, formatWindow(w))
		}
	}

	text := strings.Join(lines, "\n")
	leader := ""

	// horribly gross special handling to make Slack pay attention to leading
	// whitespace:
	if len(text) > 0 {
		if text[0] == '\n' {
			leader = "."
		} else if text[0] == ' ' {
			leader = ".\n"
		}
	}

	msg := fmt.Sprintf("%s%s", leader, text)
	r.sendMessageWithNameContext(msg, status, "game output")
}

func inferStatusWindow(w *interpreter.GlkWindow) bool {
	return w.Type == interpreter.TextGridWindowType &&
		w.Top == 0 &&
		w.Height <= 5
}

func (r *Room) debugFormat(output *interpreter.GlkOutput) string {
	sep1 := "============================================================"
	sep2 := "------------------------------------------------------------"
	lines := []string{sep1}

	for _, w := range output.Windows {
		lines = append(lines, formatWindow(w))
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	return strings.Join(lines, "\n")
}

func (r *Room) killGame() {
	r.logger.Info("recieved killGame request")

	if r.interp != nil {
		// TODO: stop listening to random stuff?
		r.interp.Kill()
		r.interp = nil
	}
}

func (r *Room) sendIntro(initialStartup bool) {
	var format string

	if initialStartup {
		// format = "Hi, everyone!  I’ve been asleep for a bit, but I’m awake again.  Just as a reminder, you can address me directly to get more help: `@%s help`"
	} else {
		format = "Hi, everyone!  Thanks for inviting me!  You can address me directly to get more help: `@%s help`"
	}

	if format != "" {
		msg := fmt.Sprintf(format, r.rtm.authInfo.User)
		r.sendMessage(msg)
	}
}

func (r *Room) sendMessage(text string) {
	r.rtm.sendMessage(r.ID, text)
}

// func (r *Room) sendMessageWithStatus(text string, status string) {
// 	r.rtm.sendMessageWithStatus(r.ID, text, status)
// }

func (r *Room) sendMessageWithNameContext(text string, status string, nameContext string) {
	r.rtm.sendMessageWithNameContext(r.ID, text, status, nameContext)
}

type commandHandler func(userID string, command string, args ...string)

func (r *Room) handleCommand(userID string, command string) {
	// If we have an interpreter, it gets the command.  Otherwise (or if there's
	// a leading metaCommandPrefix), it's a meta-command.

	if r.gameInProgress() && !strings.HasPrefix(command, metaCommandPrefix) {
		r.interp.SendLine(command)
		return
	}

	command = strings.TrimPrefix(command, metaCommandPrefix)

	// right now, we only do super-simple command parsing...
	words := strings.Fields(command)
	dispatch := map[string]commandHandler{
		"help":   r.commandHelp,
		"status": r.commandStatus,
		"list":   r.commandList,
		"play":   r.commandPlay,
		"kill":   r.commandKill,
		"space":  r.commandSpace,
		"key":    r.commandKey,
	}

	handler, ok := dispatch[words[0]]
	if !ok {
		handler = r.commandUnknown
	}

	handler(userID, words[0], words[1:]...)
}

func (r *Room) commandHelp(userID string, command string, args ...string) {
	msg := fmt.Sprintf("Hi!  I’m %[1]s, and I exist to help you experience the world of interactive fiction.\n\nWhen there’s a game in progress, I’ll assume that any comments directed my way are actually meant for the game, and I’ll pass them along.  If you really want to reach me directly, slap a `%[2]s` at the begining, like `@%[1]s /help` to see this message again.\n\nWhen there’s _not_ a game underway, or if your `%[2]s`-prefix your message, I can help with the following:\n* `help` - this message\n* `status` - operational status about myself\n* `list` - list the available games\n* `play game-name` - start _game-name_\n* `kill` - kill an in-progress game\n* `%[2]sspace` - send a space character to the game (needed for some prompts)\n* `%[2]skey` - send a raw key to the game: `%[2]skey` sends a space, and `%[2]skey x` sends `x`", r.rtm.authInfo.User, metaCommandPrefix)
	r.sendMessage(msg)
}

func (r *Room) commandStatus(userID string, command string, args ...string) {
	admin := false
	user, err := r.rtm.slackRTM.GetUserInfo(userID)
	if err == nil {
		for _, a := range r.config.Slack.Admins {
			if strings.EqualFold(user.Name, a) {
				admin = true
				break
			}
		}
	}

	var inProgress string
	if r.gameInProgress() {
		inProgress = "There *is* currently a game in progress."
	} else {
		inProgress = "There *is not* currently a game in progress."
	}

	typeLinks := r.rtm.getActiveRoomLinks()

	channelList := formatRoomList(typeLinks[channelRoom], "channel")
	var roomList string
	if r.roomType == directRoom && admin {
		roomList = fmt.Sprintf("I am participating in %s; %s; and chatting with %s", channelList, formatRoomList(typeLinks[groupRoom], "group"), formatRoomList(typeLinks[directRoom], "user"))

	} else {
		privateCount := len(typeLinks[groupRoom]) + len(typeLinks[directRoom])
		privates := ""
		if privateCount > 0 {
			privates = fmt.Sprintf("in %d private conversations, and ", privateCount)
		}
		roomList = fmt.Sprintf("I am %sparticipating in %s.", privates, channelList)
	}

	msg := fmt.Sprintf("%s\n\n%s", roomList, inProgress)

	r.logger.WithField("status", msg).Debug("sending status")
	r.sendMessage(msg)
}

func formatRoomList(list []string, label string) string {
	switch len(list) {
	case 0:
		return fmt.Sprintf("_no %ss_", label)

	case 1:
		return fmt.Sprintf("%s %s", label, list[0])

	case 2:
		return fmt.Sprintf("%ss %s and %s", label, list[0], list[1])
	}

	most := list[0 : len(list)-2]
	last := list[len(list)-1]
	return fmt.Sprintf("%ss %s, and %s", label, strings.Join(most, ", "), last)
}

func (r *Room) commandList(userID string, command string, args ...string) {
	dir, err := os.Open(r.config.GameDirectory)
	if err != nil {
		r.logger.WithField("path", r.config.GameDirectory).WithError(err).Error("unable to open game directory")
		r.sendMessage("I’m sorry, I wasn’t able to get to the list of games.  Please let XXX know something needs to be tweaked!")
		return
	}

	infos, err := dir.Readdir(-1)
	if err != nil {
		r.logger.WithField("path", r.config.GameDirectory).WithError(err).Error("unable to open game directory")
		r.sendMessage("I’m sorry, I wasn’t able to get the list of games.  Please let XXX know something needs to be tweaked!")
		return
	}

	files := make([]string, 0, len(infos))

	for _, info := range infos {
		if info.Mode().IsRegular() {
			files = append(files, info.Name())
		}
	}

	warning := ""

	if r.gameInProgress() {
		warning = "\n\n_Do note that there's currently a game in progress; you’ll need to finish or `/kill` it before you can start a new game._"
	}

	msg := fmt.Sprintf("The following games are currently available:\n* `%s`\n\nYou can start a game using `@%s play game-name`%s", strings.Join(files, "`\n* `"), r.rtm.authInfo.User, warning)
	r.sendMessage(msg)
}

func (r *Room) commandPlay(userID string, command string, args ...string) {
	if r.gameInProgress() {
		r.sendMessage("_There's currently a game in progress; you’ll need to finish or `/kill` it before you can start a new game._")
		return
	}

	r.startGame(args[0])
}

func (r *Room) commandKill(userID string, command string, args ...string) {
	if !r.gameInProgress() {
		r.sendMessage("There's _not_ currently a game in progress!")
		return
	}

	r.killGame()
}

func (r *Room) commandSpace(userID string, command string, args ...string) {
	if !r.gameInProgress() {
		r.sendMessage("There's _not_ currently a game in progress!")
		return
	}

	r.interp.SendKey(" ")
}

func (r *Room) commandKey(userID string, command string, args ...string) {
	if !r.gameInProgress() {
		r.sendMessage("There's _not_ currently a game in progress!")
		return
	}

	key := " "
	if len(args) > 0 {
		key = args[0]
	}

	r.interp.SendKey(key)
}

func (r *Room) commandUnknown(userID string, command string, args ...string) {
	r.logger.WithField("command", command).Debug("unknown command")
	r.sendMessage(fmt.Sprintf("I’m sorry, I don’t know how to `%s`.", command))
}

func (r *Room) gameInProgress() bool {
	// Should we also check to see that the underlying process is really
	// working?  (This could/should be exposed as a helper on Interpreter
	// itself.)
	return r.interp != nil
}
