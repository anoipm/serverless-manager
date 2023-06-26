package state

import (
	"context"
	"time"

	"github.com/kyma-project/serverless-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	requeueDuration = time.Second * 3
)

func sFnUpdateProcessingState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateProcessing)
		s.instance.UpdateConditionUnknown(condition, reason, msg)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(sFnRequeue(), nil, nil), nil, err
	}
}

func sFnUpdateProcessingTrueState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateProcessing)
		s.instance.UpdateConditionTrue(condition, reason, msg)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(sFnRequeue(), nil, nil), nil, err
	}
}

func sFnUpdateReadyState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateReady)
		s.instance.UpdateConditionTrue(condition, reason, msg)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(sFnStop(), nil, nil), nil, err
	}
}

func sFnUpdateErrorState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, err error) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateError)
		s.instance.UpdateConditionFalse(condition, reason, err)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(nil, nil, err), nil, err
	}
}

func sFnUpdateDeletingState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateDeleting)
		s.instance.UpdateConditionUnknown(condition, reason, msg)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(sFnRequeue(), nil, nil), nil, err
	}
}

func sFnUpdateDeletingTrueState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateDeleting)
		s.instance.UpdateConditionTrue(condition, reason, msg)

		err := updateServerlessStatus(ctx, r, s)
		return buildSFnEmitEvent(sFnRequeue(), nil, nil), nil, err
	}
}

func sFnUpdateServerless() stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		return stopWithError(r.client.Update(ctx, &s.instance))
	}
}

func sFnUpdateServedTrue() stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setServed(v1alpha1.ServedTrue)
		err := updateServerlessStatus(ctx, r, s)
		return sFnRequeue(), nil, err
	}
}

func sFnUpdateWarningState(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, msg string) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setState(v1alpha1.StateWarning)
		s.instance.UpdateConditionTrue(condition, reason, msg)

		return updateServerlessStatus(buildSFnEmitEvent(sFnStop(), nil, nil), ctx, r, s)
	}

}

func sFnUpdateServedFalse(condition v1alpha1.ConditionType, reason v1alpha1.ConditionReason, err error) stateFn {
	return func(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
		s.setServed(v1alpha1.ServedFalse)
		return nextState(sFnUpdateErrorState(condition, reason, err))
	}
}

func updateServerlessStatus(ctx context.Context, r *reconciler, s *systemState) error {
	instance := s.instance.DeepCopy()
	r.client.Status().Update(ctx, instance)
}
