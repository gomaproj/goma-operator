package controller

import (
	"context"
	gomaprojv1beta1 "github.com/jkaninda/goma-operator/api/v1beta1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// createHpa creates HPA
func createHpa(r GatewayReconciler, ctx context.Context, req ctrl.Request, gateway *gomaprojv1beta1.Gateway) error {
	logger := log.FromContext(ctx)
	var metrics []autoscalingv2.MetricSpec
	targetCPUUtilizationPercentage := gateway.Spec.AutoScaling.TargetCPUUtilizationPercentage
	targetMemoryUtilizationPercentage := gateway.Spec.AutoScaling.TargetMemoryUtilizationPercentage
	// Add CPU metric if targetCPUUtilizationPercentage is set
	if targetCPUUtilizationPercentage != 0 {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: int32Ptr(targetCPUUtilizationPercentage),
				},
			},
		})
	}
	// Add Memory metric if targetMemoryUtilizationPercentage is set
	if targetMemoryUtilizationPercentage != 0 {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: "memory",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: int32Ptr(targetMemoryUtilizationPercentage),
				},
			},
		})
	}
	// Create HPA
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: int32Ptr(gateway.Spec.AutoScaling.MinReplicas),
			MaxReplicas: gateway.Spec.AutoScaling.MaxReplicas,
			Metrics:     metrics,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       req.Name,
			},
		},
	}
	// Check if the hpa already exists
	var existHpa autoscalingv2.HorizontalPodAutoscaler
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &existHpa)
	if err != nil && client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get HorizontalPodAutoscaler")
		return err
	}
	if err != nil && client.IgnoreNotFound(err) == nil {
		// Create the HPA if it doesn't exist
		if err = controllerutil.SetControllerReference(gateway, hpa, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference")
			return err
		}
		if err = r.Create(ctx, hpa); err != nil {
			logger.Error(err, "Failed to create HorizontalPodAutoscaler")
			return err
		}
		logger.Info("Created HorizontalPodAutoscaler", "HorizontalPodAutoscaler.Name", hpa.Name)
	} else {
		logger.Info("HorizontalPodAutoscaler already exists", "HorizontalPodAutoscaler.Name", hpa.Name)
		// Update the Deployment if the spec has changed
		if !reflect.DeepEqual(existHpa.Spec, hpa.Spec) {
			existHpa.Spec = hpa.Spec
			if err = r.Update(ctx, &existHpa); err != nil {
				logger.Error(err, "Failed to update Deployment")
				return err
			}
			logger.Info("Updated HorizontalPodAutoscaler", "HorizontalPodAutoscaler.Name", hpa.Name)
		}
	}
	return nil
}
