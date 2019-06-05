package common

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"
	clientcache "k8s.io/client-go/tools/cache"

	"sigs.k8s.io/controller-runtime/pkg/cache"

	machinev1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
)

func WaitForMachines(cfg *rest.Config, testFunc func([]*machinev1.Machine) bool, timeOut time.Duration) error {
	logger := log.WithField("client", "machine")
	logger.Infof("Waiting for Machine")
	stop := make(chan struct{})
	done := make(chan struct{})
	scheme, err := machinev1.SchemeBuilder.Build()
	if err != nil {
		return err
	}
	internalCache, err := cache.New(cfg, cache.Options{
		Namespace: "openshift-machine-api",
		Scheme:    scheme,
	})
	if err != nil {
		return err
	}
	informer, err := internalCache.GetInformer(&machinev1.Machine{})
	if err != nil {
		return err
	}
	onUpdate := func() {
		list := informer.GetStore().List()
		machineList := []*machinev1.Machine{}
		for _, item := range list {
			machine, ok := item.(*machinev1.Machine)
			if !ok {
				log.Fatalf("Item is not of type Machine: %#v", item)
				continue
			}
			machineList = append(machineList, machine)
		}
		if testFunc(machineList) {
			done <- struct{}{}
		}
	}
	informer.AddEventHandler(
		&clientcache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { onUpdate() },
			UpdateFunc: func(oldObj, newObj interface{}) { onUpdate() },
			DeleteFunc: func(obj interface{}) { onUpdate() },
		})

	go internalCache.Start(stop)
	defer func() { stop <- struct{}{} }()

	select {
	case <-time.After(timeOut):
		return fmt.Errorf("timed out waiting for machines")
	case <-done:
	}
	return nil
}
