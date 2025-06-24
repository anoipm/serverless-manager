package state

import (
	"context"
	serverlessmetrics "github.com/kyma-project/serverless/components/buildless-serverless/internal/controller/metrics"
	"github.com/kyma-project/serverless/components/buildless-serverless/internal/controller/validator"
	"strings"

	"github.com/kyma-project/serverless/components/buildless-serverless/internal/controller/fsm"

	serverlessv1alpha2 "github.com/kyma-project/serverless/components/buildless-serverless/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func sFnValidateFunction(_ context.Context, m *fsm.StateMachine) (fsm.StateFn, *ctrl.Result, error) {
	serverlessmetrics.ResourceProcessedTotal.WithLabelValues(string(m.State.Function.Spec.Runtime)).Inc()
	v := validator.New(&m.State.Function, m.FunctionConfig)
	validationResults := v.Validate()
	if len(validationResults) != 0 {
		m.State.Function.UpdateCondition(
			serverlessv1alpha2.ConditionConfigurationReady,
			metav1.ConditionFalse,
			serverlessv1alpha2.ConditionReasonInvalidFunctionSpec,
			strings.Join(validationResults, ". "))
		return stop()
	}

	return nextState(sFnHandleGitSources)
}
