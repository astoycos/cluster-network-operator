package flowsconfig

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	errors2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBootStrapOvsConfigMap_SharedTarget(t *testing.T) {
	fc := Bootstrap(&fakeClientReader{
		configMap: &corev1.ConfigMap{
			Data: map[string]string{
				"sharedTarget":       "1.2.3.4:3030",
				"cacheActiveTimeout": "3200ms",
				"cacheMaxFlows":      "33",
				"sampling":           "55",
			},
		},
	})

	assert.Equal(t, "1.2.3.4:3030", fc.Target)
	// verify that the 200ms get truncated
	assert.EqualValues(t, 3, *fc.CacheActiveTimeout)
	assert.EqualValues(t, 33, *fc.CacheMaxFlows)
	assert.EqualValues(t, 55, *fc.Sampling)
}

func TestBootStrapOvsConfigMap_NodePort(t *testing.T) {
	fc := Bootstrap(&fakeClientReader{
		configMap: &corev1.ConfigMap{
			Data: map[string]string{
				"nodePort":           "3131",
				"cacheActiveTimeout": "invalid timeout",
				"cacheMaxFlows":      "invalid int",
			},
		},
	})

	assert.Equal(t, ":3131", fc.Target)
	// verify that invalid or unspecified fields are ignored
	assert.Nil(t, fc.CacheActiveTimeout)
	assert.Nil(t, fc.CacheMaxFlows)
	assert.Nil(t, fc.Sampling)
}

func TestBootStrapOvsConfigMap_IncompleteMap(t *testing.T) {
	fc := Bootstrap(&fakeClientReader{
		configMap: &corev1.ConfigMap{
			Data: map[string]string{
				"cacheActiveTimeout": "3200ms",
				"cacheMaxFlows":      "33",
				"sampling":           "55",
			},
		},
	})

	// without sharedTarget nor nodePort, flow collection can't be set
	assert.Nil(t, fc)
}

func TestBootStrapOvsConfigMap_UnexistingMap(t *testing.T) {
	fc := Bootstrap(&fakeClientReader{configMap: nil})

	// without sharedTarget nor nodePort, flow collection can't be set
	assert.Nil(t, fc)
}

type fakeClientReader struct {
	configMap *corev1.ConfigMap
}

func (f *fakeClientReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object) error {
	if cmPtr, ok := obj.(*corev1.ConfigMap); !ok {
		return fmt.Errorf("expecting *corev1.ConfigMap, got %T", obj)
	} else if f.configMap == nil {
		return &errors2.StatusError{ErrStatus: metav1.Status{
			Reason: metav1.StatusReasonNotFound,
		}}
	} else {
		*cmPtr = *f.configMap
	}
	return nil
}

func (f *fakeClientReader) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return errors.New("unexpected invocation to List")
}
