package project

import "context"

type Repository interface {
	Save(context.Context, Project) error
	Get(context.Context, string) (Project, error)
	List(context.Context) ([]Project, error)
	Delete(context.Context, string) error
}

type AtomicRepository interface {
	Repository
	Update(context.Context, string, func(*Project) error) (Project, error)
}

type DraftDeletionRepository interface {
	DeleteDraft(context.Context, string, string, int64) error
}
