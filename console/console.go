package console

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

type Console struct {
	gui *gocui.Gui
}

func (c *Console) Close() {
	c.gui.Close()
}

func New(cfg *Config) (*Console, error) {
	gui, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}

	maxX, maxY := gui.Size()
	if v, err := gui.SetView("hello", maxX/2-7, maxY/2, maxX/2+7, maxY/2+2); err != nil {
		if err != gocui.ErrUnknownView {
			return nil, err
		}
		fmt.Fprintln(v, "Hello world!")
	}

	return &Console{
		gui: gui,
	}, nil
}
