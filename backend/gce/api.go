package gce

import (
	"fmt"
	"time"

	"github.com/coreos/flannel/Godeps/_workspace/src/code.google.com/p/goauth2/compute/serviceaccount"
	"github.com/coreos/flannel/Godeps/_workspace/src/code.google.com/p/google-api-go-client/compute/v1"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
)

type gceAPI struct {
	project        string
	computeService *compute.Service
	gceNetwork     *compute.Network
	gceInstance    *compute.Instance
}

func newAPI() (*gceAPI, error) {
	client, err := serviceaccount.NewClient(&serviceaccount.Options{})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}

	cs, err := compute.New(client)
	if err != nil {
		return nil, fmt.Errorf("error creating compute service: %v", err)
	}

	networkName, err := networkFromMetadata()
	if err != nil {
		return nil, fmt.Errorf("error getting network metadata: %v", err)
	}

	prj, err := projectFromMetadata()
	if err != nil {
		return nil, fmt.Errorf("error getting project: %v", err)
	}

	instanceName, err := instanceNameFromMetadata()
	if err != nil {
		return nil, fmt.Errorf("error getting instance name: %v", err)
	}

	instanceZone, err := instanceZoneFromMetadata()
	if err != nil {
		return nil, fmt.Errorf("error getting instance zone: %v", err)
	}

	gn, err := cs.Networks.Get(prj, networkName).Do()
	if err != nil {
		return nil, fmt.Errorf("error getting network from compute service: %v", err)
	}

	gi, err := cs.Instances.Get(prj, instanceZone, instanceName).Do()
	if err != nil {
		return nil, fmt.Errorf("error getting instance from compute service: %v", err)
	}

	return &gceAPI{
		project:        prj,
		computeService: cs,
		gceNetwork:     gn,
		gceInstance:    gi,
	}, nil
}

func (api *gceAPI) getRoute(subnet string) (*compute.Route, error) {
	routeName := formatRouteName(subnet)
	return api.computeService.Routes.Get(api.project, routeName).Do()
}

func (api *gceAPI) deleteRoute(subnet string) (*compute.Operation, error) {
	routeName := formatRouteName(subnet)
	return api.computeService.Routes.Delete(api.project, routeName).Do()
}

func (api *gceAPI) insertRoute(subnet string) (*compute.Operation, error) {
	log.Infof("Inserting route for subnet: %v", subnet)
	route := &compute.Route{
		Name:            formatRouteName(subnet),
		DestRange:       subnet,
		Network:         api.gceNetwork.SelfLink,
		NextHopInstance: api.gceInstance.SelfLink,
		Priority:        1000,
		Tags:            []string{},
	}
	return api.computeService.Routes.Insert(api.project, route).Do()
}

func (api *gceAPI) pollOperationStatus(operationName string) error {
	for i := 0; i < 100; i++ {
		operation, err := api.computeService.GlobalOperations.Get(api.project, operationName).Do()
		if err != nil {
			return fmt.Errorf("error fetching operation status: %v", err)
		}

		if operation.Error != nil {
			return fmt.Errorf("error running operation: %v", operation.Error)
		}

		if i%5 == 0 {
			log.Infof("%v operation status: %v waiting for completion...", operation.OperationType, operation.Status)
		}

		if operation.Status == "DONE" {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for operation to finish")
}

func formatRouteName(subnet string) string {
	return fmt.Sprintf("flannel-%s", replacer.Replace(subnet))
}
