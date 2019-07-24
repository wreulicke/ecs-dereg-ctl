package internal

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

func containsTarget(instances []string, instance string) bool {
	for _, v := range instances {
		if v == instance {
			return true
		}
	}
	return false
}

func waitForDeregister(client *Client, instances []string, targetGroups []*string) error {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			log.Println("Describe target health")
			healthDescriptions, err := client.DescribeAllInstancesInTargetGroups(targetGroups)
			if err != nil {
				return errors.Wrap(err, "Cannot describe all target healths")
			}
			var foundAnyInstance bool
			for _, v := range healthDescriptions {
				if v.Target.Id != nil {

					foundAnyInstance = foundAnyInstance || containsTarget(instances, *v.Target.Id)
				}
				if foundAnyInstance {
					log.Println("Yet contains...")
					break
				}
			}
			if !foundAnyInstance {
				log.Println("Deregistered")
				break loop
			}
		}
	}
	return nil
}

func GracefulShutdown(client *Client, cluster string, instances []string) error {
	targetGroupArns, err := client.GetAllTargetGroupsInCluster(aws.String(cluster))
	if err != nil {
		return err
	} else if len(targetGroupArns) == 0 {
		return fmt.Errorf("target group is not found. %v", targetGroupArns)
	}

	containerInstances, err := client.DescribeAllContainerInstances(aws.String(cluster))
	if err != nil {
		return err
	}

	var drainingInstances []*ecs.ContainerInstance
	for _, ci := range containerInstances {
		if containsTarget(instances, *ci.Ec2InstanceId) {
			log.Printf("found instances. %s", *ci.Ec2InstanceId)
			drainingInstances = append(drainingInstances, ci)
		}
	}
	var arns []*string
	for _, ci := range drainingInstances {
		arns = append(arns, ci.ContainerInstanceArn)
	}
	if len(arns) == 0 {
		return errors.Errorf("Designated instances are not found as container instance in cluster. instances: %v", instances)
	}
	_, err = client.UpdateContainerInstancesState(&ecs.UpdateContainerInstancesStateInput{
		Cluster:            aws.String(cluster),
		Status:             aws.String("DRAINING"),
		ContainerInstances: arns,
	})
	if err != nil {
		return err
	}

	for _, arn := range arns {
		o, err := client.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
			Cluster:            aws.String(cluster),
			ContainerInstances: []*string{arn},
		})
		if err != nil {
			return errors.Wrap(err, "Cannot describe container instances")
		}
		if len(o.Failures) != 0 {
			return errors.Errorf("Cannot describe container instances. failures: %v", o.Failures)
		}
		if *o.ContainerInstances[0].RunningTasksCount != 0 {
			log.Println("Found running tasks. wait for draining")
			time.Sleep(1 * time.Second)
			continue
		}
		i := &ecs.DeregisterContainerInstanceInput{
			Cluster:           aws.String(cluster),
			ContainerInstance: arn,
		}
		_, err = client.DeregisterContainerInstance(i)
		if err != nil {
			return errors.Wrap(err, "Cannot deregister container instances")
		}
	}
	// TODO waitForDeregister
	err = waitForDeregister(client, instances, targetGroupArns) // TODO multiple
	if err != nil {
		return err
	}
	terminateResponse, err := client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice(instances),
	})
	if err != nil {
		return err
	}
	for _, v := range terminateResponse.TerminatingInstances {
		log.Printf("%s is terminated.", *v.InstanceId)
	}
	return nil
}
