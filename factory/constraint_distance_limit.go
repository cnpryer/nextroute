package factory

import (
	"fmt"
	"math"

	"github.com/nextmv-io/nextroute"
	"github.com/nextmv-io/sdk/common"
	sdkNextRoute "github.com/nextmv-io/sdk/nextroute"
	"github.com/nextmv-io/sdk/nextroute/factory"
	"github.com/nextmv-io/sdk/nextroute/schema"
)

// addDistanceLimitConstraint adds a distance limit for routes to the model.
func addDistanceLimitConstraint(
	input schema.Input,
	model sdkNextRoute.Model,
	_ factory.Options,
) (sdkNextRoute.Model, error) {
	composed := nextroute.NewComposedPerVehicleTypeExpression(
		nextroute.NewConstantExpression(
			"constant-route-distance",
			0,
		),
	)

	limit := nextroute.NewVehicleTypeDistanceExpression(
		"distanceLimit",
		common.NewDistance(math.MaxFloat64, common.Meters),
	)
	hasDistanceLimit := false
	for _, vehicleType := range model.VehicleTypes() {
		maxDistance := input.Vehicles[vehicleType.Index()].MaxDistance
		if maxDistance == nil {
			continue
		}

		hasDistanceLimit = true

		// Check if custom data is set properly.
		data, ok := vehicleType.Data().(vehicleTypeData)
		if !ok {
			return nil, fmt.Errorf(
				fmt.Sprintf("could not read custom data for vehicle %s",
					vehicleType.ID(),
				),
			)
		}

		// Get distance expression and set limit for the vehicle type.
		distanceExpression := data.DistanceExpression
		composed.Set(vehicleType, distanceExpression)
		limit.SetDistance(vehicleType, common.NewDistance(float64(*maxDistance), common.Meters))
	}

	if !hasDistanceLimit {
		return model, nil
	}

	// Create and then add constraint to model.
	maxConstraint, err := nextroute.NewMaximum(
		composed,
		limit,
	)
	if err != nil {
		return nil, err
	}
	maxConstraint.(sdkNextRoute.Identifier).SetID("distance_limit")

	err = model.AddConstraint(maxConstraint)
	if err != nil {
		return nil, err
	}

	return model, nil
}
