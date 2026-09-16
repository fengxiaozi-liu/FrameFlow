package provider

import "context"

type CredentialVault interface {
	Put(context.Context, string, string) error
	Has(context.Context, string) (bool, error)
	Delete(context.Context, string) error
}
