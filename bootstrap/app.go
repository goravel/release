package bootstrap

import (
	"github.com/goravel/framework/contracts/console"
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	"goravel/app/console/commands"
	"goravel/config"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithProviders(Providers).
		WithConfig(config.Boot).
		WithCommands(func() []console.Command {
			return []console.Command{
				commands.NewRelease(),
			}
		}).
		Start()
}
