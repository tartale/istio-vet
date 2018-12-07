/*
Copyright 2018 Aspen Mesh Authors.

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

package conflictingvirtualservicehost

import (
	"fmt"
	"strings"

	v1alpha3 "github.com/aspenmesh/istio-client-go/pkg/apis/networking/v1alpha3"
	netv1alpha3 "github.com/aspenmesh/istio-client-go/pkg/client/listers/networking/v1alpha3"
	apiv1 "github.com/aspenmesh/istio-vet/api/v1"
	"github.com/aspenmesh/istio-vet/pkg/vetter"
	"github.com/aspenmesh/istio-vet/pkg/vetter/util"
	"k8s.io/client-go/listers/core/v1"
)

const (
	vetterID       = "ConflictingVirtualServiceHost"
	vsHostNoteType = "host-in-multiple-vs"
	vsHostSummary  = "Multiple VirtualServices define the same host - ${host}"
	vsHostMsg      = "The VirtualServices ${vs_names} in namespace(s) ${namespaces}" +
		" define the same host, ${host}. A host name can be defined by only one VirtualService." +
		" Consider updating the VirtualService(s) to have unique hostnames."
)

// VsHost implements Vetter interface
type VsHost struct {
	nsLister v1.NamespaceLister
	vsLister netv1alpha3.VirtualServiceLister
	cmLister v1.ConfigMapLister
}

// createVirtualServiceNotes checks for multiple vs defining the same host and
// generates notes for these cases
func createVirtualServiceNotes(virtualServices []*v1alpha3.VirtualService) ([]*apiv1.Note, error) {
	vsMap := make(map[string][]*v1alpha3.VirtualService)
	var hostname string
	var err error
	for _, vs := range virtualServices {
		for _, host := range vs.Spec.GetHosts() {
			hostname, err = util.ConvertHostnameToFQDN(host, vs.Namespace)
			if err != nil {
				fmt.Printf("Unable to convert hostname: %s\n", err.Error())
				return nil, err
			}
			if _, ok := vsMap[hostname]; !ok {
				vsMap[hostname] = []*v1alpha3.VirtualService{vs}
			} else {
				vsMap[hostname] = append(vsMap[hostname], vs)
			}
		}
	}
	// create vet notes
	notes := []*apiv1.Note{}
	for host, vsList := range vsMap {
		if len(vsList) > 1 {
			// there are multiple vs defining a host
			vsNames, vsNamespaces := []string{}, []string{}
			for _, vs := range vsList {
				vsNames = append(vsNames, vs.Name)
				vsNamespaces = append(vsNamespaces, vs.Namespace)
			}
			notes = append(notes, &apiv1.Note{
				Type:    vsHostNoteType,
				Summary: vsHostSummary,
				Msg:     vsHostMsg,
				Level:   apiv1.NoteLevel_ERROR,
				Attr: map[string]string{
					"host":       host,
					"vs_names":   strings.Join(vsNames, ", "),
					"namespaces": strings.Join(vsNamespaces, ", ")}})
		}
	}
	for i := range notes {
		notes[i].Id = util.ComputeID(notes[i])
	}
	return notes, nil
}

// Vet returns the list of generated notes
func (v *VsHost) Vet() ([]*apiv1.Note, error) {
	virtualServices, err := util.ListVirtualServicesInMesh(v.nsLister, v.cmLister, v.vsLister)
	if err != nil {
		fmt.Printf("Error occurred retrieving VirtualServices: %s\n", err.Error())
		return nil, err
	}
	notes, err := createVirtualServiceNotes(virtualServices)
	if err != nil {
		fmt.Printf("Error creating Conflicting VirtualService notes: %s\n", err.Error())
		return nil, err
	}
	return notes, nil
}

// Info returns information about the vetter
func (v *VsHost) Info() *apiv1.Info {
	return &apiv1.Info{Id: vetterID, Version: "0.1.0"}
}

// NewVetter returns "VsHost" which implements the Vetter Tnterface
func NewVetter(factory vetter.ResourceListGetter) *VsHost {
	return &VsHost{
		nsLister: factory.K8s().Core().V1().Namespaces().Lister(),
		cmLister: factory.K8s().Core().V1().ConfigMaps().Lister(),
		vsLister: factory.Istio().Networking().V1alpha3().VirtualServices().Lister(),
	}
}
