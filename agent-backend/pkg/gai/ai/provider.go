package ai

import "context"

type Provider interface {
	Name() string
	Model(name string) (Model, error)
	ListModels(ctx context.Context) ([]string, error)
}
