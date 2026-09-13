package provider

type Repository interface {
	Save(Config) error
	Get(string) (Config, bool)
	List(Capability) []Config
	Delete(string) error
}
