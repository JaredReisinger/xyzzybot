package games

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

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
	games, err := fs.getGames()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)

	for name := range games {
		names = append(names, name)
	}

	return names, nil
}

func (fs *FileSys) getGames() (map[string]os.FileInfo, error) {
	dir, err := os.Open(fs.Directory)
	if err != nil {
		return nil, err
	}

	infos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	games := make(map[string]os.FileInfo)

	for _, info := range infos {
		if info.Mode().IsRegular() {
			// Remove extension for game list
			name := info.Name()
			ext := path.Ext(name)
			games[strings.TrimSuffix(name, ext)] = info
		}
	}

	return games, nil
}

// GetGameFile returns the path to the game (in a form that can be passed to
// things like game interpreters).
func (fs *FileSys) GetGameFile(name string) (string, error) {
	games, err := fs.getGames()
	if err != nil {
		return "", err
	}

	info, ok := games[name]
	if !ok {
		return "", fmt.Errorf("Game “%s” not found", name)
	}

	return path.Join(fs.Directory, info.Name()), nil
}

// AddGameFile adds a new game to the repository
func (fs *FileSys) AddGameFile(fileName string, r io.Reader) error {
	// FUTURE: Ensure there are no relative file parts ("..", "/") in the
	// name...
	gameFile := path.Join(fs.Directory, fileName)
	logger2 := fs.Logger.WithFields(log.Fields{
		"game": fileName,
		"file": gameFile,
	})

	logger2.Info("adding game")
	f, err := os.Create(gameFile)
	if err != nil {
		logger2.WithError(err).Error("creating game file")
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, r)
	if err != nil {
		logger2.WithError(err).Error("writing game file")
		return err
	}

	logger2.WithField("written", written).Info("game file written")

	return nil
}

// DeleteGameFile removes a game from the repository
func (fs *FileSys) DeleteGameFile(fileName string) error {
	// FUTURE: Ensure there are no relative file parts ("..", "/") in the
	// name...
	gameFile := path.Join(fs.Directory, fileName)
	logger := fs.Logger.WithFields(log.Fields{
		"game": fileName,
		"file": gameFile,
	})

	logger.Info("deleting game")
	err := os.Remove(gameFile)
	if err != nil {
		logger.WithError(err).Error("deleting game file")
		return err
	}

	logger.Info("game file deleted")

	return nil
}
