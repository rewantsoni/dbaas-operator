package grafana

import (
	"context"
	"github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	"github.com/RHEcosystemAppEng/dbaas-operator/controllers/reconcilers"
	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	corev1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace     = "openshift-dbaas-monitoring"
	operatorGroup = "dbaas-monitoring"
	grafanaName   = "dbaas-grafana-operator"
	grafanaCSV    = "grafana-operator.v4.2.0"
	managedBy     = "app.kubernetes.io/managed-by"
	operatorName  = "dbaas-operator"
)

type Reconciler struct {
	client client.Client
	log    logr.Logger
}

func NewReconciler(client client.Client, log logr.Logger) reconcilers.PlatformReconciler {
	return &Reconciler{
		client: client,
		log:    log,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, cr *v1alpha1.DBaaSPlatform, status2 *v1alpha1.DBaaSPlatformStatus) (v1alpha1.PlatformsInstlnStatus, error) {

	status, err := r.reconcileNamespace(ctx)
	if status != v1alpha1.ResultSuccess {
		return status, err
	}

	status, err = r.reconcileOperatorGroup(ctx)
	if status != v1alpha1.ResultSuccess {
		return status, err
	}

	status, err = r.reconcileSubscription(ctx)
	if status != v1alpha1.ResultSuccess {
		return status, err
	}

	return v1alpha1.ResultSuccess, nil
}

func (r *Reconciler) Cleanup(ctx context.Context, cr *v1alpha1.DBaaSPlatform) (v1alpha1.PlatformsInstlnStatus, error) {

	subscription := &corev1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grafanaName,
			Namespace: namespace,
		},
	}
	err := r.client.Delete(ctx, subscription)
	if err != nil && !errors.IsNotFound(err) {
		return v1alpha1.ResultFailed, err
	}

	return v1alpha1.ResultSuccess, nil
}

func (r *Reconciler) reconcileNamespace(ctx context.Context) (v1alpha1.PlatformsInstlnStatus, error) {

	key := types.NamespacedName{Name: namespace}
	var namespace corev1.Namespace
	err := r.client.Get(ctx, key, &namespace)
	if err != nil && !errors.IsNotFound(err) {
		return v1alpha1.ResultFailed, err
	}

	if errors.IsNotFound(err) {
		r.log.Info("Creating namespace")
		err = r.client.Create(ctx, newNamespace())
		return v1alpha1.ResultFailed, err
	}

	// requeue if namespace is marked for deletion
	// TODO: decide if want to use finalizers to prevent deletion but
	// we also need to solve how to properly cleanup / uninstall operator
	if namespace.Status.Phase != corev1.NamespaceActive {
		r.log.Info("Namespace is present but not active", "phase", namespace.Status.Phase)
		return v1alpha1.ResultInProgress, nil
	}
	return v1alpha1.ResultSuccess, nil
}

func (r *Reconciler) reconcileOperatorGroup(ctx context.Context) (v1alpha1.PlatformsInstlnStatus, error) {
	log := r.log.WithValues("Name", grafanaName)
	log.V(6).Info("Reconciling OperatorGroup")

	key := types.NamespacedName{
		Name:      operatorGroup,
		Namespace: namespace,
	}
	var operatorGroup operatorsv1.OperatorGroup

	err := r.client.Get(ctx, key, &operatorGroup)
	if err != nil && !errors.IsNotFound(err) {
		return v1alpha1.ResultFailed, err
	}

	// create
	desired := newOperatorGroup()
	if errors.IsNotFound(err) {
		log.Info("Creating OperatorGroup")
		err := r.client.Create(ctx, desired)
		return v1alpha1.ResultFailed, err
	}

	// update
	if !reflect.DeepEqual(operatorGroup.Spec, desired.Spec) {
		log.Info("Updating OperatorGroup")
		operatorGroup.Spec = desired.Spec
		err := r.client.Update(ctx, &operatorGroup)
		if err != nil {
			return v1alpha1.ResultFailed, err
		}
	}

	return v1alpha1.ResultSuccess, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context) (v1alpha1.PlatformsInstlnStatus, error) {
	log := r.log.WithValues("Name", grafanaName)
	key := types.NamespacedName{
		Name:      grafanaName,
		Namespace: namespace,
	}
	var subscription corev1alpha1.Subscription
	err := r.client.Get(ctx, key, &subscription)
	if err != nil && !errors.IsNotFound(err) {
		return v1alpha1.ResultFailed, err
	}

	// create
	desired := newSubscription()
	if errors.IsNotFound(err) {
		log.Info("Creating Grafana Operator Subscription")
		err := r.client.Create(ctx, desired)
		if err != nil {
			return v1alpha1.ResultFailed, err
		}
		return v1alpha1.ResultInProgress, nil
	}

	if subscription.Spec.StartingCSV == desired.Spec.StartingCSV {
		return v1alpha1.ResultSuccess, nil
	}

	r.log.WithValues("Name", subscription.Name).Info("Deleting Subscription")
	if err := r.client.Delete(ctx, &subscription); err != nil {
		return v1alpha1.ResultFailed, err
	}

	r.log.WithValues("Name", subscription.Status.InstalledCSV).Info("Deleting CSV")
	csv := corev1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterServiceVersion",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscription.Status.InstalledCSV,
			Namespace: namespace,
			Labels:    commonLabels(),
		},
	}
	if err := r.client.Delete(ctx, &csv); err != nil {
		return v1alpha1.ResultFailed, err
	}

	r.log.WithValues("Name", subscription.Name).Info("Creating Subscription")
	err = r.client.Create(ctx, &subscription)
	if err != nil {
		return v1alpha1.ResultFailed, err
	}
	return v1alpha1.ResultSuccess, nil
}

func newNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: commonLabels(),
		},
	}
}

func newSubscription() *corev1alpha1.Subscription {
	return &corev1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Subscription",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      grafanaName,
			Namespace: namespace,
			Labels:    commonLabels(),
		},
		Spec: &corev1alpha1.SubscriptionSpec{
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
			Package:                "grafana-operator",
			Channel:                "v4",
			InstallPlanApproval:    corev1alpha1.ApprovalAutomatic,
			StartingCSV:            grafanaCSV,
			Config: &corev1alpha1.SubscriptionConfig{
				Env: []corev1.EnvVar{
					{
						Name:  "DASHBOARD_NAMESPACES_ALL",
						Value: "true",
					},
				},
			},
		},
	}
}

func commonLabels() map[string]string {
	return map[string]string{
		managedBy: operatorName,
	}
}

func newOperatorGroup() *operatorsv1.OperatorGroup {
	return &operatorsv1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorsv1.SchemeGroupVersion.String(),
			Kind:       "OperatorGroup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorGroup,
			Namespace: namespace,
			Labels:    commonLabels(),
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				namespace,
			},
		},
	}
}
