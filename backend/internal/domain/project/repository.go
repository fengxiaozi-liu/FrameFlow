package project

type Repository interface {
	Save(Project) error
	Get(string) (Project, bool)
	List() []Project
	Delete(string) error
}
