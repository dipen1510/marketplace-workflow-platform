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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/controller"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/publishing"
)

func main() {
	namespace := flag.String("namespace", "argo", "namespace containing Argo Workflos")
	kubeconfig := flag.String(
		"kubeconfig",
		defaultKubeconfig(),
		"path to kubeconfig",
	)
	publishingAddress :=
		flag.String(
			"publishing-address",
			"localhost:50051",
			"Marketplace Publishing gRPC address",
		)

	rpcTimeout :=
		flag.Duration(
			"rpc-timeout",
			3*time.Second,
			"Marketplace Publishing RPC timeout",
		)

	flag.Parse()

	publishingClient, err :=
		publishing.NewClient(
			*publishingAddress,
			*rpcTimeout,
		)

	if err != nil {

		fmt.Fprintf(
			os.Stderr,
			"failed to create publishing client: %v\n",
			err,
		)

		os.Exit(1)
	}

	defer publishingClient.Close()

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
