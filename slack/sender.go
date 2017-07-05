package slack

import log "github.com/sirupsen/logrus"

type Sender struct {
	channel string
	logger  log.FieldLogger
}

func NewSender(channel string, logger log.FieldLogger) *Sender {
	return &Sender{
		channel: channel,
		logger: logger.WithFields(log.Fields{
			"component": "slackSender",
			"channel":   channel,
		}),
	}
}

// func (s *Sender) Listen(outchan chan *interpreter.GlkOutput) {
// 	s.logger.Info("setting up sender")
// 	for {
// 		output := <-outchan
// 		debugOutput := s.debugFormat(output)
// 		s.logger.WithField("output", debugOutput).Debug("recieved output")
// 	}
// }
//
// func (s *Sender) debugFormat(output *interpreter.GlkOutput) string {
// 	sep1 := "============================================================"
// 	sep2 := "------------------------------------------------------------"
// 	lines := []string{sep1}
//
// 	for _, w := range output.Windows {
// 		lines = append(lines, FormatWindow(w))
// 		lines = append(lines, sep2)
// 	}
//
// 	lines = append(lines, sep1)
//
// 	return strings.Join(lines, "\n")
// }
