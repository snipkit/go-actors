package shared

import (
	"fmt"

	"github.com/khulnasoft/goactors/actor"
	"github.com/khulnasoft/goactors/remote"
)

type Player struct{}

func NewPlayer() actor.Receiver {
	return &Player{}
}

func (p *Player) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
	case *remote.TestMessage:
		fmt.Println(string(msg.Data))
	}
}
