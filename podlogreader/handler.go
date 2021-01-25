package main

import (
	"encoding/json"
	"errors"
	_ "fmt"
	log "github.com/Sirupsen/logrus"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const controllerPrefix = "podlogreader"
const controllerLabelKey = controllerPrefix + "-affiliate"
const controllerLabelValue = "enable"

// Handler interface contains the methods that are required
type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(objOld, objNew interface{})
}

// TheHandler is a sample implementation of Handler
type TheHandler struct {
	client                        kubernetes.Interface
	option_CreateSaAndRolebinding bool
}

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
	t.processPod(pod)

	// log.Infof("    ResourceVersion: %s", pod.ObjectMeta.ResourceVersion)
	// log.Infof("    NodeName: %s", pod.Spec.NodeName)
	// log.Infof("    Phase: %s", pod.Status.Phase)
}

// ObjectDeleted is called when an object is deleted
func (t *TheHandler) ObjectDeleted(obj interface{}) {
	log.Info("TheHandler.ObjectDeleted")
	// assert the type to a Pod object to pull out relevant data
	pod := obj.(*core_v1.Pod)
	t.processPod(pod)
}

// ObjectUpdated is called when an object is updated
func (t *TheHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("TheHandler.ObjectUpdated")
	// assert the type to a Pod object to pull out relevant data
	pod := objNew.(*core_v1.Pod)
	t.processPod(pod)

}

func (t *TheHandler) pp(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "  ")
	return string(s)
}

func (t *TheHandler) checkIfPodContainsLabel(pod *core_v1.Pod, labelKey string, labelValue string) bool {
	labelFoundValue, labelFoundExists := pod.ObjectMeta.Labels[labelKey]
	if !labelFoundExists {
		// label was not found
		return false
	}

	if labelFoundValue == labelValue {
		// label exists and has matching value
		return true
	} else {
		// labels exista but its value does not match
		return false
	}
}

// Usefull:
//   https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#rolebinding-v1-rbac-authorization-k8s-io

func (t *TheHandler) discoverReplicasetOfPod(pod *core_v1.Pod) (*apps_v1.ReplicaSet, error) {
	ns := pod.ObjectMeta.Namespace
	for _, oRef := range pod.ObjectMeta.OwnerReferences {
		if oRef.Kind == "ReplicaSet" {
			rsName := oRef.Name
			rs, err := t.client.AppsV1().ReplicaSets(ns).Get(rsName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return rs, nil
		}
	}
	return nil, errors.New("This pod does not seem to have an ownerRef of kind ReplicaSet - this is unexpected...")
}

func (t *TheHandler) discoverDeploymentOfReplicaset(rs *apps_v1.ReplicaSet) (*apps_v1.Deployment, error) {
	ns := rs.ObjectMeta.Namespace
	for _, oRef := range rs.ObjectMeta.OwnerReferences {
		if oRef.Kind == "Deployment" {
			deployName := oRef.Name
			deploy, err := t.client.AppsV1().Deployments(ns).Get(deployName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return deploy, nil
		}
	}
	return nil, errors.New("This replicaset does not seem to have an ownerRef of kind Deployment - this is unexpected...")
}

func (t *TheHandler) discoverPodsNamesOfDeployment(deploy *apps_v1.Deployment) ([]string, error) {
	podNamesList := []string{}
	labelSelector := deploy.Spec.Selector
	matchLabels := labelSelector.MatchLabels // matchLabels type is map[string]string
	ns := deploy.ObjectMeta.Namespace
	podList, err := t.client.CoreV1().Pods(ns).List(meta_v1.ListOptions{
		LabelSelector: labels.Set(matchLabels).String(),
		Limit:         100,
	})
	if err != nil {
		return podNamesList, err
	}
	for _, aPod := range podList.Items {
		podNamesList = append(podNamesList, aPod.ObjectMeta.Name)
	}
	return podNamesList, nil
}

func (t *TheHandler) createOrUpdateRole(roleName string, podNamesList []string, ns string) (*rbac_v1.Role, bool, error) {
	desiredRole := &rbac_v1.Role{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Rules: []rbac_v1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"pods/log"},
				ResourceNames: podNamesList,
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	}

	role, err := t.client.RbacV1().Roles(ns).Get(roleName, meta_v1.GetOptions{})
	if err != nil {
		// role does not exist, lets create it
		role, err = t.client.RbacV1().Roles(ns).Create(desiredRole)
		if err != nil {
			return nil, false, err
		}
		return role, true, nil

	} else {
		// role already exists, lets update it
		role, err = t.client.RbacV1().Roles(ns).Update(desiredRole)
		if err != nil {
			return nil, false, err
		}
		return role, false, nil
	}
}

func (t *TheHandler) createOrIgnoreServiceaccount(saName string, ns string) (*core_v1.ServiceAccount, bool, error) {
	desiredSa := &core_v1.ServiceAccount{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      saName,
			Namespace: ns,
		},
	}

	sa, err := t.client.CoreV1().ServiceAccounts(ns).Get(saName, meta_v1.GetOptions{})
	if err != nil {
		// sa does not exist, lets create it
		sa, err = t.client.CoreV1().ServiceAccounts(ns).Create(desiredSa)
		if err != nil {
			return nil, false, err
		}
		return sa, true, nil

	} else {
		// sa already exists, lets update it
		sa, err = t.client.CoreV1().ServiceAccounts(ns).Update(desiredSa)
		if err != nil {
			return nil, false, err
		}
		return sa, false, nil
	}
}

func (t *TheHandler) createOrIgnoreRolebinding(rolebindingName string, ns string, saName string, roleName string) (*rbac_v1.RoleBinding, bool, error) {
	desiredRolebinding := &rbac_v1.RoleBinding{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      rolebindingName,
			Namespace: ns,
		},
		RoleRef: rbac_v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbac_v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: ns,
			},
		},
	}

	rolebinding, err := t.client.RbacV1().RoleBindings(ns).Get(rolebindingName, meta_v1.GetOptions{})
	if err != nil {
		// rolebinding does not exist, lets create it
		rolebinding, err = t.client.RbacV1().RoleBindings(ns).Create(desiredRolebinding)
		if err != nil {
			return nil, false, err
		}
		return rolebinding, true, nil

	} else {
		// rolebinding already exists, lets update it
		rolebinding, err = t.client.RbacV1().RoleBindings(ns).Update(desiredRolebinding)
		if err != nil {
			return nil, false, err
		}
		return rolebinding, false, nil
	}
}

func (t *TheHandler) processPod(pod *core_v1.Pod) {
	log.Info("TheHandler.processPod")

	// Check if pod  contains controllerLabel, and if not then return immediately
	if !t.checkIfPodContainsLabel(pod, controllerLabelKey, controllerLabelValue) {
		return
	}
	ns := pod.ObjectMeta.Namespace
	log.Info(">> Found Pod '", pod.ObjectMeta.Name, "', in namespace '", ns, "', with the controllerLabel '", controllerLabelKey, ": ", controllerLabelValue, "'")

	// Discover the pod's owner-ref replicaset, if it exists
	rs, err := t.discoverReplicasetOfPod(pod)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info(">> Found ReplicaSet: '", rs.ObjectMeta.Name, "'")

	// Discover the replicaset's owner-ref deployment, if it exists
	deploy, err := t.discoverDeploymentOfReplicaset(rs)
	if err != nil {
		log.Error(err)
		return
	}
	deployName := deploy.ObjectMeta.Name
	log.Info(">> Found Deployment: '", deployName, "'")

	// Discover the names-of-all-pods-of-the-deployment
	podNamesList, err := t.discoverPodsNamesOfDeployment(deploy)
	log.Info(">> Found podNamesList: '", podNamesList, "'")

	roleName := controllerPrefix + "-" + deployName
	role, created, err := t.createOrUpdateRole(roleName, podNamesList, ns)
	if err != nil {
		log.Error(err)
		return
	}
	if created {
		log.Info(">> Created role: '", role.ObjectMeta.Name, "' with resourceNames as the podNamesList")
	} else {
		log.Info(">> Updated role: '", role.ObjectMeta.Name, "' with resourceNames '", podNamesList, "'")
	}

	if t.option_CreateSaAndRolebinding == true {
		saName := controllerPrefix + "-" + deployName
		sa, created, err := t.createOrIgnoreServiceaccount(saName, ns)
		if err != nil {
			log.Error(err)
			return
		}
		if created {
			log.Info(">> Created ServiceAccount: '", sa.ObjectMeta.Name, "'")
		} else {
			log.Info(">> Ignored existing ServiceAccount: '", sa.ObjectMeta.Name, "'  (no change made)")
		}

		rolebindingName := controllerPrefix + "-" + deployName
		rolebinding, created, err := t.createOrIgnoreRolebinding(rolebindingName, ns, saName, roleName)
		if err != nil {
			log.Error(err)
			return
		}
		if created {
			log.Info(">> Created Rolebinding: '", rolebinding.ObjectMeta.Name, "'")
		} else {
			log.Info(">> Ignored existing Rolebinding: '", rolebinding.ObjectMeta.Name, "'   (no change made)")
		}
	}
}
