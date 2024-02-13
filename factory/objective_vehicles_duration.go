package factory

import (
	"github.com/nextmv-io/nextroute"
	sdkNextRoute "github.com/nextmv-io/sdk/nextroute"
	"github.com/nextmv-io/sdk/nextroute/factory"
	"github.com/nextmv-io/sdk/nextroute/schema"
)

// addVehiclesDurationObjective adds the minimization of the sum of vehicles
// duration to the model.
func addVehiclesDurationObjective(
	_ schema.Input,
	model sdkNextRoute.Model,
	options factory.Options,
) (sdkNextRoute.Model, error) {
	o := nextroute.NewVehiclesDurationObjective()
	_, err := model.Objective().NewTerm(options.Objectives.VehiclesDuration, o)
	if err != nil {
		return nil, err
	}

	return model, nil
}
