package internal

import (
	"context"
	"errors"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

type Client struct {
	ecsiface.ECSAPI
	elbv2iface.ELBV2API
	ec2iface.EC2API
}

func NewClient(sess *session.Session) *Client {
	return NewClientWithInterface(ecs.New(sess), elbv2.New(sess), ec2.New(sess))
}

func NewClientWithInterface(ecs ecsiface.ECSAPI, elbv2 elbv2iface.ELBV2API, ec2 ec2iface.EC2API) *Client {
	return &Client{
		ECSAPI:   ecs,
		ELBV2API: elbv2,
		EC2API:   ec2,
	}
}

func (c *Client) ListAllContainerInstances(ctx context.Context, i *ecs.ListContainerInstancesInput) ([]*string, error) {
	var containerInstances []*string
	err := c.ECSAPI.ListContainerInstancesPagesWithContext(ctx, i, func(loi *ecs.ListContainerInstancesOutput, b bool) bool {
		containerInstances = append(containerInstances, loi.ContainerInstanceArns...)
		return b
	})
	if err != nil {
		return nil, err
	}
	return containerInstances, nil
}

func (c *Client) DescribeAllContainerInstances(ctx context.Context, cluster *string) ([]*ecs.ContainerInstance, error) {
	containerInstances, err := c.ListAllContainerInstances(ctx, &ecs.ListContainerInstancesInput{
		Cluster: cluster,
	})
	if err != nil {
		return nil, err
	}
	o, err := c.DescribeContainerInstancesWithContext(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            cluster,
		ContainerInstances: containerInstances,
	})
	if err != nil {
		return nil, err
	}
	if len(o.Failures) != 0 {
		for _, v := range o.Failures {
			log.Printf("%s is not found. reason: %s", *v.Arn, *v.Reason)
		}
		return nil, errors.New("Not found container instances")
	}
	return o.ContainerInstances, nil
}

func (c *Client) DescribeTargetGroupArns(ctx context.Context, targetGroups []*string) ([]*string, error) {
	var targetGroupArns []*string
	c.ELBV2API.DescribeTargetGroupsPagesWithContext(ctx, &elbv2.DescribeTargetGroupsInput{
		Names: targetGroups,
	}, func(o *elbv2.DescribeTargetGroupsOutput, b bool) bool {
		for _, v := range o.TargetGroups {
			if v.TargetGroupArn != nil {
				targetGroupArns = append(targetGroupArns, v.TargetGroupArn)
			}
		}
		return b
	})
	return targetGroupArns, nil
}

func (c *Client) DescribeAllInstancesInTargetGroups(ctx context.Context, targetGroupArns []*string) (ds []*elbv2.TargetHealthDescription, err error) {
	for _, arn := range targetGroupArns {
		o, err := c.ELBV2API.DescribeTargetHealthWithContext(ctx, &elbv2.DescribeTargetHealthInput{
			TargetGroupArn: arn,
		})
		if err != nil {
			return nil, err
		}
		ds = append(ds, o.TargetHealthDescriptions...)
	}
	return
}

func (c *Client) ListAllServicesInCluster(ctx context.Context, cluster *string) (services []*string, err error) {
	err = c.ListServicesPagesWithContext(ctx, &ecs.ListServicesInput{
		Cluster: cluster,
	}, func(o *ecs.ListServicesOutput, b bool) bool {
		services = append(services, o.ServiceArns...)
		return b
	})
	return
}

func (c *Client) GetAllTargetGroupsInCluster(ctx context.Context, cluster *string) ([]*string, error) {
	arns, err := c.ListAllServicesInCluster(ctx, cluster)
	if err != nil {
		return nil, err
	}
	services, err := c.DescribeServicesWithContext(ctx, &ecs.DescribeServicesInput{
		Cluster:  cluster,
		Services: arns,
	})
	if err != nil {
		return nil, err
	}
	if len(services.Failures) != 0 {
		return nil, errors.New("Service is not found")
	}
	var targetGroupArns []*string
	for _, v := range services.Services {
		for _, l := range v.LoadBalancers {
			targetGroupArns = append(targetGroupArns, l.TargetGroupArn)
		}
	}
	return targetGroupArns, nil
}
