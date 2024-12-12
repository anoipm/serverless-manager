package state

import (
	"context"
	"fmt"
	serverlessv1alpha2 "github.com/kyma-project/serverless/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

//TODO: Add states:
// - validate - components/serverless/internal/controllers/serverless/validation.go
// - gitSources - stateFnGitCheckSources

func sFnHandleDeployment(ctx context.Context, m *stateMachine) (stateFn, *ctrl.Result, error) {
	builtDeployment := NewDeploymentBuilder(m).build()

	clusterDeployment, resultGet, errGet := m.getOrCreateDeployment(ctx, builtDeployment)
	if clusterDeployment == nil {
		//TODO: think what we should return here (in context of state machine)
		return nil, resultGet, errGet
	}

	resultUpdate, errUpdate := m.updateDeploymentIfNeeded(ctx, clusterDeployment, builtDeployment)
	if errUpdate != nil {
		//TODO: think what we should return here (in context of state machine)
		return nil, resultUpdate, errUpdate
	}
	return nextState(sFnHandleService)
}

func (m *stateMachine) getOrCreateDeployment(ctx context.Context, builtDeployment *appsv1.Deployment) (*appsv1.Deployment, *ctrl.Result, error) {
	currentDeployment := &appsv1.Deployment{}
	f := m.state.instance
	deploymentErr := m.client.Get(ctx, client.ObjectKey{
		Namespace: f.Namespace,
		Name:      f.Name,
	}, currentDeployment)

	if deploymentErr == nil {
		return currentDeployment, nil, nil
	}
	if !errors.IsNotFound(deploymentErr) {
		m.log.Error(deploymentErr, "unable to fetch Deployment for Function")
		return nil, nil, deploymentErr
	}

	createResult, createErr := m.createDeployment(ctx, builtDeployment)
	return nil, createResult, createErr
}

func (m *stateMachine) createDeployment(ctx context.Context, deployment *appsv1.Deployment) (*ctrl.Result, error) {
	m.log.Info("creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)

	// Set the ownerRef for the Deployment, ensuring that the Deployment
	// will be deleted when the Function CR is deleted.
	controllerutil.SetControllerReference(&m.state.instance, deployment, m.scheme)

	if err := m.client.Create(ctx, deployment); err != nil {
		m.log.Error(err, "failed to create new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		m.state.instance.UpdateCondition(
			serverlessv1alpha2.ConditionRunning,
			metav1.ConditionFalse,
			serverlessv1alpha2.ConditionReasonDeploymentFailed,
			fmt.Sprintf("Deployment %s/%s create failed: %s", deployment.Namespace, deployment.Name, err.Error()))
		return nil, err
	}
	m.state.instance.UpdateCondition(
		serverlessv1alpha2.ConditionRunning,
		metav1.ConditionUnknown,
		serverlessv1alpha2.ConditionReasonDeploymentCreated,
		fmt.Sprintf("Deployment %s/%s updated", deployment.Namespace, deployment.Name))

	return &ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (m *stateMachine) updateDeploymentIfNeeded(ctx context.Context, clusterDeployment *appsv1.Deployment, builtDeployment *appsv1.Deployment) (*ctrl.Result, error) {
	// Ensure the Deployment data matches the desired state
	if !deploymentChanged(clusterDeployment, builtDeployment) {
		return nil, nil
	}

	//TODO: think if it's better to update only some fields
	clusterDeployment.Spec.Template = builtDeployment.Spec.Template
	clusterDeployment.Spec.Replicas = builtDeployment.Spec.Replicas
	return m.updateDeployment(ctx, clusterDeployment)
}

func deploymentChanged(a *appsv1.Deployment, b *appsv1.Deployment) bool {
	aSpec := a.Spec.Template.Spec.Containers[0]
	bSpec := b.Spec.Template.Spec.Containers[0]

	return aSpec.Image != bSpec.Image ||
		!reflect.DeepEqual(a.Spec.Template.ObjectMeta.Labels, b.Spec.Template.ObjectMeta.Labels) ||
		*a.Spec.Replicas != *b.Spec.Replicas ||
		!reflect.DeepEqual(aSpec.WorkingDir, bSpec.WorkingDir) ||
		!reflect.DeepEqual(aSpec.Command, bSpec.Command) ||
		!reflect.DeepEqual(aSpec.Resources, bSpec.Resources) ||
		!reflect.DeepEqual(aSpec.Env, bSpec.Env) ||
		!reflect.DeepEqual(aSpec.VolumeMounts, bSpec.VolumeMounts)
}

func (m *stateMachine) updateDeployment(ctx context.Context, clusterDeployment *appsv1.Deployment) (*ctrl.Result, error) {
	if err := m.client.Update(ctx, clusterDeployment); err != nil {
		m.log.Error(err, "Failed to update Deployment", "Deployment.Namespace", clusterDeployment.Namespace, "Deployment.Name", clusterDeployment.Name)
		m.state.instance.UpdateCondition(
			serverlessv1alpha2.ConditionRunning,
			metav1.ConditionFalse,
			serverlessv1alpha2.ConditionReasonDeploymentFailed,
			fmt.Sprintf("Deployment %s/%s update failed: %s", clusterDeployment.Namespace, clusterDeployment.Name, err.Error()))
		return nil, err
	}
	m.state.instance.UpdateCondition(
		serverlessv1alpha2.ConditionRunning,
		metav1.ConditionUnknown,
		serverlessv1alpha2.ConditionReasonDeploymentUpdated,
		fmt.Sprintf("Deployment %s/%s updated", clusterDeployment.Namespace, clusterDeployment.Name))
	// Requeue the request to ensure the Deployment is updated
	//TODO: rethink if it's better solution
	return &ctrl.Result{Requeue: true}, nil
}
