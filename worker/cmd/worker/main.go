package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/dipen1510/marketplace-workflow-platform/worker/internal/workflow"
)

const (
	ExitSuccess    = 0
	ExitTransient  = 10
	ExitPermanent  = 20
	ExitValidation = 30
)

func main() {

	var operation string
	var failureMode string
	var attempt int

	flag.StringVar(
		&operation,
		"operation",
		"",
		"Marketplace workflow operation",
	)

	flag.StringVar(
		&failureMode,
		"failure-mode",
		"success",
		"failure simulation mode",
	)

	flag.IntVar(
		&attempt,
		"attempt",
		0,
		"workflow retry attempt",
	)

	flag.Parse()

	if operation == "" {
		fmt.Println("operation is required")
		os.Exit(ExitPermanent)
	}

	fmt.Println("=================================")
	fmt.Println("Marketplace Workflow Worker")
	fmt.Printf("operation=%s\n", operation)
	fmt.Printf("failureMode=%s\n", failureMode)
	fmt.Printf("attempt=%d\n", attempt)
	fmt.Println("=================================")

	ctx := context.Background()

	err := workflow.Run(
		ctx,
		operation,
		failureMode,
		attempt,
	)

	if err == nil {

		fmt.Printf(
			"operation=%s completed successfully\n",
			operation,
		)

		os.Exit(ExitSuccess)
	}

	handleError(err)
}

func handleError(err error) {

	fmt.Printf("worker failed: %v\n", err)

	var workflowErr *workflow.WorkflowError

	if !errors.As(err, &workflowErr) {

		fmt.Println(
			"unclassified error; treating as permanent failure",
		)

		os.Exit(ExitPermanent)
	}

	switch workflowErr.Type {

	case workflow.ErrorTransient:

		fmt.Printf(
			"classification=TRANSIENT exitCode=%d\n",
			ExitTransient,
		)

		os.Exit(ExitTransient)

	case workflow.ErrorValidation:

		fmt.Printf(
			"classification=VALIDATION exitCode=%d\n",
			ExitValidation,
		)

		os.Exit(ExitValidation)

	case workflow.ErrorPermanent:

		fmt.Printf(
			"classification=PERMANENT exitCode=%d\n",
			ExitPermanent,
		)

		os.Exit(ExitPermanent)

	default:

		fmt.Println("unknown error classification")

		os.Exit(ExitPermanent)
	}
}
