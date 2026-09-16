package material

import "context"

type Repository interface {
	Save(context.Context, Asset) error
	Get(context.Context, string) (Asset, error)
	List(context.Context, Kind) ([]Asset, error)
	Delete(context.Context, string) error
}
