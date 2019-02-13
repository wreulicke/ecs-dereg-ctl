package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
	"github.com/wreulicke/ecs-dereg-ctl/internal"
	"gopkg.in/urfave/cli.v1"
)

func ContainsTarget(instances []string, instance string) bool {
	for _, v := range instances {
		if v == instance {
			return true
		}
	}
	return false
}

func waitForDeregister(ctx context.Context, client *internal.Client, instances []string, targetGroups []*string) error {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			log.Println("Describe target health")
			healthDescriptions, err := client.DescribeAllInstancesInTargetGroups(ctx, targetGroups)
			if err != nil {
				return errors.Wrap(err, "Cannot describe all target healths")
			}
			var foundAnyInstance bool
			for _, v := range healthDescriptions {
				if v.Target.Id != nil {
					foundAnyInstance = foundAnyInstance || ContainsTarget(instances, *v.Target.Id)
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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatal(err)
	}
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "instances, i",
			Usage: "deregister instances",
		},
		cli.StringFlag{
			Name:  "cluster, c",
			Usage: "cluster",
		},
	}
	app.Action = func(c *cli.Context) error {
		if !c.IsSet("instances") {
			return errors.New("instances is required")
		}
		if !c.IsSet("cluster") {
			return errors.New("cluster is required")
		}

		client := internal.NewClient(sess)
		targetGroupArns, err := client.GetAllTargetGroupsInCluster(ctx, aws.String(c.String("cluster")))
		if err != nil {
			return err
		}

		containerInstances, err := client.DescribeAllContainerInstances(ctx, aws.String(c.String("cluster")))
		if err != nil {
			return err
		}

		instances := c.StringSlice("instances")
		var drainingInstances []*ecs.ContainerInstance
		for _, ci := range containerInstances {
			if ContainsTarget(instances, *ci.Ec2InstanceId) {
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
		_, err = client.UpdateContainerInstancesStateWithContext(ctx, &ecs.UpdateContainerInstancesStateInput{
			Cluster:            aws.String(c.String("cluster")),
			Status:             aws.String("DRAINING"),
			ContainerInstances: arns,
		})
		if err != nil {
			return err
		}

		for _, arn := range arns {
			o, err := client.DescribeContainerInstancesWithContext(ctx, &ecs.DescribeContainerInstancesInput{
				Cluster:            aws.String(c.String("cluster")),
				ContainerInstances: []*string{arn},
			})
			if err != nil {
				return errors.Wrap(err, "Cannot descrobe container instances")
			}
			if len(o.Failures) != 0 {
				return errors.Errorf("Cannot descrobe container instances. failures: %v", o.Failures)
			}
			if *o.ContainerInstances[0].RunningTasksCount != 0 {
				log.Println("Found running tasks. wait for draining")
				time.Sleep(1 * time.Second)
				continue
			}
			i := &ecs.DeregisterContainerInstanceInput{
				Cluster:           aws.String(c.String("cluster")),
				ContainerInstance: arn,
			}
			_, err = client.DeregisterContainerInstanceWithContext(ctx, i)
			if err != nil {
				return errors.Wrap(err, "Cannot deregister container instances")
			}
		}
		// TODO waitForDeregister
		err = waitForDeregister(ctx, client, instances, targetGroupArns) // TODO multiple
		if err != nil {
			return err
		}
		terminateResponse, err := client.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	doneCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		err := app.Run(os.Args)
		if err != nil {
			errCh <- err
			return
		}
		doneCh <- struct{}{}
	}()
	select {
	case <-c:
		log.Println("canceled")
		cancel()
	case err := <-errCh:
		log.Fatal(err)
		cancel()
	case <-doneCh:
		log.Println("done")
		cancel()
	}
}
