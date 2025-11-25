package sudp

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	t.Run("Shouldn't accept tcp", func(t *testing.T) {
		assert := assert.New(t)

		_, err := Dial("tcp", "127.0.0.1:8090")

		assert.Error(err)
	})
}

func TestDialConn_SetDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestDialConn_SetReadDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetReadDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestDialConn_SetWriteDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetWriteDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}
