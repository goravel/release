package commands

import (
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type Major struct{}

func NewMajor() *Major {
	return &Major{}
}

// Signature The name and signature of the console command.
func (r *Major) Signature() string {
	return "major"
}

// Description The console command description.
func (r *Major) Description() string {
	return "Release major version"
}

// Extend The console command extend.
func (r *Major) Extend() command.Extend {
	return command.Extend{
		Category: "release",
		Arguments: []command.Argument{
			&command.ArgumentString{
				Name:     "tag",
				Required: true,
			},
		},
		Flags: []command.Flag{
			&command.BoolFlag{
				Name:    "real",
				Aliases: []string{"r"},
				Usage:   "Real release",
			},
			&command.BoolFlag{
				Name:    "refresh",
				Aliases: []string{},
				Usage:   "Refresh Go module proxy cache before release",
			},
			&command.StringFlag{
				Name:    "framework-branch",
				Aliases: []string{"fb"},
				Usage:   "Optional, release framework branch, sometimes go mod cannot fetch the latest master",
			},
		},
	}
}

// Handle Execute the console command.
func (r *Major) Handle(ctx console.Context) error {
	release := NewRelease(ctx)

	return release.Major()
}
