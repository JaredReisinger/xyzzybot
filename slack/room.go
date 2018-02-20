package slack

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/fizmo"
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
	ID          string
	roomType    roomType
	name        string
	link        string // formatted `<#C1234|foo>` or `<@U1234|bob>` link
	config      *Config
	manager     *Manager
	interpreter fizmo.Interpreter
	logger      log.FieldLogger
}

func newRoom(config *Config, manager *Manager, id string, roomType roomType, name string, link string) *Room {
	return &Room{
		ID:       id,
		roomType: roomType,
		name:     name,
		link:     link,
		config:   config,
		manager:  manager,
		logger: config.Logger.WithFields(log.Fields{
			"component": "slack",
			"room":      name,
			"roomID":    id,
		}),
	}
}

func (r *Room) startGame(name string) (err error) {
	if r.gameInProgress() {
		err = errors.New("game already in progress, ignoring start-game request")
		r.logger.WithError(err).Error("starting game")
		return err
	}

	// Create a working directory for the interpreter...
	workingDir := path.Join(r.config.WorkingRoot, r.ID)
	err = os.MkdirAll(workingDir, os.FileMode(0755))
	if err != nil {
		r.logger.WithError(err).Error("creating working directory")
		return err
	}

	// Create a new interpreter for the requested game...
	r.logger.WithField("game", name).Info("starting game")
	gameFile, err := r.config.Games.GetGameFile(name)
	if err != nil {
		r.logger.WithError(err).Error("getting game file")
		return err
	}
	i, err := r.config.InterpreterFactory.NewInterpreter(gameFile, workingDir, log.Fields{
		"game": name,
		"room": r.ID,
	})
	if err != nil {
		r.logger.WithError(err).Error("starting interpreter")
		return err
	}

	go r.listenForGameOutput(i.GetOutputChannel())

	err = i.Start()
	if err != nil {
		r.logger.WithError(err).Error("starting interpreter")
		return err
	}

	r.interpreter = i
	return nil
}

func (r *Room) listenForGameOutput(outchan chan *fizmo.Output) {
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

func (r *Room) sendOutputMessage(output *fizmo.Output) {
	statusParts := []string{}

	for _, col := range output.Status.Columns {
		columnParts := []string{}
		for _, line := range col.Lines {
			columnParts = append(columnParts, formatSpans(line.Text))
		}
		statusParts = append(statusParts, strings.Join(columnParts, " | "))
	}

	status := strings.Join(statusParts, " — ")

	lines := []string{}

	for _, line := range output.Story {
		lines = append(lines, formatSpans(line))
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

// func inferStatusWindow(w *fizmo.Window) bool {
// 	return w.Type == fizmo.TextGridWindow &&
// 		w.Top == 0 &&
// 		w.Height <= 5
// }

func (r *Room) debugFormat(output *fizmo.Output) string {
	return formatDebugOutput(output)
}

func (r *Room) killGame() {
	r.logger.Info("recieved killGame request")

	if r.interpreter != nil {
		// TODO: stop listening to random stuff?
		r.interpreter.Kill()
		r.interpreter = nil
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
		msg := fmt.Sprintf(format, r.manager.authInfo.User)
		r.sendMessage(msg)
	}
}

func (r *Room) sendMessage(text string) {
	r.manager.sendMessage(r.ID, text)
}

// func (r *Room) sendMessageWithStatus(text string, status string) {
// 	r.manager.sendMessageWithStatus(r.ID, text, status)
// }

func (r *Room) sendMessageWithNameContext(text string, status string, nameContext string) {
	r.manager.sendMessageWithNameContext(r.ID, text, status, nameContext)
}

type commandContext struct {
	msgEvent *slack.MessageEvent
}

type commandHandler func(cmdContext *commandContext, command string, args ...string)

type commandDescription struct {
	name       string
	handler    commandHandler
	adminOnly  bool
	inGameOnly bool
	short      string
	long       string
}

// We use a list (rather than a map) for the commands... (a) there aren't that
// many of them, so a linear search is fast enough, (b) we want to be able to
// create the command list in a specific order, and (c) we might want commands
// to use more than just first word in the future, and letting commands "opt in"
// only works if you step through all of them.
//
// Should we separate the names/descriptions and the handlers?  The former are
// all constants, but the handlers are tied to a particular room object, and
// thus *not* constant at all...  I still think I prefer keeping everything
// together, though.

func (r *Room) getCommandDescriptions() []*commandDescription {
	return []*commandDescription{
		&commandDescription{
			"help",
			r.commandHelp,
			false,
			false,
			"this message",
			"[long help for help]",
		},
		&commandDescription{
			"status",
			r.commandStatus,
			false,
			false,
			"operational status about myself",
			"[long help for status]",
		},
		&commandDescription{
			"list",
			r.commandList,
			false,
			false,
			"list the available games",
			"[long help for list]",
		},
		&commandDescription{
			"play",
			r.commandPlay,
			false,
			false,
			"with a game name (*play _game-name_\u200d*), starts _game-name_",
			"[long help for play]",
		},
		&commandDescription{
			"kill",
			r.commandKill,
			false,
			true,
			"kill the current in-progress game",
			"[long help for kill]",
		},
		// &commandDescription{
		// 	"space",
		// 	r.commandSpace,
		// 	false,
		// 	true,
		// 	"send a space character to the game (needed for some prompts)",
		// 	"[long help for space]",
		// },
		// &commandDescription{
		// 	"key",
		// 	r.commandKey,
		// 	false,
		// 	true,
		// 	"with a character (*%[1]skey x*), sends a raw key to the game",
		// 	"[long help for key]",
		// },
		&commandDescription{
			"upload",
			r.commandUpload,
			true,
			false,
			"adds a new game to the system from a url",
			"If you tell me to *upload _url-to-game_*, I’ll retrieve the game and add it to the list.  Note that this will only work if you’re a xyzzybot admin.  If you’re looking for games, try <http://ifdb.tads.org/|the Interactive Fiction Database>.  You can also add a new game to the system by uploading a file with a `@xyzzybot upload` comment.",
		},
		&commandDescription{
			"delete",
			r.commandDelete,
			true,
			false,
			"remove a game from the system",
			"If you tell me to *delete _game-name_*, I’ll delete the game from the list.  Note that this will only work if you’re a xyzzybot admin.",
		},
	}
}

func (r *Room) handleCommand(msgEvent *slack.MessageEvent, command string) {
	// If we have an interpreter, it gets the command.  Otherwise (or if there's
	// a leading metaCommandPrefix), it's a meta-command.
	if r.gameInProgress() && !strings.HasPrefix(command, metaCommandPrefix) {
		r.interpreter.Send(command)
		return
	}

	command = strings.TrimPrefix(command, metaCommandPrefix)

	// right now, we only do super-simple command parsing...
	words := strings.Fields(command)

	cmdContext := &commandContext{msgEvent}

	commandDescs := r.getCommandDescriptions()
	var desc *commandDescription
	for _, d := range commandDescs {
		if d.name == words[0] {
			desc = d
			break
		}
	}

	if desc.adminOnly && !r.fromAdmin(cmdContext) {
		// TODO: point out that the user isn't an admin...
		desc = nil
	}

	handler := r.commandUnknown
	if desc != nil {
		handler = desc.handler
	}

	handler(&commandContext{msgEvent}, words[0], words[1:]...)
}

func (r *Room) commandHelp(cmdContext *commandContext, command string, args ...string) {
	if len(args) > 0 {
		switch args[0] {
		case "command", "commands":
			r.helpCommands()
			return
		default:
			commandDescs := r.getCommandDescriptions()
			for _, d := range commandDescs {
				// TODO: only show admin commands to an admin?
				if d.name == args[0] {
					r.sendMessage(d.long)
					return
				}
			}

			prefix := ""
			if r.gameInProgress() {
				prefix = metaCommandPrefix
			}
			r.sendMessage(fmt.Sprintf("I’m afraid I don’t know how to help with *%s*.  Perhaps try a plain *%shelp* to start with?", args[0], prefix))
			return
		}
	}

	// Default, short help
	r.helpBrief()
}

func (r *Room) helpBrief() {
	gameInProgress := r.gameInProgress()
	contextualPrefix := ""
	if gameInProgress {
		contextualPrefix = metaCommandPrefix
	}

	commandDescs := r.getCommandDescriptions()

	formattedCommands := make([]string, 0, len(commandDescs))

	for _, d := range commandDescs {
		if !gameInProgress && d.inGameOnly {
			continue
		}
		formattedCommands = append(formattedCommands, fmt.Sprintf("*%s%s*", contextualPrefix, d.name))
	}

	lines := make([]string, 0, 3)

	lines = append(lines, fmt.Sprintf("I’m %s, and I’m here to help you experience the world of interactive fiction.\n\nRight now, I can: %s.", r.manager.authInfo.User, strings.Join(formattedCommands, ", ")))

	lines = append(lines, fmt.Sprintf("\nYou can also try *%shelp commands* to get further details.", contextualPrefix))

	msg := strings.Join(lines, "\n")
	r.sendMessage(msg)
}

func (r *Room) helpCommands() {
	commandDescs := r.getCommandDescriptions()
	lines := make([]string, 0, len(commandDescs)+3)

	lines = append(lines, "I’m pretty good about understanding whether you’re chatting with other people or writing commands for an in-progress game, so for the most part things should _just work_.  (If I don’t seem to be paying attention, address me directly—with *@%[2]s _command_*—and see if that helps.)\n\nWhen there’s not a game running, there are several things I can do for you:")

	for _, d := range commandDescs {
		if d.inGameOnly {
			continue
		}
		adminOnly := ""
		if d.adminOnly {
			adminOnly = " _(admin only)_"
		}
		lines = append(lines, fmt.Sprintf("     *%s* — %s%s", d.name, d.short, adminOnly))
	}

	lines = append(lines, "\nWhen there _is_ a game under way, there are a few additional things I can help with:")

	for _, d := range commandDescs {
		if !d.inGameOnly {
			continue
		}
		lines = append(lines, fmt.Sprintf("     *%s%s* — %s", "%[1]s", d.name, d.short))
	}

	lines = append(lines, "You might have noticed that these commands are *%[1]s*-prefixed.  That’s how I can tell that a command is meant for me rather than the game.  You can also use any of the first set of commands during a game, but you’ll need to *%[1]s*-prefix them so that I know they’re meant for me.")

	format := strings.Join(lines, "\n")
	msg := fmt.Sprintf(format, metaCommandPrefix, r.manager.authInfo.User)
	r.sendMessage(msg)
}

func (r *Room) fromAdmin(cmdContext *commandContext) bool {
	_, admin := r.manager.getUser(cmdContext.msgEvent.User)
	return admin
}

func (r *Room) commandStatus(cmdContext *commandContext, command string, args ...string) {
	admin := r.fromAdmin(cmdContext)

	var inProgress string
	if r.gameInProgress() {
		inProgress = "There *is* currently a game in progress."
	} else {
		inProgress = "There *is not* currently a game in progress."
	}

	typeLinks := r.manager.getActiveRoomLinks()

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
		return fmt.Sprintf("no %ss", label)

	case 1:
		return fmt.Sprintf("%s %s", label, list[0])

	case 2:
		return fmt.Sprintf("%ss %s and %s", label, list[0], list[1])
	}

	most := list[0 : len(list)-2]
	last := list[len(list)-1]
	return fmt.Sprintf("%ss %s, and %s", label, strings.Join(most, ", "), last)
}

func (r *Room) commandList(cmdContext *commandContext, command string, args ...string) {
	games, err := r.config.Games.GetGames()
	if err != nil {
		r.logger.WithError(err).Error("unable to get games")
		r.sendMessage(fmt.Sprintf("I’m sorry, I wasn’t able to get to the list of games.  Please let %s know something needs to be tweaked!", r.config.Admins[0]))
		return
	}

	warning := ""

	if r.gameInProgress() {
		warning = fmt.Sprintf("\n\n_Do note that there's currently a game in progress; you’ll need to finish or `%skill` it before you can start a new game._", metaCommandPrefix)
	}

	msg := fmt.Sprintf("The following games are currently available:\n     *%s*\n\nYou can start a game using *play _game-name_*%s", strings.Join(games, "*\n     *"), warning)
	r.sendMessage(msg)
}

func (r *Room) commandPlay(cmdContext *commandContext, command string, args ...string) {
	if r.gameInProgress() {
		r.sendMessage(fmt.Sprintf("_There's currently a game in progress; you’ll need to finish or `%skill` it before you can start a new game._", metaCommandPrefix))
		return
	}

	if len(args) == 0 {
		r.sendMessage("I’m looking forward to starting a new game, but I don’t know what game you want to play.  You can use *list* to list the available games, and you would have found out...")
		r.commandList(cmdContext, "list")
		return
	}

	err := r.startGame(args[0])
	if err != nil {
		// r.killGame()
		r.sendMessage(fmt.Sprintf("There was a problem starting the game: “%s”", err.Error()))
	}
}

func (r *Room) commandKill(cmdContext *commandContext, command string, args ...string) {
	if !r.gameInProgress() {
		r.sendMessage("There's _not_ currently a game in progress!")
		return
	}

	r.killGame()
}

// func (r *Room) commandSpace(cmdContext *commandContext, command string, args ...string) {
// 	if !r.gameInProgress() {
// 		r.sendMessage("There's _not_ currently a game in progress!")
// 		return
// 	}
//
// 	// r.interpreter.SendKey(" ")
// 	r.interpreter.Send(" ")
// }
//
// func (r *Room) commandKey(cmdContext *commandContext, command string, args ...string) {
// 	if !r.gameInProgress() {
// 		r.sendMessage("There's _not_ currently a game in progress!")
// 		return
// 	}
//
// 	key := " "
// 	if len(args) > 0 {
// 		key = args[0]
// 	}
//
// 	// r.interpreter.SendKey(key)
// 	r.interpreter.SendChar(rune(key[0]))
// }

var gameLink = regexp.MustCompile("<(.+)(|.+)?>")

func (r *Room) commandUpload(cmdContext *commandContext, command string, args ...string) {
	if len(args) != 1 {
		r.sendMessage("I expect one—and _only_ one—URL from which to retrieve a game file: *upload _url-to-game_*")
		return
	}

	// Slack wraps "<>" around URLs, so we need to strip them...
	uri := args[0]
	if strings.HasPrefix(uri, "<") {
		matches := gameLink.FindStringSubmatch(args[0])
		if matches == nil {
			r.logger.WithField("url", uri).Error("could not extract url")
			r.sendMessage(fmt.Sprintf("I’m sorry, I couldn’t make sense of `%s` as a url.", uri))
			return
		}
		uri = matches[1]
	}

	logger := r.logger.WithField("url", uri)

	urlParts, err := url.Parse(uri)
	if err != nil {
		logger.WithError(err).Error("could not parse url")
		r.sendMessage(fmt.Sprintf("I wasn’t able to understand %s... %s", uri, err.Error()))
		return
	}

	filename := path.Base(urlParts.Path)
	err = r.manager.downloadGame(uri, filename)
	if err != nil {
		logger.WithError(err).Error("downloading")
		r.sendMessage(err.Error())
		return
	}

	r.sendMessage(fmt.Sprintf("Upload complete!  The game *%s* is now available!", filename))

}

func (r *Room) commandDelete(cmdContext *commandContext, command string, args ...string) {
	if len(args) != 1 {
		r.sendMessage("I expect one—and _only_ one—game to remove from the system: *delete _game-name_*")
		return
	}

	// Slack wraps "<>" around URLs, so we need to strip them...
	filename := args[0]
	logger := r.logger.WithField("game", filename)

	err := r.manager.deleteGame(filename)
	if err != nil {
		logger.WithError(err).Error("deleting")
		r.sendMessage(err.Error())
		return
	}

	r.sendMessage(fmt.Sprintf("Delete complete!  The game *%s* is no longer available!", filename))

}

func (r *Room) commandUnknown(cmdContext *commandContext, command string, args ...string) {
	r.logger.WithField("command", command).Debug("unknown command")
	r.sendMessage(fmt.Sprintf("I’m sorry, I don’t know how to `%s`.", command))
}

func (r *Room) gameInProgress() bool {
	// Should we also check to see that the underlying process is really
	// working?  (This could/should be exposed as a helper on Interpreter
	// itself.)
	return r.interpreter != nil
}
