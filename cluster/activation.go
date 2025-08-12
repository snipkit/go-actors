package cluster

import (
	"math"
	"math/rand"
	"strconv"
)

const (
	DefaultRegion = "default"
)

// SelectMemberFunc is invoked during the activation process.
// Given ActivationDetails, it returns the member on which the actor will be spawned.
type SelectMemberFunc func(ActivationDetails) *Member

// ActivationConfig contains configuration for actor activation in the cluster.
type ActivationConfig struct {
	id           string
	region       string
	selectMember SelectMemberFunc
}

// NewActivationConfig returns a new ActivationConfig with default values.
func NewActivationConfig() *ActivationConfig {
	return &ActivationConfig{
		id:           strconv.Itoa(rand.Intn(math.MaxInt)),
		region:       DefaultRegion,
		selectMember: SelectRandomMember,
	}
}

// WithSelectMemberFunc sets the function to select the member during activation.
// Returns the updated ActivationConfig for chaining.
func (c *ActivationConfig) WithSelectMemberFunc(fn SelectMemberFunc) *ActivationConfig {
	c.selectMember = fn
	return c
}

// WithID sets the actor ID to be activated in the cluster.
// Defaults to a random identifier if not set.
func (c *ActivationConfig) WithID(id string) *ActivationConfig {
	c.id = id
	return c
}

// WithRegion sets the region where the actor should be spawned.
// Defaults to "default" if not set.
func (c *ActivationConfig) WithRegion(region string) *ActivationConfig {
	c.region = region
	return c
}

// ActivationDetails provides information needed to select a cluster member for activation.
type ActivationDetails struct {
	Region  string    // Region for actor activation
	Members []*Member // Pre-filtered cluster members by actor kind
	Kind    string    // Kind of the actor
}

// SelectRandomMember randomly selects a member from the available cluster members.
// Returns nil if the Members slice is empty.
func SelectRandomMember(details ActivationDetails) *Member {
	if len(details.Members) == 0 {
		return nil
	}
	return details.Members[rand.Intn(len(details.Members))]
}
