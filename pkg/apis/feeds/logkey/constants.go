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

package logkey

const (
	kNative = "knative.dev/"

	// ClusterEventSource is the key used for cluster scoped event source name in structured logs
	ClusterEventSource = kNative + "clustereventsource"

	// ClusterEventType is the key used for cluster event type name in structured logs
	ClusterEventType = kNative + "clustereventtype"

	// EventSource is the key used for event source name in structured logs
	EventSource = kNative + "eventsource"

	// EventType is the key used for event type name in structured logs
	EventType = kNative + "eventtype"

	// Feed is the key used for feed name in structured logs
	Feed = kNative + "feed"
)
