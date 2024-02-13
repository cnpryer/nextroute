package factory

import (
	"fmt"

	"github.com/nextmv-io/nextroute"
	"github.com/nextmv-io/sdk/common"
	"github.com/nextmv-io/sdk/measure"
	sdkNextRoute "github.com/nextmv-io/sdk/nextroute"
	"github.com/nextmv-io/sdk/nextroute/factory"
	"github.com/nextmv-io/sdk/nextroute/schema"
)

// addVehicles adds the vehicle types to the Model.
func addVehicles(
	input schema.Input,
	model sdkNextRoute.Model,
	options factory.Options,
) (sdkNextRoute.Model, error) {
	data, err := getModelData(model)
	if err != nil {
		return nil, err
	}

	travelDuration := travelDurationExpression(input)
	durationGroupsExpression := NewDurationGroupsExpression(model.NumberOfStops(), len(input.Vehicles))
	distanceExpression := distanceExpression(input.DistanceMatrix)

	inputVehicleHasAlternateStops := false

	constraint, err := nextroute.NewAttributesConstraint()

	if err != nil {
		return nil, err
	}

	for idx, inputVehicle := range input.Vehicles {
		vehicleType, err := newVehicleType(
			inputVehicle,
			model,
			distanceExpression,
			travelDuration,
			durationGroupsExpression,
		)
		if err != nil {
			return nil, err
		}

		vehicle, err := newVehicle(inputVehicle, vehicleType, model, options)
		if err != nil {
			return nil, err
		}

		if inputVehicle.AlternateStops != nil {
			inputVehicleHasAlternateStops = true
			vehicle.First().SetMeasureIndex(len(input.Stops) + len(*input.AlternateStops) + idx*2)
			vehicle.Last().SetMeasureIndex(len(input.Stops) + len(*input.AlternateStops) + idx*2 + 1)

			constraint.SetVehicleTypeAttributes(
				vehicleType,
				[]string{alternateVehicleAttribute(idx)},
			)
			for _, alternateID := range *inputVehicle.AlternateStops {
				alternateStop, err := model.Stop(data.stopIDToIndex[alternateStopID(alternateID, inputVehicle)])
				if err != nil {
					return nil, err
				}
				constraint.SetStopAttributes(alternateStop, []string{alternateVehicleAttribute(idx)})
			}
		}
	}

	if inputVehicleHasAlternateStops {
		err = model.AddConstraint(constraint)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// newVehicleType returns the VehicleType that the Model needs.
func newVehicleType(
	vehicle schema.Vehicle,
	model sdkNextRoute.Model,
	distanceExpression sdkNextRoute.DistanceExpression,
	durationExpression sdkNextRoute.DurationExpression,
	durationGroupsExpression DurationGroupsExpression,
) (sdkNextRoute.ModelVehicleType, error) {
	if durationExpression == nil {
		s := common.NewSpeed(*vehicle.Speed, common.MetersPerSecond)
		durationExpression = nextroute.NewTravelDurationExpression(distanceExpression, s)
		durationExpression.SetName(fmt.Sprintf(
			"travelDuration(%s,%s,%s)",
			vehicle.ID,
			distanceExpression.Name(),
			s,
		))
	}

	vehicleType, err := model.NewVehicleType(
		nextroute.NewTimeIndependentDurationExpression(durationExpression),
		durationGroupsExpression,
	)
	if err != nil {
		return nil, err
	}

	vehicleType.SetID(vehicle.ID)
	vehicleType.SetData(vehicleTypeData{
		DistanceExpression: distanceExpression,
	})

	return vehicleType, nil
}

func newVehicle(
	inputVehicle schema.Vehicle,
	vehicleType sdkNextRoute.ModelVehicleType,
	model sdkNextRoute.Model,
	options factory.Options,
) (sdkNextRoute.ModelVehicle, error) {
	startLocation := common.NewInvalidLocation()
	var err error
	if inputVehicle.StartLocation != nil {
		startLocation, err = common.NewLocation(
			inputVehicle.StartLocation.Lon,
			inputVehicle.StartLocation.Lat,
		)
		if err != nil {
			return nil, err
		}
	}
	start, err := model.NewStop(startLocation)
	if err != nil {
		return nil, err
	}
	start.SetID(inputVehicle.ID + "-start")

	endLocation := common.NewInvalidLocation()
	if inputVehicle.EndLocation != nil {
		endLocation, err = common.NewLocation(
			inputVehicle.EndLocation.Lon,
			inputVehicle.EndLocation.Lat,
		)
		if err != nil {
			return nil, err
		}
	}
	end, err := model.NewStop(endLocation)
	if err != nil {
		return nil, err
	}
	end.SetID(inputVehicle.ID + "-end")

	startTime := model.Epoch()
	if !options.Constraints.Disable.VehicleStartTime && inputVehicle.StartTime != nil {
		startTime = *inputVehicle.StartTime
	}

	vehicle, err := model.NewVehicle(
		vehicleType,
		startTime,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}

	vehicle.SetID(inputVehicle.ID)
	vehicle.SetData(inputVehicle)

	return vehicle, nil
}

// travelDurationExpressions returns the expressions that define how vehicles
// travel from one stop to another and the time it takes them to process a stop
// (service it).
func travelDurationExpression(input schema.Input) sdkNextRoute.DurationExpression {
	var travelDuration sdkNextRoute.DurationExpression
	if input.DurationMatrix != nil {
		travelDuration = nextroute.NewDurationExpression(
			"travelDuration",
			nextroute.NewMeasureByIndexExpression(measure.Matrix(*input.DurationMatrix)),
			common.Second,
		)
	}

	return travelDuration
}

// distanceExpression creates a distance expression for later use.
func distanceExpression(distanceMatrix *[][]float64) sdkNextRoute.DistanceExpression {
	distanceExpression := nextroute.NewHaversineExpression()
	if distanceMatrix != nil {
		distanceExpression = nextroute.NewDistanceExpression(
			"travelDistance",
			nextroute.NewMeasureByIndexExpression(measure.Matrix(*distanceMatrix)),
			common.Meters,
		)
	}
	return distanceExpression
}
