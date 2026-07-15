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
			&command.StringFlag{
				Name:    "packages",
				Aliases: []string{"p"},
				Usage:   "Extra packages to release, format: pkg1@version1,pkg2@version2",
			},
		},
	}
}

// Handle Execute the console command.
func (r *Patch) Handle(ctx console.Context) error {
	release := NewRelease(ctx)

	return release.Patch()
}
