package client

import (
	"goshop/services/identity/contract"
)

// Re-export Identity contract types and constructors for backward compatibility
type (
	IdentityClient             = contract.IdentityClient
	HTTPIdentityClient         = contract.HTTPIdentityClient
	GRPCIdentityClient         = contract.GRPCIdentityClient
	ClientOption               = contract.ClientOption
	GRPCOption                 = contract.GRPCOption
	InternalUserDTO            = contract.InternalUserDTO
	InternalBatchUsersResponse = contract.InternalBatchUsersResponse
	AddressDTO                 = contract.AddressDTO
)

var (
	NewHTTPIdentityClient = contract.NewHTTPIdentityClient
	NewGRPCIdentityClient = contract.NewGRPCIdentityClient
	WithHTTPClient        = contract.WithHTTPClient
	WithCacheTTL          = contract.WithCacheTTL
	WithGRPCCacheTTL      = contract.WithGRPCCacheTTL
)
