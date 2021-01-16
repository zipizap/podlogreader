package main

import (
	log "github.com/Sirupsen/logrus"
	core_v1 "k8s.io/api/core/v1"
)

// Handler interface contains the methods that are required
type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(objOld, objNew interface{})
}

// TheHandler is a sample implementation of Handler
type TheHandler struct{}

// Init handles any handler initialization
func (t *TheHandler) Init() error {
	log.Info("TheHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *TheHandler) ObjectCreated(obj interface{}) {
	log.Info("TheHandler.ObjectCreated")
	// assert the type to a Pod object to pull out relevant data
	pod := obj.(*core_v1.Pod)
	log.Infof("    ResourceVersion: %s", pod.ObjectMeta.ResourceVersion)
	log.Infof("    NodeName: %s", pod.Spec.NodeName)
	log.Infof("    Phase: %s", pod.Status.Phase)
}

// ObjectDeleted is called when an object is deleted
func (t *TheHandler) ObjectDeleted(obj interface{}) {
	log.Info("TheHandler.ObjectDeleted")
}

// ObjectUpdated is called when an object is updated
func (t *TheHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("TheHandler.ObjectUpdated")
}
