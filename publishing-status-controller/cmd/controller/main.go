package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	wfclientset "github.com/argoproj/argo-workflows/v4/pkg/client/clientset/versioned"
	wfinformers "github.com/argoproj/argo-workflows/v4/pkg/client/informers/externalversions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"net/http"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/controller"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/publishing"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/metrics"
)

func main() {
	namespace := flag.String("namespace", "argo", "namespace containing Argo Workflos")
	kubeconfig := flag.String(
		"kubeconfig",
		"",
		"path to kubeconfig",
	)
	publishingURL :=
		flag.String(
			"publishing-url",
			"http://localhost:8080",
			"Marketplace Publishing REST API base URL",
		)

	httpTimeout :=
		flag.Duration(
			"http-timeout",
			3*time.Second,
			"Marketplace Publishing HTTP timeout",
		)

	metricsAddress :=
		flag.String(
			"metrics-address",
			":9090",
			"Prometheus metrics listen address",
		)

	flag.Parse()
	metricsRecorder :=
		metrics.NewRecorder()

	publishingClient, err :=
		publishing.NewClient(
			*publishingURL,
			*httpTimeout,
			metricsRecorder,
		)

	if err != nil {

		fmt.Fprintf(
			os.Stderr,
			"failed to create publishing client: %v\n",
			err,
		)

		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()
	config, err :=
		clientcmd.BuildConfigFromFlags(
			"",
			*kubeconfig,
		)

	config, err = buildKubeConfig(
		*kubeconfig,
	)

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to create Kubernetes config: %v\n",
			err,
		)

		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to load kubeconfig: %v\n",
			err,
		)

		os.Exit(1)
	}

	argoClient, err :=
		wfclientset.NewForConfig(config)

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to create Argo client: %v\n",
			err,
		)

		os.Exit(1)
	}
	metricsMux :=
		http.NewServeMux()

	metricsMux.Handle(
		"/metrics",
		metricsRecorder.Handler(),
	)

	metricsServer :=
		&http.Server{
			Addr: *metricsAddress,

			Handler: metricsMux,
		}

	go func() {

		fmt.Printf(
			"Prometheus metrics listening on %s/metrics\n",
			*metricsAddress,
		)

		err :=
			metricsServer.
				ListenAndServe()

		if err != nil &&
			err != http.ErrServerClosed {

			fmt.Fprintf(
				os.Stderr,
				"metrics server failed: %v\n",
				err,
			)

			cancel()
		}
	}()
	// Watch only the Argo namespace.
	informerFactory :=
		wfinformers.NewSharedInformerFactoryWithOptions(
			argoClient,
			0*time.Second,
			wfinformers.WithNamespace(*namespace),
		)

	workflowInformer :=
		informerFactory.
			Argoproj().
			V1alpha1().
			Workflows().
			Informer()

	statusController, err :=
		controller.New(
			workflowInformer,
			publishingClient,
			metricsRecorder,
		)

	if err != nil {

		fmt.Fprintf(
			os.Stderr,
			"failed to create controller: %v\n",
			err,
		)

		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to create controller: %v\n",
			err,
		)

		os.Exit(1)
	}
	fmt.Printf(
		"Starting Marketplace Publishing Status Controller namespace=%s\n",
		*namespace,
	)

	informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(
		ctx.Done(),
		workflowInformer.HasSynced,
	) {
		fmt.Fprintln(
			os.Stderr,
			"failed to synchronize workflow informer cache",
		)

		os.Exit(1)
	}

	fmt.Println(
		"Workflow informer cache synchronized",
	)

	statusController.Run(
		ctx,
		2,
	)

	//<-ctx.Done()

	fmt.Println(
		"Stopping Marketplace Publishing Status Controller",
	)
}

func defaultKubeconfig() string {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return ""
	}

	return filepath.Join(
		homeDir,
		".kube",
		"config",
	)
}

func buildKubeConfig(
	kubeconfig string,
) (*rest.Config, error) {

	// Explicit kubeconfig always wins.
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(
			"",
			kubeconfig,
		)
	}

	// Running inside Kubernetes.
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return rest.InClusterConfig()
	}

	// Local development fallback.
	return clientcmd.BuildConfigFromFlags(
		"",
		defaultKubeconfig(),
	)
}
