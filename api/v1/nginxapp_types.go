/*

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SimpleNginxApp struct {
	Image       string `json:"image"`
	WelcomeText string `json:"welcometext"`
}

// NginxAppSpec defines the desired state of NginxApp
type NginxAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	MainApp   SimpleNginxApp `json:"mainapp"`
	HelperApp SimpleNginxApp `json:"helperapp"`
}

// +kubebuilder:validation:Enum=Pending;Running;Failed
type ApplicationPhase string

const (
	PhasePending ApplicationPhase = "Pending"
	PhaseRunning ApplicationPhase = "Running"
	PhaseFailed  ApplicationPhase = "Failed"
)

// NginxAppStatus defines the observed state of NginxApp
type NginxAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Phase ApplicationPhase `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName="napp"

// NginxApp is the Schema for the nginxapps API
type NginxApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxAppSpec   `json:"spec,omitempty"`
	Status NginxAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NginxAppList contains a list of NginxApp
type NginxAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxApp{}, &NginxAppList{})
}
