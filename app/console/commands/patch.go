package commands

import (
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type Patch struct{}

func NewPatch() *Patch {
	return &Patch{}
}

// Signature The name and signature of the console command.
func (r *Patch) Signature() string {
	return "patch"
}

// Description The console command description.
func (r *Patch) Description() string {
	return "Release patch version"
}

// Extend The console command extend.
func (r *Patch) Extend() command.Extend {
	return command.Extend{
		Category: "release",
		Flags: []command.Flag{
			&command.StringFlag{
				Name:    "framework",
				Aliases: []string{"f"},
				Usage:   "Patch framework tag, e.g. v1.16.2",
			},
			&command.StringFlag{
				Name:    "packages",
				Aliases: []string{"p"},
				Usage:   "Related packages branch, e.g. v1.4.x",
			},
		},
	}
}

// Handle Execute the console command.
func (r *Patch) Handle(ctx console.Context) error {
	release := NewRelease(ctx)

	return release.Patch()
}
