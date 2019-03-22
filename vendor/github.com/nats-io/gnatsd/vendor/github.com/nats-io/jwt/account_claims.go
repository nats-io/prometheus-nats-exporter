package jwt

import (
	"errors"

	"github.com/nats-io/nkeys"
)

// Signifies no limit.
const NoLimit = -1

// OperatorLimits are used to limit access by an account
type OperatorLimits struct {
	Subs            int64 `json:"subs,omitempty"`      // Max number of subscriptions
	Conn            int64 `json:"conn,omitempty"`      // Max number of active connections
	Imports         int64 `json:"imports,omitempty"`   // Max number of imports
	Exports         int64 `json:"exports,omitempty"`   // Max number of exports
	Data            int64 `json:"data,omitempty"`      // Max number of bytes
	Payload         int64 `json:"payload,omitempty"`   // Max message payload
	WildcardExports bool  `json:"wildcards,omitempty"` // Are wildcards allowed in exports
}

// IsEmpty returns true if all of the limits are 0/false.
func (o *OperatorLimits) IsEmpty() bool {
	return *o == OperatorLimits{}
}

// IsUnlimited returns true if all limits are
func (o *OperatorLimits) IsUnlimited() bool {
	return *o == OperatorLimits{NoLimit, NoLimit, NoLimit, NoLimit, NoLimit, NoLimit, true}
}

// Validate checks that the operator limits contain valid values
func (o *OperatorLimits) Validate(vr *ValidationResults) {
	// negative values mean unlimited, so all numbers are valid
}

// Account holds account specific claims data
type Account struct {
	Imports    Imports        `json:"imports,omitempty"`
	Exports    Exports        `json:"exports,omitempty"`
	Identities []Identity     `json:"identity,omitempty"`
	Limits     OperatorLimits `json:"limits,omitempty"`
}

// Validate checks if the account is valid, based on the wrapper
func (a *Account) Validate(acct *AccountClaims, vr *ValidationResults) {
	a.Imports.Validate(acct.Subject, vr)
	a.Exports.Validate(vr)
	a.Limits.Validate(vr)

	for _, i := range a.Identities {
		i.Validate(vr)
	}

	if !a.Limits.IsEmpty() && a.Limits.Imports >= 0 && int64(len(a.Imports)) > a.Limits.Imports {
		vr.AddError("the account contains more imports than allowed by the operator")
	}

	// Check Imports and Exports for limit violations.
	if a.Limits.Imports != NoLimit {
		if int64(len(a.Imports)) > a.Limits.Imports {
			vr.AddError("the account contains more imports than allowed by the operator")
		}
	}
	if a.Limits.Exports != NoLimit {
		if int64(len(a.Exports)) > a.Limits.Exports {
			vr.AddError("the account contains more exports than allowed by the operator")
		}
		// Check for wildcard restrictions
		if !a.Limits.WildcardExports {
			for _, ex := range a.Exports {
				if ex.Subject.HasWildCards() {
					vr.AddError("the account contains wildcard exports that are not allowed by the operator")
				}
			}
		}
	}
}

// AccountClaims defines the body of an account JWT
type AccountClaims struct {
	ClaimsData
	Account `json:"nats,omitempty"`
}

// NewAccountClaims creates a new account JWT
func NewAccountClaims(subject string) *AccountClaims {
	if subject == "" {
		return nil
	}
	c := &AccountClaims{}
	// Set to unlimited to start. We do it this way so we get compiler
	// errors if we add to the OperatorLimits.
	c.Limits = OperatorLimits{NoLimit, NoLimit, NoLimit, NoLimit, NoLimit, NoLimit, true}
	c.Subject = subject
	return c
}

// Encode converts account claims into a JWT string
func (a *AccountClaims) Encode(pair nkeys.KeyPair) (string, error) {
	if !nkeys.IsValidPublicAccountKey(a.Subject) {
		return "", errors.New("expected subject to be account public key")
	}

	a.ClaimsData.Type = AccountClaim
	return a.ClaimsData.encode(pair, a)
}

// DecodeAccountClaims decodes account claims from a JWT string
func DecodeAccountClaims(token string) (*AccountClaims, error) {
	v := AccountClaims{}
	if err := Decode(token, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (a *AccountClaims) String() string {
	return a.ClaimsData.String(a)
}

// Payload pulls the accounts specific payload out of the claims
func (a *AccountClaims) Payload() interface{} {
	return &a.Account
}

// Validate checks the accounts contents
func (a *AccountClaims) Validate(vr *ValidationResults) {
	a.ClaimsData.Validate(vr)
	a.Account.Validate(a, vr)

	if nkeys.IsValidPublicAccountKey(a.ClaimsData.Issuer) {
		if len(a.Identities) > 0 {
			vr.AddWarning("self-signed account JWTs shouldn't contain identity proofs")
		}
		if !a.Limits.IsEmpty() {
			vr.AddWarning("self-signed account JWTs shouldn't contain operator limits")
		}
	}
}

// ExpectedPrefixes defines the types that can encode an account jwt, account and operator
func (a *AccountClaims) ExpectedPrefixes() []nkeys.PrefixByte {
	return []nkeys.PrefixByte{nkeys.PrefixByteAccount, nkeys.PrefixByteOperator}
}

// Claims returns the accounts claims data
func (a *AccountClaims) Claims() *ClaimsData {
	return &a.ClaimsData
}
