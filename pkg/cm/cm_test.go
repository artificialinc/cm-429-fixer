// Package cn_test is the test package for cm.
package cm_test

import (
	"context"
	"testing"

	"github.com/artificialinc/cm-429-fixer/pkg/cm"
	acmev1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
)

func buildOrder(name, namespace string, status *acmev1.OrderStatus) *acmev1.Order {
	return &acmev1.Order{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: *status,
	}
}

func buildChallenge(name, namespace string, status *acmev1.ChallengeStatus) *acmev1.Challenge {
	return &acmev1.Challenge{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: *status,
	}
}

func TestWatcherOrder(t *testing.T) {
	type test struct {
		name     string
		existing func(*testing.T) []runtime.Object
		actions  []func(*testing.T, context.Context, versioned.Interface)
		expected func(*testing.T, context.Context, versioned.Interface)
	}

	tests := []test{
		{
			name: "add",
			actions: []func(t *testing.T, ctx context.Context, c versioned.Interface){
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Orders("default").Create(ctx, buildOrder("order1", "default", &acmev1.OrderStatus{
						State:  acmev1.Errored,
						Reason: "some 429 error",
					}), metav1.CreateOptions{})
					assert.NoError(t, err)
				},
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Orders("default").Create(ctx, buildOrder("order2", "default", &acmev1.OrderStatus{
						State:  acmev1.Errored,
						Reason: "some other error",
					}), metav1.CreateOptions{})
					assert.NoError(t, err)
				},
			},
			expected: func(t *testing.T, ctx context.Context, c versioned.Interface) {
				orders, err := c.AcmeV1().Orders("default").List(ctx, metav1.ListOptions{})
				assert.NoError(t, err)
				for _, o := range orders.Items {
					if o.Name == "order1" {
						assert.Equal(t, acmev1.Pending, o.Status.State)
						assert.Equal(t, "", o.Status.Reason)
					} else if o.Name == "order2" {
						assert.Equal(t, acmev1.Errored, o.Status.State)
						assert.Equal(t, "some other error", o.Status.Reason)
					}
				}
			},
		},
		{
			name: "update",
			existing: func(*testing.T) []runtime.Object {
				return []runtime.Object{
					buildOrder("order1", "default", &acmev1.OrderStatus{
						State: acmev1.Pending,
					}),
					buildOrder("order2", "default", &acmev1.OrderStatus{
						State: acmev1.Pending,
					}),
				}
			},
			actions: []func(t *testing.T, ctx context.Context, c versioned.Interface){
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Orders("default").UpdateStatus(ctx, buildOrder("order1", "default", &acmev1.OrderStatus{
						State:  acmev1.Errored,
						Reason: "some 429 error",
					}), metav1.UpdateOptions{})
					assert.NoError(t, err)
				},
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Orders("default").UpdateStatus(ctx, buildOrder("order2", "default", &acmev1.OrderStatus{
						State:  acmev1.Errored,
						Reason: "some other error",
					}), metav1.UpdateOptions{})
					assert.NoError(t, err)
				},
			},
			expected: func(t *testing.T, ctx context.Context, c versioned.Interface) {
				orders, err := c.AcmeV1().Orders("default").List(ctx, metav1.ListOptions{})
				assert.NoError(t, err)
				for _, o := range orders.Items {
					if o.Name == "order1" {
						assert.Equal(t, acmev1.Pending, o.Status.State)
						assert.Equal(t, "", o.Status.Reason)
					} else if o.Name == "order2" {
						assert.Equal(t, acmev1.Errored, o.Status.State)
						assert.Equal(t, "some other error", o.Status.Reason)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.TODO())

			var existing []runtime.Object
			if tt.existing != nil {
				existing = tt.existing(t)
			}

			client := fake.NewSimpleClientset(existing...)

			w := cm.NewWatcher(
				cm.WithClient(client),
			)

			ready := make(chan bool)

			go func() {
				w.Run(ctx, ready)
			}()

			// Wait for the controller to sync
			doneWaiting := false
			for !doneWaiting {
				doneWaiting = <-ready
			}

			for _, a := range tt.actions {
				a(t, ctx, client)
			}

			cancel()

		})
	}
}

func TestWatcherChallenge(t *testing.T) {
	type test struct {
		name     string
		existing func(*testing.T) []runtime.Object
		actions  []func(*testing.T, context.Context, versioned.Interface)
		expected func(*testing.T, context.Context, versioned.Interface)
	}

	tests := []test{
		{
			name: "add",
			actions: []func(t *testing.T, ctx context.Context, c versioned.Interface){
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Challenges("default").Create(ctx, buildChallenge("order1", "default", &acmev1.ChallengeStatus{
						State:  acmev1.Errored,
						Reason: "some 429 error",
					}), metav1.CreateOptions{})
					assert.NoError(t, err)
				},
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Challenges("default").Create(ctx, buildChallenge("order2", "default", &acmev1.ChallengeStatus{
						State:  acmev1.Errored,
						Reason: "some other error",
					}), metav1.CreateOptions{})
					assert.NoError(t, err)
				},
			},
			expected: func(t *testing.T, ctx context.Context, c versioned.Interface) {
				orders, err := c.AcmeV1().Challenges("default").List(ctx, metav1.ListOptions{})
				assert.NoError(t, err)
				for _, o := range orders.Items {
					if o.Name == "order1" {
						assert.Equal(t, acmev1.Pending, o.Status.State)
						assert.Equal(t, "", o.Status.Reason)
					} else if o.Name == "order2" {
						assert.Equal(t, acmev1.Errored, o.Status.State)
						assert.Equal(t, "some other error", o.Status.Reason)
					}
				}
			},
		},
		{
			name: "update",
			existing: func(*testing.T) []runtime.Object {
				return []runtime.Object{
					buildChallenge("order1", "default", &acmev1.ChallengeStatus{
						State: acmev1.Pending,
					}),
					buildChallenge("order2", "default", &acmev1.ChallengeStatus{
						State: acmev1.Pending,
					}),
				}
			},
			actions: []func(t *testing.T, ctx context.Context, c versioned.Interface){
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Challenges("default").UpdateStatus(ctx, buildChallenge("order1", "default", &acmev1.ChallengeStatus{
						State:  acmev1.Errored,
						Reason: "some 429 error",
					}), metav1.UpdateOptions{})
					assert.NoError(t, err)
				},
				func(t *testing.T, ctx context.Context, c versioned.Interface) {
					_, err := c.AcmeV1().Challenges("default").UpdateStatus(ctx, buildChallenge("order2", "default", &acmev1.ChallengeStatus{
						State:  acmev1.Errored,
						Reason: "some other error",
					}), metav1.UpdateOptions{})
					assert.NoError(t, err)
				},
			},
			expected: func(t *testing.T, ctx context.Context, c versioned.Interface) {
				orders, err := c.AcmeV1().Challenges("default").List(ctx, metav1.ListOptions{})
				assert.NoError(t, err)
				for _, o := range orders.Items {
					if o.Name == "order1" {
						assert.Equal(t, acmev1.Pending, o.Status.State)
						assert.Equal(t, "", o.Status.Reason)
					} else if o.Name == "order2" {
						assert.Equal(t, acmev1.Errored, o.Status.State)
						assert.Equal(t, "some other error", o.Status.Reason)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.TODO())

			var existing []runtime.Object
			if tt.existing != nil {
				existing = tt.existing(t)
			}

			client := fake.NewSimpleClientset(existing...)

			w := cm.NewWatcher(
				cm.WithClient(client),
			)

			ready := make(chan bool)

			go func() {
				w.Run(ctx, ready)
			}()

			// Wait for the controller to sync
			doneWaiting := false
			for !doneWaiting {
				doneWaiting = <-ready
			}

			for _, a := range tt.actions {
				a(t, ctx, client)
			}

			cancel()

		})
	}
}
