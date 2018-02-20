package games

import (
	"io"
)

// Repository abstracts the available games (TODO: move this to a new
// package?)  Future work: allow adding/downloading of new game files.
type Repository interface {
	// GetGames returns the list of available games.  This may (in the future)
	// include metadata about the games, a game image/icon, etc.
	GetGames() ([]string, error)

	// GetGameFile returns the path to the game (in a form that can be passed to
	// things like game interpreters).
	GetGameFile(game string) (string, error)

	// AddGameFile adds a new game to the repository
	AddGameFile(fileName string, r io.Reader) error

	// DeleteGameFile removes a game from the repository
	DeleteGameFile(fileName string) error
}
