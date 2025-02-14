package cplatform

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	confluent "github.com/wayarmy/gonfluent"
)

var err error

//kafkaTopicRBAC will bind the roles to users/scope
// example:
// resource "kafka_topic_rbac" "user_confluent_test" {
//   principal  = "User:confluent-test"
//   role 		= "Operation"
//	 cluster_id = "zxvdlaskdjal"
// 	 cluster_type = "Kafka"
//
//   provider = confluent-kafka.confluent
//	}
// Resource ID = principal + "|" + clusterId + "|" + principal + "|" + role
func kafkaTopicRBAC() *schema.Resource {
	return &schema.Resource{
		CreateContext: kafkaTopicRBACCreate,
		DeleteContext: kafkaTopicRBACDelete,
		ReadContext:   kafkaTopicRBACRead,

		Schema: map[string]*schema.Schema{
			"principal": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "Defined the principal - User or subject",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					if !strings.Contains(v, "User:") || !strings.Contains(v, "Group:") && strings.Contains(v, "|") {
						errs = append(errs, fmt.Errorf("%q must be defined with User: or Group: and must not have |, got: %s", key, v))
					}
					return
				},
			},
			"role": {
				Type:         schema.TypeString,
				ForceNew:     true,
				Required:     true,
				Description:  "Role of",
				DefaultFunc:  schema.EnvDefaultFunc("ROLE", "DeveloperRead"),
				ValidateFunc: validation.StringInSlice(scopeRole, false),
			},
			"resource_type": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("RESOURCE_TYPE", ""),
			},
			"pattern_type": {
				Type:         schema.TypeString,
				ForceNew:     true,
				Required:     true,
				ValidateFunc: validation.StringInSlice(validPatternType, false),
			},
			"name": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"cluster_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Computed:    true,
				Description: "The ID of cluster",
			},
		},
	}
}

func kafkaTopicRBACRead(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*confluent.Client)
	principal := d.Get("principal").(string)
	role := d.Get("role").(string)
	r := parseIdToResourcesList(d.Id())
	var clusterId string
	if d.Get("cluster_id") == nil {
		clusterId = r[0]
	} else {
		clusterId = d.Get("cluster_id").(string)
	}

	cDetails := &confluent.ClusterDetails{}
	cDetails.Clusters.KafkaCluster = clusterId

	roleBindings, err := c.LookupRoleBinding(principal, role, *cDetails)
	if err != nil {
		return diag.FromErr(err)
	}
	for _, v := range roleBindings {
		if v.Name == d.Get("name").(string) {
			err = d.Set("resource_type", v.ResourceType)
			if err == nil {
				err = d.Set("pattern_type", v.PatternType)
			}
		}
	}
	return diag.FromErr(err)
}

func kafkaTopicRBACCreate(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*confluent.Client)
	principal := d.Get("principal").(string)
	role := d.Get("role").(string)
	clusterId := d.Get("cluster_id").(string)

	cDetails := &confluent.ClusterDetails{
		Clusters: confluent.Clusters{
			KafkaCluster: clusterId,
		},
	}

	u := confluent.RoleBinding{
		Scope: *cDetails,
		ResourcePatterns: []confluent.ResourcePattern{
			{
				ResourceType: d.Get("resource_type").(string),
				Name:         d.Get("name").(string),
				PatternType:  d.Get("pattern_type").(string),
			},
		},
	}

	err = c.IncreaseRoleBinding(principal, role, u)
	if err != nil {
		return diag.FromErr(err)
	}

	rId := clusterId + "|" + principal + "|" + role + "|" + d.Get("resource_type").(string) + "|" + d.Get("name").(string) + "|" + d.Get("pattern_type").(string)
	d.SetId(rId)
	return nil
}

func kafkaTopicRBACDelete(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*confluent.Client)
	principal := d.Get("principal").(string)
	role := d.Get("role").(string)
	clusterId := d.Get("cluster_id").(string)

	cDetails := &confluent.ClusterDetails{
		Clusters: confluent.Clusters{
			KafkaCluster: clusterId,
		},
	}

	u := confluent.RoleBinding{
		Scope: *cDetails,
		ResourcePatterns: []confluent.ResourcePattern{
			{
				ResourceType: d.Get("resource_type").(string),
				Name:         d.Get("name").(string),
				PatternType:  d.Get("pattern_type").(string),
			},
		},
	}

	err = c.DecreaseRoleBinding(principal, role, u)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
