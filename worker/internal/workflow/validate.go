package workflow

import (
	"context"
	"fmt"
)

func Validate(
	ctx context.Context,
	failureMode string,
	attempt int,
) error {

	fmt.Printf(
		"starting marketplace validation: mode=%s attempt=%d\n",
		failureMode,
		attempt,
	)

	switch failureMode {

	case "success":

		fmt.Println("validation succeeded")
		return nil

	case "transient":

		if attempt < 2 {
			return NewTransientError(
				"OCI_SERVICE_UNAVAILABLE",
				"OCI API temporarily unavailable",
				nil,
			)
		}

		fmt.Println("OCI API recovered")
		fmt.Println("validation succeeded")

		return nil

	case "permanent":

		return NewPermanentError(
			"INVALID_LISTING_STATE",
			"listing cannot be published in its current state",
			nil,
		)

	case "invalid-image":

		return NewValidationError(
			"INVALID_IMAGE",
			"Marketplace image validation failed",
			nil,
		)

	default:

		return NewPermanentError(
			"UNKNOWN_FAILURE_MODE",
			fmt.Sprintf("unknown failure mode: %s", failureMode),
			nil,
		)
	}
}
