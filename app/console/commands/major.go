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
		Flags: []command.Flag{
			&command.StringFlag{
				Name:    "framework",
				Aliases: []string{"f"},
				Usage:   "Release framework tag",
			},
			&command.StringFlag{
				Name:    "packages",
				Aliases: []string{"p"},
				Usage:   "Release packages tag",
			},
			&command.BoolFlag{
				Name:    "real",
				Aliases: []string{"r"},
				Usage:   "Real release",
			},
		},
	}
}

// Handle Execute the console command.
func (r *Major) Handle(ctx console.Context) error {
	release := NewRelease(ctx)

	return release.Major()
}
