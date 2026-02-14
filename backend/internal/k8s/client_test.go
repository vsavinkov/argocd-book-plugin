package k8s

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newFakeApp(namespace, name string, annotations map[string]string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})
	app.SetNamespace(namespace)
	app.SetName(name)
	if annotations != nil {
		app.SetAnnotations(annotations)
	}
	return app
}

func newFakeClient(objects ...runtime.Object) Client {
	scheme := runtime.NewScheme()
	fakeDyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			applicationGVR: "ApplicationList",
		},
		objects...,
	)
	return NewClientFromDynamic(fakeDyn)
}

func TestGetBookingStatus_NotBooked(t *testing.T) {
	app := newFakeApp("argocd", "my-app", nil)
	c := newFakeClient(app)

	bookedBy, _, err := c.GetBookingStatus(context.Background(), "argocd", "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bookedBy != "" {
		t.Fatalf("expected empty bookedBy, got %q", bookedBy)
	}
}

func TestGetBookingStatus_Booked(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	bookedBy, bookedAt, err := c.GetBookingStatus(context.Background(), "argocd", "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bookedBy != "alice" {
		t.Fatalf("expected bookedBy=alice, got %q", bookedBy)
	}
	if bookedAt.IsZero() {
		t.Fatal("expected non-zero bookedAt")
	}
}

func TestGetBookingStatus_AppNotFound(t *testing.T) {
	c := newFakeClient()

	_, _, err := c.GetBookingStatus(context.Background(), "argocd", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent app")
	}
}

func TestBookApp_Success(t *testing.T) {
	app := newFakeApp("argocd", "my-app", nil)
	c := newFakeClient(app)

	err := c.BookApp(context.Background(), "argocd", "my-app", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the patch was issued
	dynClient := c.(*client).dynamic.(*dynamicfake.FakeDynamicClient)
	actions := dynClient.Actions()
	var foundPatch bool
	for _, a := range actions {
		if a.GetVerb() == "patch" {
			foundPatch = true
			break
		}
	}
	if !foundPatch {
		t.Fatal("expected a patch action")
	}
}

func TestBookApp_AlreadyBookedBySameUser(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	err := c.BookApp(context.Background(), "argocd", "my-app", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBookApp_Conflict(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	err := c.BookApp(context.Background(), "argocd", "my-app", "bob")
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestUnbookApp_ByBooker(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	err := c.UnbookApp(context.Background(), "argocd", "my-app", "alice", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnbookApp_ByOtherUser_Forbidden(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	err := c.UnbookApp(context.Background(), "argocd", "my-app", "bob", false)
	if err == nil {
		t.Fatal("expected forbidden error")
	}
}

func TestUnbookApp_ByAdmin(t *testing.T) {
	app := newFakeApp("argocd", "my-app", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	c := newFakeClient(app)

	err := c.UnbookApp(context.Background(), "argocd", "my-app", "bob", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListBookings(t *testing.T) {
	app1 := newFakeApp("argocd", "app1", map[string]string{
		AnnotationBookedBy: "alice",
		AnnotationBookedAt: "2026-01-15T10:00:00Z",
	})
	app2 := newFakeApp("argocd", "app2", nil)
	app3 := newFakeApp("argocd", "app3", map[string]string{
		AnnotationBookedBy: "bob",
		AnnotationBookedAt: "2026-01-15T11:00:00Z",
	})
	c := newFakeClient(app1, app2, app3)

	bookings, err := c.ListBookings(context.Background(), "argocd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bookings) != 2 {
		t.Fatalf("expected 2 bookings, got %d", len(bookings))
	}
}

