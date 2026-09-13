package material

type Repository interface {
	Save(Asset) error
	Get(string) (Asset, bool)
	List(Kind) []Asset
	Delete(string) error
}
