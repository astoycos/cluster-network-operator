package flowsconfig

import (
	"context"
	"log"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/cluster-network-operator/pkg/names"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ovsFlowsConfigMapName   = "ovs-flows-config"
	ovsFlowsConfigNamespace = names.APPLIED_NAMESPACE
)

type FlowsConfig struct {
	// Target IP:port of the flow collector
	Target string

	// CacheActiveTimeout is the max period during which the reporter will aggregate flows before sending
	CacheActiveTimeout *uint

	// CacheMaxFlows is the max number of flows in an aggregate; when reached, the reporter sends the flows
	CacheMaxFlows *uint

	// Sampling is the sampling rate on the reporter. 100 means one flow on 100 is sent. 0 means disabled.
	Sampling *uint
}

// WatchForConfigMap setups the passed controller to watch for any change in the
// configmap openshift-network-operator/ovs-flows-config
func WatchForConfigMap(c controller.Controller) error {
	return c.Watch(&source.Kind{Type: &corev1.ConfigMap{}},
		handler.EnqueueRequestsFromMapFunc(ReconcileRequests),
		predicate.ResourceVersionChangedPredicate{},
	)
}

// ReconcileRequests filters non-ovs-flows-config events and forwards a request to the
// openshift-network-operator/cluster operator
func ReconcileRequests(object client.Object) []reconcile.Request {
	if object == nil {
		log.Println(ovsFlowsConfigMapName + ": can't create a reconcile request for a nil object")
		return nil
	}
	n := object.GetName()
	ns := object.GetNamespace()
	if n != ovsFlowsConfigMapName || ns != ovsFlowsConfigNamespace {
		return nil
	}
	log.Println(ovsFlowsConfigMapName + ": enqueuing operator reconcile request from configmap")
	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Name:      names.OPERATOR_CONFIG,
		Namespace: names.APPLIED_NAMESPACE,
	}}}
}

// Bootstrap looks for the openshift-network-operator/ovs-flows-config configmap, and
// returns it or returns nil if it does not exist (or can't be properly parsed).
// Usually, the second argument will be net.LookupIP
func Bootstrap(cl client.Reader) *FlowsConfig {
	cm := corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{
		Name:      ovsFlowsConfigMapName,
		Namespace: ovsFlowsConfigNamespace,
	}, &cm); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Printf("%s: error fetching configmap: %v", ovsFlowsConfigMapName, err)
		}
		// ovs-flows-config is not defined. Ignoring from bootstrap
		return nil
	}
	fc := FlowsConfig{}
	// fetching string fields and transforming them to OVS format
	if st, ok := cm.Data["sharedTarget"]; ok {
		fc.Target = st
	} else if np, ok := cm.Data["nodePort"]; ok {
		// empty host will be interpreted as Node IP by ovn-kubernetes
		fc.Target = ":" + np
	} else {
		log.Printf("%s: wrong data section: either sharedTarget or nodePort sections are needed: %+v",
			ovsFlowsConfigMapName, cm.Data)
		return nil
	}

	if catStr, ok := cm.Data["cacheActiveTimeout"]; ok {
		if catd, err := time.ParseDuration(catStr); err != nil {
			log.Printf("%s: wrong cacheActiveTimeout value %s. Ignoring: %v",
				ovsFlowsConfigMapName, catStr, err)
		} else {
			catf := catd.Seconds()
			catu := uint(catf)
			if catf != float64(catu) {
				log.Printf("%s: cacheActiveTimeout %s will be truncated to %d seconds",
					ovsFlowsConfigMapName, catStr, catu)
			}
			fc.CacheActiveTimeout = &catu
		}
	}

	if cmfStr, ok := cm.Data["cacheMaxFlows"]; ok {
		if cmf, err := strconv.ParseUint(cmfStr, 10, 32); err != nil {
			log.Printf("%s: wrong cacheMaxFlows value %s. Ignoring: %v",
				ovsFlowsConfigMapName, cmfStr, err)
		} else {
			cmfu := uint(cmf)
			fc.CacheMaxFlows = &cmfu
		}
	}

	if sStr, ok := cm.Data["sampling"]; ok {
		if sampling, err := strconv.ParseUint(sStr, 10, 32); err != nil {
			log.Printf("%s: wrong sampling value %s. Ignoring: %v",
				ovsFlowsConfigMapName, sStr, err)
		} else {
			su := uint(sampling)
			fc.Sampling = &su
		}
	}

	return &fc
}
