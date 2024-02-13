package nextroute

import (
	"github.com/nextmv-io/sdk/common"
	"github.com/nextmv-io/sdk/nextroute"
)

// NewAttributesConstraint returns a new AttributesConstraint.
func NewAttributesConstraint() (nextroute.AttributesConstraint, error) {
	return &attributesConstraintImpl{
		modelConstraintImpl: newModelConstraintImpl(
			"attributes",
			nextroute.ModelExpressions{},
		),
		stopAttributes:        make(map[int][]string),
		vehicleTypeAttributes: make(map[int][]string),
	}, nil
}

type attributesConstraintImpl struct {
	stopAttributes        map[int][]string
	vehicleTypeAttributes map[int][]string
	modelConstraintImpl
	compatible   []bool
	vehicleTypes int
}

func (l *attributesConstraintImpl) Lock(model nextroute.Model) error {
	vehicleTypeAttributes := make(map[int]map[string]bool)
	vehicleTypes := model.VehicleTypes()
	l.vehicleTypes = len(vehicleTypes)
	modelImpl := model.(*modelImpl) // we assume that the model is a modelImpl
	for _, vehicleType := range vehicleTypes {
		vehicleTypeAttributes[vehicleType.Index()] = make(map[string]bool)
		for _, attribute := range l.vehicleTypeAttributes[vehicleType.Index()] {
			vehicleTypeAttributes[vehicleType.Index()][attribute] = true
		}
	}

	// Determine which stops are individually compatible with which vehicle
	// types.
	stopVehicleCompatible := make([]bool, model.NumberOfStops()*len(vehicleTypes))
	for _, stop := range modelImpl.stops {
		for _, vehicleType := range vehicleTypes {
			idx := l.mapTwoIndices(stop.Index(), vehicleType.Index())
			stopVehicleCompatible[idx] = len(l.stopAttributes[stop.Index()]) == 0
			for _, stopAttribute := range l.stopAttributes[stop.Index()] {
				if _, ok := vehicleTypeAttributes[vehicleType.Index()][stopAttribute]; ok {
					stopVehicleCompatible[idx] = true
					break
				}
			}
		}
	}

	// Determine which plan unit is compatible with which vehicle type by
	// gathering all the stops in the plan unit and checking if they are
	// compatible with the vehicle type.
	l.compatible = make([]bool, len(modelImpl.planUnits)*len(vehicleTypes))
	for _, planUnit := range model.PlanStopsUnits() {
		stops := planUnit.Stops()
		for _, vehicleType := range vehicleTypes {
			compatible := true
			for _, stop := range stops {
				idx := l.mapTwoIndices(stop.Index(), vehicleType.Index())
				compatible = compatible && stopVehicleCompatible[idx]
			}
			idx := l.mapTwoIndices(planUnit.Index(), vehicleType.Index())
			l.compatible[idx] = compatible
		}
	}

	return nil
}

func (l *attributesConstraintImpl) String() string {
	return l.name
}

func (l *attributesConstraintImpl) StopAttributes(stop nextroute.ModelStop) []string {
	if attributes, hasAttributes := l.stopAttributes[stop.Index()]; hasAttributes {
		return common.DefensiveCopy(attributes)
	}
	return []string{}
}

func (l *attributesConstraintImpl) VehicleTypeAttributes(vehicle nextroute.ModelVehicleType) []string {
	if attributes, hasAttributes := l.vehicleTypeAttributes[vehicle.Index()]; hasAttributes {
		return common.DefensiveCopy(attributes)
	}
	return []string{}
}

func (l *attributesConstraintImpl) SetStopAttributes(
	stop nextroute.ModelStop,
	stopAttributes []string,
) {
	if stop.Model().IsLocked() {
		panic("cannot set stop attributes after model is locked")
	}
	l.stopAttributes[stop.Index()] = common.Unique(stopAttributes)
}

func (l *attributesConstraintImpl) SetVehicleTypeAttributes(
	vehicleType nextroute.ModelVehicleType,
	vehicleAttributes []string,
) {
	if vehicleType.Model().IsLocked() {
		panic("cannot set vehicle type attributes after model is locked")
	}
	l.vehicleTypeAttributes[vehicleType.Index()] = common.Unique(vehicleAttributes)
}

func (l *attributesConstraintImpl) CheckCost() nextroute.Cost {
	return nextroute.Constant
}

func (l *attributesConstraintImpl) EstimationCost() nextroute.Cost {
	return nextroute.Constant
}

func (l *attributesConstraintImpl) EstimateIsViolated(
	move nextroute.SolutionMoveStops,
) (isViolated bool, stopPositionsHint nextroute.StopPositionsHint) {
	moveImpl := move.(*solutionMoveStopsImpl)
	planUnitIdx := moveImpl.planUnit.modelPlanStopsUnit.Index()
	vehicleType := moveImpl.vehicle().ModelVehicle().VehicleType()
	idx := l.mapTwoIndices(planUnitIdx, vehicleType.Index())
	compatible := l.compatible[idx]
	if compatible {
		return false, constNoPositionsHint
	}
	return true, constSkipVehiclePositionsHint
}

func (l *attributesConstraintImpl) mapTwoIndices(i, j int) int {
	return i*l.vehicleTypes + j
}
