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

package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "github.com/AlexsJones/hello-operator/api/v1"
)

// EmitterReconciler reconciles a Emitter object
type EmitterReconciler struct {
	client.Client
	Log    logr.Logger
	CurrentEmitterDeployments map[string]string
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.hello.operator.com,resources=emitters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.hello.operator.com,resources=emitters/status,verbs=get;update;patch

func (r *EmitterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("emitter", req.NamespacedName)

	// your logic here
	var emitter webappv1.Emitter
	if err := r.Get(ctx, req.NamespacedName, &emitter); err == nil {
		pName := emitter.Spec.PairName
		deploymentName := fmt.Sprintf("emitter-%s", pName)
		r.CurrentEmitterDeployments[req.Name] = deploymentName
		if pName == "" {
			r.Log.Info("no emitter pair name found skipping")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}


		// Check if the emitter has an existing deployment-pair
		var deployment v1.Deployment

		if err := r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace,
			Name:      deploymentName,
		}, &deployment); err != nil {
			r.Log.Info("deployment not found")
			statusError := err.(*errors.StatusError)
			if statusError.Status().Code == 404 {
				// Create the deployment

				file, fErr := ioutil.ReadFile("manifests/emitter-deployment.yaml")
				if fErr != nil {
					r.Log.Error(fErr, "unable to create deployment")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				decoder := scheme.Codecs.UniversalDecoder()
				_, _, err = decoder.Decode(file, nil, &deployment)

				if err != nil {
					r.Log.Error(err,fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				deployment.Namespace = req.Namespace
				deployment.Name = deploymentName
				deployment.Annotations = map[string]string{
					"generated" : "hello-operator",
				}

				err = r.Client.Create(ctx, &deployment)
				if err != nil {
					r.Log.Error(err, "unable to fetch emitter")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

			}
			return ctrl.Result{}, client.IgnoreNotFound(err)

		}
	} else {
		r.Log.Info("unable to fetch emitter, checking for dangling deployments...")
		if r.CurrentEmitterDeployments[req.Name] != "" {

			r.Log.Info("deleting existing deployment...")
			var deployment v1.Deployment
			if e := r.Get(ctx, types.NamespacedName{
				Namespace: req.Namespace,
				Name:      r.CurrentEmitterDeployments[req.Name],
			}, &deployment); e == nil {

				if err := r.Client.Delete(ctx,&deployment); err != nil {
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func (r *EmitterReconciler) SetupWithManager(mgr ctrl.Manager) error {

	r.CurrentEmitterDeployments = make(map[string]string)
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.Emitter{}).
		Complete(r)
}
