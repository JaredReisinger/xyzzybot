package glk // import "github.com/JaredReisinger/xyzzybot/glk"

import (
	log "github.com/sirupsen/logrus"
)

// InterpreterFactory ..
type InterpreterFactory interface {
	NewInterpreter(gameFile string, fields log.Fields) (Interpreter, error)
}

// Interpreter represents the interface to a Glk-based game interpreter.  Output
// is asynchronous, and can occur any time after Start() has been called.
type Interpreter interface {
	GetOutputChannel() chan *Output
	Start() error
	SendLine(command string) error
	SendChar(char rune) error
	SendInput(input *Input) error
	Kill()
}
