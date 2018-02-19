package fizmo // import "github.com/JaredReisinger/xyzzybot/fizmo"

import (
	log "github.com/sirupsen/logrus"
)

// InterpreterFactory ..
type InterpreterFactory interface {
	NewInterpreter(gameFile string, workingDir string, fields log.Fields) (Interpreter, error)
}

// Interpreter represents the interface to a game interpreter.  Output
// is asynchronous, and can occur any time after Start() has been called.
type Interpreter interface {
	GetOutputChannel() chan *Output
	Start() error
	Send(input string) error
	Kill()
}
