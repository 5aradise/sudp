package sudp

import (
	"errors"
)

var (
	ErrPacketCorrupted = errors.New("packet corrupted while writing")
)
