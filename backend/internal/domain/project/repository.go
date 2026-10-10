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

// DeleteProjectChecked checks revisions and active tasks in the same transaction.
type ProjectDeletionRepository interface {
	DeleteProjectChecked(context.Context, string, DeletionExpectation) error
}
