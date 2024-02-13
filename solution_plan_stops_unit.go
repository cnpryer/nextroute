package nextroute

import (
	"context"
	"fmt"
	"sync"

	"github.com/nextmv-io/sdk/common"
	"github.com/nextmv-io/sdk/nextroute"
)

type solutionPlanStopsUnitImpl struct {
	modelPlanStopsUnit nextroute.ModelPlanStopsUnit
	solutionStops      []solutionStopImpl
}

func (p *solutionPlanStopsUnitImpl) String() string {
	return fmt.Sprintf("solutionPlanStopsUnit{%v, planned=%v}",
		p.modelPlanStopsUnit,
		p.IsPlanned(),
	)
}

func (p *solutionPlanStopsUnitImpl) SolutionStop(stop nextroute.ModelStop) nextroute.SolutionStop {
	return p.solutionStop(stop)
}

func (p *solutionPlanStopsUnitImpl) solutionStop(stop nextroute.ModelStop) solutionStopImpl {
	for _, solutionStop := range p.solutionStops {
		if solutionStop.ModelStop().Index() == stop.Index() {
			return solutionStop
		}
	}
	panic(
		fmt.Errorf("solution stop for model stop %s [%v] not found in unit %v",
			stop.ID(),
			stop.Index(),
			p.modelPlanStopsUnit.Index(),
		),
	)
}

func (p *solutionPlanStopsUnitImpl) PlannedPlanStopsUnits() nextroute.SolutionPlanStopsUnits {
	if p.IsPlanned() {
		return nextroute.SolutionPlanStopsUnits{p}
	}
	return nextroute.SolutionPlanStopsUnits{}
}

func (p *solutionPlanStopsUnitImpl) ModelPlanUnit() nextroute.ModelPlanUnit {
	return p.modelPlanStopsUnit
}

func (p *solutionPlanStopsUnitImpl) ModelPlanStopsUnit() nextroute.ModelPlanStopsUnit {
	return p.modelPlanStopsUnit
}

func (p *solutionPlanStopsUnitImpl) Index() int {
	return p.modelPlanStopsUnit.Index()
}

func (p *solutionPlanStopsUnitImpl) Solution() nextroute.Solution {
	return p.solutionStops[0].Solution()
}

func (p *solutionPlanStopsUnitImpl) solution() *solutionImpl {
	return p.solutionStops[0].solution
}

func (p *solutionPlanStopsUnitImpl) Stops() nextroute.ModelStops {
	return p.modelPlanStopsUnit.Stops()
}

func (p *solutionPlanStopsUnitImpl) SolutionStops() nextroute.SolutionStops {
	solutionStops := make(nextroute.SolutionStops, len(p.solutionStops))
	for i, solutionStop := range p.solutionStops {
		solutionStops[i] = solutionStop
	}
	return solutionStops
}

func (p *solutionPlanStopsUnitImpl) solutionStopsImpl() []solutionStopImpl {
	solutionStops := make([]solutionStopImpl, len(p.solutionStops))
	copy(solutionStops, p.solutionStops)
	return solutionStops
}

func (p *solutionPlanStopsUnitImpl) IsPlanned() bool {
	if len(p.solutionStops) == 0 {
		return false
	}
	for _, solutionStop := range p.solutionStops {
		if !solutionStop.IsPlanned() {
			return false
		}
	}
	return true
}

func (p *solutionPlanStopsUnitImpl) IsFixed() bool {
	for _, solutionStop := range p.solutionStops {
		if solutionStop.ModelStop().IsFixed() {
			return true
		}
	}
	return false
}

func (p *solutionPlanStopsUnitImpl) UnPlan() (bool, error) {
	if !p.IsPlanned() || p.IsFixed() {
		return false, nil
	}

	solution := p.Solution().(*solutionImpl)

	solution.Model().OnUnPlan(p)

	if planUnitsUnit, isMemberOf := p.modelPlanStopsUnit.PlanUnitsUnit(); isMemberOf {
		solutionPlanUnitsUnit := solution.SolutionPlanUnit(planUnitsUnit)
		solution.plannedPlanUnits.remove(solutionPlanUnitsUnit)
		solution.unPlannedPlanUnits.add(solutionPlanUnitsUnit)
	} else {
		solution.plannedPlanUnits.remove(p)
		solution.unPlannedPlanUnits.add(p)
	}

	success, err := p.unplan()
	if err != nil {
		success = false
	}

	if success {
		solution.Model().OnUnPlanSucceeded(p)
	} else {
		if planUnitsUnit, isMemberOf := p.modelPlanStopsUnit.PlanUnitsUnit(); isMemberOf {
			solutionPlanUnitsUnit := solution.SolutionPlanUnit(planUnitsUnit)
			solution.unPlannedPlanUnits.remove(solutionPlanUnitsUnit)
			solution.plannedPlanUnits.add(solutionPlanUnitsUnit)
		} else {
			solution.unPlannedPlanUnits.remove(p)
			solution.plannedPlanUnits.add(p)
		}
		solution.Model().OnUnPlanFailed(p)
	}
	return success, err
}

func (p *solutionPlanStopsUnitImpl) StopPositions() nextroute.StopPositions {
	if p.IsPlanned() {
		return common.Map(p.solutionStops, func(solutionStop solutionStopImpl) nextroute.StopPosition {
			return newStopPosition(
				solutionStop.previous(),
				solutionStop,
				solutionStop.next(),
			)
		})
	}
	return nextroute.StopPositions{}
}

var unplanSolutionMove = sync.Pool{
	New: func() any {
		return &solutionMoveStopsImpl{
			stopPositions: make([]stopPositionImpl, 0, 64),
		}
	},
}

func (p *solutionPlanStopsUnitImpl) unplan() (bool, error) {
	solution := p.solutionStops[0].Solution().(*solutionImpl)

	// TODO: solutionStop.detach() modifies the solution so we have to
	// create the move here, even though we don't need it only if
	// isFeasible() returns a constraint.
	move := unplanSolutionMove.Get().(*solutionMoveStopsImpl)
	defer func() {
		move.stopPositions = move.stopPositions[:0]
		unplanSolutionMove.Put(move)
	}()
	move.planUnit = p
	move.value = 0.0
	move.valueSeen = 0
	move.allowed = true
	for _, solutionStop := range p.solutionStops {
		move.stopPositions = append(move.stopPositions, newStopPosition(
			solutionStop.previous(),
			solutionStop,
			solutionStop.next(),
		))
	}

	idx := p.solutionStops[0].PreviousIndex()
	for _, solutionStop := range p.solutionStops {
		solutionStop.detach()
	}

	constraint, _, err := solution.isFeasible(idx, true)
	if err != nil {
		return false, err
	}
	if constraint != nil {
		planned, err := move.Execute(context.Background())
		if err != nil {
			return false, err
		}
		if !planned {
			return false,
				fmt.Errorf(
					"failed undoing failed unplan",
				)
		}
	}
	return true, nil
}
