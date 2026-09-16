package provider

import "context"

type Repository interface {
	Save(context.Context, Config) error
	Get(context.Context, string) (Config, error)
	List(context.Context, Capability) ([]Config, error)
	Delete(context.Context, string) error
}
