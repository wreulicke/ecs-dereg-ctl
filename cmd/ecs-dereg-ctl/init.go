package main

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/urfave/cli"
	ctl "github.com/wreulicke/ecs-dereg-ctl"
	"github.com/wreulicke/ecs-dereg-ctl/internal"
)

var app *cli.App

var version string

func init() {
	app = cli.NewApp()
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "instances, i",
			Usage: "deregister instances",
		},
		cli.StringFlag{
			Name:  "cluster, c",
			Usage: "ECS cluster",
		},
		cli.StringFlag{
			Name:  "region",
			Usage: "aws region",
		},
		cli.StringFlag{
			Name:  "profile, p",
			Usage: "aws profile",
		},
	}
	app.Action = action
}

func action(c *cli.Context) error {
	if !c.IsSet("instances") {
		return errors.New("instances is required")
	}
	if !c.IsSet("cluster") {
		return errors.New("cluster is required")
	}
	var profile string
	if c.IsSet("profile") {
		profile = c.String("profile")
	}
	var region *string
	if c.IsSet("region") {
		r := c.String("region")
		region = &r
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: region,
		},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return err
	}

	client := internal.NewClient(sess)
	return ctl.GracefulShutdown(client, c.String("cluster"), c.StringSlice("instances"))
}
