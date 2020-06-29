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

package main

import (
	"flag"
	"os"

	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"code.cloudfoundry.org/capi-k8s-release/src/cf-api-kpack-watcher/capi"
	"code.cloudfoundry.org/capi-k8s-release/src/cf-api-kpack-watcher/controllers"
	"code.cloudfoundry.org/capi-k8s-release/src/cf-api-kpack-watcher/image_registry"
	buildpivotaliov1alpha1 "github.com/pivotal/kpack/pkg/client/clientset/versioned/scheme"
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds"
	// +kubebuilder:scaffold:imports
)

var (
	// scheme   = runtime.NewScheme()
	scheme   = buildpivotaliov1alpha1.Scheme
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = buildpivotaliov1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	// TODO: add some startup validation for necessary config to interact with CCNG (e.g. its domain)

	// TODO: change this to somehow use `lager` for consistency?
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "7cba68d7.build.pivotal.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	client := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	keychainFactory, err := k8sdockercreds.NewSecretKeychainFactory(client)
	if err != nil {
		panic(err.Error())
	}

	if err = (&controllers.BuildReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Build"),
		Scheme: mgr.GetScheme(),
		// TODO: use `capi.NewCFAPIClient()` instead
		CFAPIClient:        capi.NewCAPIClient(),
		ImageConfigFetcher: image_registry.NewImageConfigFetcher(keychainFactory),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Build")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
