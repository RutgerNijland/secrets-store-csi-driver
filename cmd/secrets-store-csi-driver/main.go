/*
Copyright 2018 The Kubernetes Authors.

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

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	// +kubebuilder:scaffold:imports
)

var (
	endpoint           = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName         = flag.String("drivername", "secrets-store.csi.k8s.io", "name of the driver")
	nodeID             = flag.String("nodeid", "", "node id")
	debug              = flag.Bool("debug", false, "sets log to debug level")
	logFormatJSON      = flag.Bool("log-format-json", false, "set log formatter to json")
	logReportCaller    = flag.Bool("log-report-caller", false, "include the calling method as fields in the log")
	providerVolumePath = flag.String("provider-volume", "/etc/kubernetes/secrets-store-csi-providers", "Volume path for provider")
	minProviderVersion = flag.String("min-provider-version", "", "set minimum supported provider versions with current driver")
	metricsAddr        = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	// grpcSupportedProviders is a ; separated string that can contain a list of providers. The reason it's a string is to allow scenarios
	// where the driver is being used with 2 providers, one which supports grpc and other using binary for provider.
	grpcSupportedProviders = flag.String("grpc-supported-providers", "", "set list of providers that support grpc for driver-provider [alpha]")

	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	flag.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	if *logFormatJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.SetReportCaller(*logReportCaller)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		log.Fatalf("failed to start manager, error: %+v", err)
	}

	if err = (&controllers.SecretProviderClassPodStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		NodeID: *nodeID,
		Reader: mgr.GetCache(),
		Writer: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		log.Fatalf("failed to create controller, error: %+v", err)
	}
	// +kubebuilder:scaffold:builder

	go func() {
		log.Infof("starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Fatalf("failed to run manager, eror: %+v", err)
		}
	}()

	handle()
}

func handle() {
	driver := secretsstore.GetDriver()
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("failed to initialize driver, error getting config: %+v", err)
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: nil})
	if err != nil {
		log.Fatalf("failed to initialize driver, error creating client: %+v", err)
	}
	driver.Run(*driverName, *nodeID, *endpoint, *providerVolumePath, *minProviderVersion, *grpcSupportedProviders, c)
}
