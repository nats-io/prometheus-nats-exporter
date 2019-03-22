package jwt

import (
	"errors"

	"github.com/nats-io/nkeys"
)

// Cluster stores the cluster specific elements of a cluster JWT
type Cluster struct {
	Trust       []string `json:"identity,omitempty"`
	Accounts    []string `json:"accts,omitempty"`
	AccountURL  string   `json:"accturl,omitempty"`
	OperatorURL string   `json:"opurl,omitempty"`
}

// Validate checks the cluster and permissions for a cluster JWT
func (c *Cluster) Validate(vr *ValidationResults) {
	// fixme validate cluster data
}

// ClusterClaims defines the data in a cluster JWT
type ClusterClaims struct {
	ClaimsData
	Cluster `json:"nats,omitempty"`
}

// NewClusterClaims creates a new cluster JWT with the specified subject/public key
func NewClusterClaims(subject string) *ClusterClaims {
	if subject == "" {
		return nil
	}
	c := &ClusterClaims{}
	c.Subject = subject
	return c
}

// Encode tries to turn the cluster claims into a JWT string
func (c *ClusterClaims) Encode(pair nkeys.KeyPair) (string, error) {
	if !nkeys.IsValidPublicClusterKey(c.Subject) {
		return "", errors.New("expected subject to be a cluster public key")
	}
	c.ClaimsData.Type = ClusterClaim
	return c.ClaimsData.encode(pair, c)
}

// DecodeClusterClaims tries to parse cluster claims from a JWT string
func DecodeClusterClaims(token string) (*ClusterClaims, error) {
	v := ClusterClaims{}
	if err := Decode(token, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *ClusterClaims) String() string {
	return c.ClaimsData.String(c)
}

// Payload returns the cluster specific data
func (c *ClusterClaims) Payload() interface{} {
	return &c.Cluster
}

// Validate checks the generic and cluster data in the cluster claims
func (c *ClusterClaims) Validate(vr *ValidationResults) {
	c.ClaimsData.Validate(vr)
	c.Cluster.Validate(vr)
}

// ExpectedPrefixes defines the types that can encode a cluster JWT, operator or cluster
func (c *ClusterClaims) ExpectedPrefixes() []nkeys.PrefixByte {
	return []nkeys.PrefixByte{nkeys.PrefixByteOperator, nkeys.PrefixByteCluster}
}

// Claims returns the generic data
func (c *ClusterClaims) Claims() *ClaimsData {
	return &c.ClaimsData
}
