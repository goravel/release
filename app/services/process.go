package services

import "github.com/goravel/framework/support/process"

type Process interface {
	Run(command string) (string, error)
}

type ProcessImpl struct {
}

func NewProcessImpl() *ProcessImpl {
	return &ProcessImpl{}
}

func (r *ProcessImpl) Run(command string) (string, error) {
	return process.Run(command)
}
