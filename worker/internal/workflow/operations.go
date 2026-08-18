package workflow

import (
	"context"
	"fmt"
)

func Run(
	ctx context.Context,
	operation string,
	failureMode string,
	attempt int,
) error {

	switch operation {

	case "discover":
		return Discover(ctx)

	case "validate":
		return Validate(ctx, failureMode, attempt)

	case "publish":
		return Publish(ctx, failureMode, attempt)

	case "promote":
		return Promote(ctx)

	case "transfer":
		return Transfer(ctx)

	case "region-expand":
		return RegionExpand(ctx)

	default:
		return NewPermanentError(
			"UNKNOWN_OPERATION",
			fmt.Sprintf("unsupported operation: %s", operation),
			nil,
		)
	}
}

func Discover(ctx context.Context) error {

	fmt.Println("=================================")
	fmt.Println("Marketplace Discover")
	fmt.Println("=================================")

	fmt.Println("fetching pending Marketplace listings...")

	// Later:
	//
	// listings, err :=
	//     publishingClient.GetPendingListings(ctx)
	//
	// Object Storage / DB / Publishing API

	fmt.Println("found Marketplace listings")
	fmt.Println("discovery completed successfully")

	return nil
}

func Publish(
	ctx context.Context,
	failureMode string,
	attempt int,
) error {

	fmt.Println("=================================")
	fmt.Println("Marketplace Publish")
	fmt.Printf("attempt=%d\n", attempt)
	fmt.Println("=================================")

	// Later:
	//
	// err :=
	//     publishingClient.PublishListing(ctx, listingID)

	switch failureMode {

	case "publish-transient":

		if attempt < 2 {
			return NewTransientError(
				"PUBLISHING_SERVICE_UNAVAILABLE",
				"publishing service temporarily unavailable",
				nil,
			)
		}

		fmt.Println("publishing service recovered")

	case "publish-permanent":

		return NewPermanentError(
			"INVALID_PUBLISH_STATE",
			"listing cannot transition to Published state",
			nil,
		)
	}

	fmt.Println("publishing artifact...")
	fmt.Println("publishing listing...")
	fmt.Println("publishing completed successfully")

	return nil
}

func Promote(ctx context.Context) error {

	fmt.Println("=================================")
	fmt.Println("Marketplace Promotion")
	fmt.Println("=================================")

	fmt.Println("finding listings eligible for promotion...")
	fmt.Println("validating promotion...")
	fmt.Println("promoting listing...")
	fmt.Println("promotion completed successfully")

	return nil
}

func Transfer(ctx context.Context) error {

	fmt.Println("=================================")
	fmt.Println("Marketplace Transfer")
	fmt.Println("=================================")

	fmt.Println("validating source region...")
	fmt.Println("validating target region...")
	fmt.Println("transferring artifacts...")
	fmt.Println("transfer completed successfully")

	return nil
}

func RegionExpand(ctx context.Context) error {

	fmt.Println("=================================")
	fmt.Println("Marketplace Region Expansion")
	fmt.Println("=================================")

	fmt.Println("discovering target region...")
	fmt.Println("validating dependencies...")
	fmt.Println("expanding Marketplace resources...")
	fmt.Println("region expansion completed successfully")

	return nil
}
