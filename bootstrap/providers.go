package bootstrap

import (
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/http"
	"github.com/goravel/framework/process"
)

func Providers() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		&http.ServiceProvider{},
		&process.ServiceProvider{},
	}
}
