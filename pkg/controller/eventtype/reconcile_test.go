/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventtype

import (
	"context"
	"errors"
	"fmt"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()
)

const (
	eventSourceName = "event-source"
	etNamespace     = "test-namespace"
	etName          = "test-event-type"
	reconcileKey    = etNamespace + "/" + etName

	fn1 = "fake-feed-name-1"
	fn2 = "fake-feed-name-2"

	getError    = "test induced error getting"
	listError   = "test induced error listing"
	updateError = "test induced error updating"
)

func init() {
	// Add types to scheme
	feedsv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:         "missing EventType",
			ReconcileKey: reconcileKey,
		},
		{
			Name:         "Internal error getting EventType",
			ReconcileKey: reconcileKey,
			WantErrMsg:   getError,
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(_ client.Client, _ context.Context, _ client.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New(getError)
					},
				},
			},
		},
		{
			Name: "new EventType: adds Finalizer",
			InitialState: []runtime.Object{
				getEventType(false),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				getEventType(true),
			},
		},
		{
			Name: "tries to add finalizer -- second EventType.Get() errors",
			InitialState: []runtime.Object{
				getEventType(false),
			},
			WantPresent: []runtime.Object{
				// Unable to update the EventType, it should still be in the initial state.
				getEventType(false),
			},
			ReconcileKey: reconcileKey,
			WantErrMsg:   getError,
			Mocks: controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					// The first Get should pass through to the inner client.
					func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						realResponse := innerClient.Get(ctx, key, obj)
						return controllertesting.Handled, realResponse
					},
					// The second Get, which runs inside the Update logic, should error.
					func(_ client.Client, _ context.Context, _ client.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New(getError)
					},
				},
			},
		},
		{
			Name: "tries to add finalizer -- EventType.Update() errors",
			InitialState: []runtime.Object{
				getEventType(false),
			},
			WantPresent: []runtime.Object{
				// Unable to update the EventType, it should still be in the initial state.
				getEventType(false),
			},
			ReconcileKey: reconcileKey,
			WantErrMsg:   updateError,
			Mocks: controllertesting.Mocks{
				MockUpdates: []controllertesting.MockUpdate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New(updateError)
					},
				},
			},
		},
		{
			Name: "old EventType: already has Finalizer",
			InitialState: []runtime.Object{
				getEventType(true),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				getEventType(true),
			},
		},
		{
			Name: "deleting EventType: no Feeds",
			InitialState: []runtime.Object{
				getDeletingEventType(true),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				// The Finalizer should have been removed.
				getDeletingEventType(false),
			},
		},
		{
			Name: "deleting EventType: Feeds not using EventType",
			InitialState: []runtime.Object{
				getDeletingEventType(true),
				getFeedUsingOtherEventType(),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				// The Finalizer should have been removed.
				getDeletingEventType(false),
			},
		}, {
			Name: "deleting EventType: Feeds using EventType in different namespace",
			InitialState: []runtime.Object{
				getDeletingEventType(true),
				getFeedUsingEventTypeInDifferentNamespace(),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				// The Finalizer should have been removed.
				getDeletingEventType(false),
			},
		},
		{
			Name: "deleting EventType: Feeds using EventType",
			InitialState: []runtime.Object{
				getDeletingEventTypeWithCompleteStatus(),
				getFeedUsingEventType(fn1),
				getFeedUsingEventType(fn2),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				// There are Feeds still using the EventType, it should still have its Finalizer and a
				// new Status added stating which Feeds are blocking its deletion.
				getDeletingEventTypeWithCompleteAndInUseStatus(fn1 + ", " + fn2),
			},
		},
		{
			Name: "deleting EventType: can't list Feeds",
			InitialState: []runtime.Object{
				getDeletingEventType(true),
			},
			ReconcileKey: reconcileKey,
			WantPresent: []runtime.Object{
				// Unable to update the EventType, it should still be in the initial state.
				getDeletingEventType(true),
			},
			WantErrMsg: listError,
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New(listError)
					},
				},
			},
		},
	}
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   zap.NewNop(),
		}
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func objectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getEventType(finalizer bool) *feedsv1alpha1.EventType {
	et := &feedsv1alpha1.EventType{
		TypeMeta: metav1.TypeMeta{
			APIVersion: feedsv1alpha1.SchemeGroupVersion.String(),
			Kind:       "EventType",
		},
		ObjectMeta: objectMeta(etNamespace, etName),
		Spec: feedsv1alpha1.EventTypeSpec{
			EventSource: eventSourceName,
		},
	}
	if finalizer {
		et.Finalizers = []string{finalizerName}
	}
	return et
}

func getDeletingEventType(finalizer bool) *feedsv1alpha1.EventType {
	et := getEventType(finalizer)
	et.ObjectMeta.DeletionTimestamp = &deletionTime
	return et
}

func getDeletingEventTypeWithCompleteStatus() *feedsv1alpha1.EventType {
	et := getDeletingEventType(true)
	et.Status.Conditions = append(et.Status.Conditions, feedsv1alpha1.CommonEventTypeCondition{
		Type:   feedsv1alpha1.EventTypeComplete,
		Status: corev1.ConditionTrue,
	})
	return et
}

func getDeletingEventTypeWithCompleteAndInUseStatus(feedNames string) *feedsv1alpha1.EventType {
	et := getDeletingEventTypeWithCompleteStatus()
	et.Status.Conditions = append(et.Status.Conditions, feedsv1alpha1.CommonEventTypeCondition{
		Type:    feedsv1alpha1.EventTypeInUse,
		Status:  corev1.ConditionTrue,
		Message: "Still in use by the Feeds: " + feedNames,
	})
	return et
}

func getFeed(namespace, name, eventType string) *feedsv1alpha1.Feed {
	feed := &feedsv1alpha1.Feed{
		TypeMeta: metav1.TypeMeta{
			APIVersion: feedsv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Feed",
		},
		ObjectMeta: objectMeta(namespace, name),
		Spec: feedsv1alpha1.FeedSpec{
			Trigger: feedsv1alpha1.EventTrigger{
				EventType: eventType,
			},
		},
	}
	return feed
}

func getFeedUsingOtherEventType() *feedsv1alpha1.Feed {
	return getFeed(etNamespace, "feed-not-using-event-type", "some-other-event-type")
}

func getFeedUsingEventType(feedName string) *feedsv1alpha1.Feed {
	return getFeed(etNamespace, feedName, etName)
}

func getFeedUsingEventTypeInDifferentNamespace() *feedsv1alpha1.Feed {
	return getFeed("some-other-namespace", "feed-in-different-namespace", etName)
}
