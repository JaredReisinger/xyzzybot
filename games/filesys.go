package games

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// FileSys ...
type FileSys struct {
	Directory string
	Logger    log.FieldLogger
	// InterpreterFactory interpreter.InterpreterFactory
}

// GetGames returns the list of available games.  This may (in the future)
// include metadata about the games, a game image/icon, etc.
func (fs *FileSys) GetGames() ([]string, error) {
	dir, err := os.Open(fs.Directory)
	if err != nil {
		return nil, err
	}

	infos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	games := make([]string, 0, len(infos))

	for _, info := range infos {
		if info.Mode().IsRegular() {
			games = append(games, info.Name())
		}
	}

	return games, nil
}

// GetGameFile returns the path to the game (in a form that can be passed to
// things like game interpreters).
func (fs *FileSys) GetGameFile(game string) (string, error) {
	gameFile := path.Join(fs.Directory, game)
	fs.Logger.WithFields(log.Fields{
		"game": game,
		"file": gameFile,
	}).Info("starting game")
	return gameFile, nil
}

// // CreateInterpreter starts an interpreter for the given game.
// func (fs *FileSys) CreateInterpreter(game string) (interpreter.Interpreter, error) {
// 	gameFile := path.Join(fs.Directory, game)
// 	fs.Logger.WithFields(log.Fields{
// 		"game": game,
// 		"file": gameFile,
// 	}).Info("starting game")
// 	return fs.InterpreterFactory.NewInterpreter(gameFile, log.Fields{"game": game})
// }
