package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"tbmux/internal/app"
	"tbmux/internal/model"
)

type Options struct {
	InitialFilter model.Filter
}

func Run(configPath string, opts Options) error {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return err
	}
	if err := app.EnsureDirs(cfg); err != nil {
		return err
	}
	st, err := app.LoadState(cfg)
	if err != nil {
		return err
	}
	m := New(configPath, cfg, st, opts.InitialFilter)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
