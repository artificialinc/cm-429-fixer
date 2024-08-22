// Package cm provides cert manager functionality
package cm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/artificialinc/cm-429-fixer/pkg/merge"
	acmev1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	apiwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientOpts is a set of options for the client
type ClientOpts struct {
	Context string
}

// GetLocalClient returns a client for the local cluster
func GetLocalClient(opts *ClientOpts) versioned.Interface {
	// Get kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}

	if opts != nil && opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	clientConfig, err := config.ClientConfig()
	if err != nil {
		panic(err)
	}

	httpClient, err := rest.HTTPClientFor(clientConfig)
	if err != nil {
		panic(err)
	}

	cmClient, err := versioned.NewForConfigAndClient(clientConfig, httpClient)
	if err != nil {
		panic(err)
	}

	return cmClient
}

// Watcher watches for orders and challenges and fixes them
type Watcher struct {
	c            versioned.Interface
	log          logr.Logger
	updateDelay  time.Duration
	resyncPeriod time.Duration
}

// Option is a function that sets some option on the watcher
type Option func(*Watcher)

// WithClient sets the client
func WithClient(c versioned.Interface) Option {
	return func(w *Watcher) {
		w.c = c
	}
}

// WithLogger sets the logger
func WithLogger(l logr.Logger) Option {
	return func(w *Watcher) {
		w.log = l
	}
}

const (
	// DefaultDelay is the default delay time
	DefaultDelay = 15 * time.Second
)

// WithUpdateDelay sets the update jitter time
func WithUpdateDelay(t time.Duration) Option {
	return func(w *Watcher) {
		w.updateDelay = t
	}
}

// WithResyncPeriod sets the resync period
func WithResyncPeriod(t time.Duration) Option {
	return func(w *Watcher) {
		w.resyncPeriod = t
	}
}

// NewWatcher creates a new watcher
func NewWatcher(opts ...Option) *Watcher {
	w := &Watcher{
		log:          logr.Discard(),
		updateDelay:  DefaultDelay,
		resyncPeriod: 15 * time.Minute,
	}
	for _, opt := range opts {
		opt(w)
	}

	if w.c == nil {
		c := GetLocalClient(nil)
		w.c = c
	}

	return w
}

// Run starts the watcher
func (w *Watcher) Run(ctx context.Context, ready chan bool) {
	challengeReady := make(chan bool)
	w.runInformer(ctx, w.challengeListWatcher(ctx), &acmev1.Challenge{}, challengeReady)
	orderReady := make(chan bool)
	w.runInformer(ctx, w.orderListWatcher(ctx), &acmev1.Order{}, orderReady)

	go func() {
		merged := merge.Bools(w.log, challengeReady, orderReady)
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-merged:
				ready <- s
			}
		}
	}()

	<-ctx.Done()
}

func (w *Watcher) updateOrder(o *acmev1.Order) {
	if o.Status.State == acmev1.Errored && strings.Contains(o.Status.Reason, "429") {
		// Copy object, informers are prohibited from modifying objects
		o = o.DeepCopy()
		// Rate limited, set status to pending to force retry
		o.Status.State = acmev1.Pending
		o.Status.Reason = ""
		// Update after delay to give time to settle
		go func() {
			w.log.Info("Rate limited, setting to pending", "order", o.Name, "namespace", o.Namespace, "delay", w.updateDelay)
			defer w.log.Info("Updated order", "order", o.Name, "namespace", o.Namespace)
			time.Sleep(w.updateDelay)
			_, err := w.c.AcmeV1().Orders(o.Namespace).UpdateStatus(context.Background(), o, metav1.UpdateOptions{})
			if err != nil {
				w.log.Error(err, "Error updating order", "order", o.Name)
			}
		}()
	}
}

func (w *Watcher) updateChallenge(c *acmev1.Challenge) {
	if c.Status.State == acmev1.Errored && strings.Contains(c.Status.Reason, "429") {
		// Copy object, informers are prohibited from modifying objects
		c = c.DeepCopy()
		w.log.Info("Rate limited, setting to pending", "challenge", c.Name, "namespace", c.Namespace)
		// Rate limited, set status to pending to force retry
		c.Status.State = acmev1.Pending
		c.Status.Reason = ""
		// Update after delay to give time to settle
		go func() {
			w.log.Info("Rate limited, setting to pending", "challenge", c.Name, "namespace", c.Namespace, "delay", w.updateDelay)
			defer w.log.Info("Updated challenge", "challenge", c.Name, "namespace", c.Namespace)
			time.Sleep(w.updateDelay)
			_, err := w.c.AcmeV1().Challenges(c.Namespace).UpdateStatus(context.Background(), c, metav1.UpdateOptions{})
			if err != nil {
				w.log.Error(err, "Error updating challenge", "challenge", c.Name)
			}
		}()
	}
}

func (w *Watcher) handleAdd(obj interface{}) {
	switch o := obj.(type) {
	case *acmev1.Order:
		w.updateOrder(o)
	case *acmev1.Challenge:
		w.updateChallenge(o)
	default:
		w.log.Error(errors.New("unexpected object type in handleAdd"), "object", obj)
	}
}

func (w *Watcher) handleUpdate(_, obj interface{}) {
	switch o := obj.(type) {
	case *acmev1.Order:
		w.updateOrder(o)
	case *acmev1.Challenge:
		w.updateChallenge(o)
	default:
		w.log.Error(errors.New("unexpected object type in handleUpdate"), "object", obj)
	}
}

func (w *Watcher) challengeListWatcher(ctx context.Context) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			o, err := w.c.AcmeV1().Challenges("").List(ctx, options)
			if err != nil {
				return nil, err
			}
			return o, nil
		},
		WatchFunc: func(options metav1.ListOptions) (apiwatch.Interface, error) {
			o, err := w.c.AcmeV1().Challenges("").Watch(ctx, options)
			if err != nil {
				return nil, err
			}
			return o, nil
		},
	}

}

func (w *Watcher) orderListWatcher(ctx context.Context) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			o, err := w.c.AcmeV1().Orders("").List(ctx, options)
			if err != nil {
				return nil, err
			}
			return o, nil
		},
		WatchFunc: func(options metav1.ListOptions) (apiwatch.Interface, error) {
			o, err := w.c.AcmeV1().Orders("").Watch(ctx, options)
			if err != nil {
				return nil, err
			}
			return o, nil
		},
	}
}

func (w *Watcher) runInformer(ctx context.Context, listerWatcher cache.ListerWatcher, objType runtime.Object, ready chan bool) {
	stopCh := make(chan struct{})
	_, informer := cache.NewInformerWithOptions(
		cache.InformerOptions{
			ListerWatcher: listerWatcher,
			ObjectType:    objType,
			ResyncPeriod:  w.resyncPeriod,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    w.handleAdd,
				UpdateFunc: w.handleUpdate,
			},
		},
	)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if informer.HasSynced() {
					ready <- true
					return
				}
			}
		}
	}()

	// Merge stopCh with ctx.Done
	go func() {
		select {
		case <-ctx.Done():
		case <-stopCh:
			close(stopCh)
		}
	}()

	go func() {
		informer.Run(stopCh)
	}()
}
