package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	"golang.org/x/exp/maps"

	"github.com/aws/aws-application-networking-k8s/pkg/aws/services"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

const (
	TagBase      = "application-networking.k8s.aws/"
	TagManagedBy = TagBase + "ManagedBy"
)

//go:generate mockgen -destination cloud_mocks.go -package aws github.com/aws/aws-application-networking-k8s/pkg/aws Cloud

type CloudConfig struct {
	VpcId       string
	AccountId   string
	Region      string
	ClusterName string
}

type Cloud interface {
	Config() CloudConfig
	Lattice() services.Lattice

	// creates lattice tags with default values populated
	DefaultTags() services.Tags

	// creates lattice tags with default values populated and merges them with provided tags
	DefaultTagsMergedWith(services.Tags) services.Tags

	// check if tags map has managedBy tag
	ContainsManagedBy(tags services.Tags) bool

	// check if managedBy tag set for lattice resource
	IsArnManaged(arn string) (bool, error)
}

// NewCloud constructs new Cloud implementation.
func NewCloud(log gwlog.Logger, cfg CloudConfig) (Cloud, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	sess.Handlers.Complete.PushFront(func(r *request.Request) {
		if r.Error != nil {
			log.Debugw("error",
				"error", r.Error.Error(),
				"serviceName", r.ClientInfo.ServiceName,
				"operation", r.Operation.Name,
				"params", r.Params,
			)
		} else {
			log.Debugw("response",
				"serviceName", r.ClientInfo.ServiceName,
				"operation", r.Operation.Name,
				"params", r.Params,
			)
		}
	})

	lattice := services.NewDefaultLattice(sess, cfg.Region)
	cl := NewDefaultCloud(lattice, cfg)
	return cl, nil
}

// Used in testing and mocks
func NewDefaultCloud(lattice services.Lattice, cfg CloudConfig) Cloud {
	return &defaultCloud{
		cfg:          cfg,
		lattice:      lattice,
		managedByTag: getManagedByTag(cfg),
	}
}

type defaultCloud struct {
	cfg          CloudConfig
	lattice      services.Lattice
	managedByTag string
}

func (c *defaultCloud) Lattice() services.Lattice {
	return c.lattice
}

func (c *defaultCloud) Config() CloudConfig {
	return c.cfg
}

func (c *defaultCloud) DefaultTags() services.Tags {
	tags := services.Tags{}
	tags[TagManagedBy] = &c.managedByTag
	return tags
}

func (c *defaultCloud) DefaultTagsMergedWith(tags services.Tags) services.Tags {
	newTags := c.DefaultTags()
	maps.Copy(newTags, tags)
	return newTags
}

func (c *defaultCloud) ContainsManagedBy(tags services.Tags) bool {
	tag, ok := tags[TagManagedBy]
	if !ok || tag == nil {
		return false
	}
	return *tag == c.managedByTag
}

func (c *defaultCloud) IsArnManaged(arn string) (bool, error) {
	tagsReq := &vpclattice.ListTagsForResourceInput{ResourceArn: &arn}
	resp, err := c.lattice.ListTagsForResource(tagsReq)
	if err != nil {
		return false, nil
	}
	isManaged := c.ContainsManagedBy(resp.Tags)
	return isManaged, nil
}

func getManagedByTag(cfg CloudConfig) string {
	return fmt.Sprintf("%s/%s/%s", cfg.AccountId, cfg.ClusterName, cfg.VpcId)
}
