package provider

type CredentialVault interface {
	Put(string, string) error
	Has(string) bool
	Delete(string) error
}
