package commands

import (
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type Preview struct{}

func NewPreview() *Preview {
	return &Preview{}
}

// Signature The name and signature of the console command.
func (r *Preview) Signature() string {
	return "preview"
}

// Description The console command description.
func (r *Preview) Description() string {
	return "Release preview, will list all repositories' changes."
}

// Extend The console command extend.
func (r *Preview) Extend() command.Extend {
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
				Name:    "packages",
				Usage:   "Whether to preview all packages' changes.",
				Value:   false,
				Aliases: []string{"p"},
			},
		},
	}
}

// Handle Execute the console command.
func (r *Preview) Handle(ctx console.Context) error {
	release := NewRelease(ctx)

	return release.Preview()
}
