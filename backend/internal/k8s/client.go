package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	AnnotationBookedBy = "booking.argocd.io/booked-by"
	AnnotationBookedAt = "booking.argocd.io/booked-at"
)

var applicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

// Booking represents the booking state of an Application.
type Booking struct {
	AppName   string `json:"appName"`
	Namespace string `json:"namespace"`
	BookedBy  string `json:"bookedBy"`
	BookedAt  string `json:"bookedAt"`
}

// Client provides operations on ArgoCD Application CR annotations.
type Client interface {
	GetBookingStatus(ctx context.Context, namespace, appName string) (bookedBy string, bookedAt time.Time, err error)
	BookApp(ctx context.Context, namespace, appName, username string) error
	UnbookApp(ctx context.Context, namespace, appName, username string, isAdmin bool) error
	ListBookings(ctx context.Context, namespace string) ([]Booking, error)
}

type client struct {
	dynamic dynamic.Interface
}

// NewClient creates a new K8s client using in-cluster config.
func NewClient() (Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return &client{dynamic: dynClient}, nil
}

// NewClientFromDynamic creates a client from an existing dynamic.Interface (for testing).
func NewClientFromDynamic(dynClient dynamic.Interface) Client {
	return &client{dynamic: dynClient}
}

func (c *client) GetBookingStatus(ctx context.Context, namespace, appName string) (string, time.Time, error) {
	app, err := c.dynamic.Resource(applicationGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get application %s/%s: %w", namespace, appName, err)
	}

	annotations := app.GetAnnotations()
	if annotations == nil {
		return "", time.Time{}, nil
	}

	bookedBy := annotations[AnnotationBookedBy]
	if bookedBy == "" {
		return "", time.Time{}, nil
	}

	bookedAt, _ := time.Parse(time.RFC3339, annotations[AnnotationBookedAt])
	return bookedBy, bookedAt, nil
}

func (c *client) BookApp(ctx context.Context, namespace, appName, username string) error {
	bookedBy, _, err := c.GetBookingStatus(ctx, namespace, appName)
	if err != nil {
		return err
	}
	if bookedBy != "" && bookedBy != username {
		return fmt.Errorf("conflict: application already booked by %s", bookedBy)
	}
	if bookedBy == username {
		return nil // already booked by the same user
	}

	now := time.Now().UTC().Format(time.RFC3339)
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				AnnotationBookedBy: username,
				AnnotationBookedAt: now,
			},
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = c.dynamic.Resource(applicationGVR).Namespace(namespace).Patch(
		ctx, appName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch application %s/%s: %w", namespace, appName, err)
	}
	return nil
}

func (c *client) UnbookApp(ctx context.Context, namespace, appName, username string, isAdmin bool) error {
	bookedBy, _, err := c.GetBookingStatus(ctx, namespace, appName)
	if err != nil {
		return err
	}
	if bookedBy == "" {
		return nil // not booked
	}
	if bookedBy != username && !isAdmin {
		return fmt.Errorf("forbidden: application is booked by %s, only they or an admin can unbook", bookedBy)
	}

	// Remove annotations by setting them to null via JSON merge patch
	patch := []byte(`{"metadata":{"annotations":{"` + AnnotationBookedBy + `":null,"` + AnnotationBookedAt + `":null}}}`)
	_, err = c.dynamic.Resource(applicationGVR).Namespace(namespace).Patch(
		ctx, appName, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch application %s/%s: %w", namespace, appName, err)
	}
	return nil
}

func (c *client) ListBookings(ctx context.Context, namespace string) ([]Booking, error) {
	list, err := c.dynamic.Resource(applicationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list applications in %s: %w", namespace, err)
	}

	var bookings []Booking
	for _, item := range list.Items {
		b := extractBooking(&item)
		if b != nil {
			bookings = append(bookings, *b)
		}
	}
	return bookings, nil
}

func extractBooking(app *unstructured.Unstructured) *Booking {
	annotations := app.GetAnnotations()
	if annotations == nil {
		return nil
	}
	bookedBy := annotations[AnnotationBookedBy]
	if bookedBy == "" {
		return nil
	}
	return &Booking{
		AppName:   app.GetName(),
		Namespace: app.GetNamespace(),
		BookedBy:  bookedBy,
		BookedAt:  annotations[AnnotationBookedAt],
	}
}
