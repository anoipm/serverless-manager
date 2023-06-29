package state

import (
	"context"
	"github.com/kyma-project/serverless-manager/api/v1alpha1"
	"github.com/kyma-project/serverless-manager/internal/chart"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// run serverless chart installation
func sFnApplyResources(ctx context.Context, r *reconciler, s *systemState) (stateFn, *ctrl.Result, error) {
	// check if condition exists
	if !s.instance.IsCondition(v1alpha1.ConditionTypeInstalled) {
		s.setState(v1alpha1.StateProcessing)
		s.instance.UpdateConditionUnknown(v1alpha1.ConditionTypeInstalled, v1alpha1.ConditionReasonInstallation,
			"Installing for configuration")

		return nextState(sFnUpdateStatusAndRequeue)
	}

	// install component
	err := chart.Install(s.chartConfig)
	if err != nil {
		r.log.Warnf("error while installing resource %s: %s",
			client.ObjectKeyFromObject(&s.instance), err.Error())
		setErrorState(s,
			v1alpha1.ConditionTypeInstalled,
			v1alpha1.ConditionReasonInstallationErr,
			err,
		)
		return nextState(sFnUpdateStatusAndStop)
	}

	// switch state verify
	return nextState(sFnVerifyResources)
}
