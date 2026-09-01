// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// node builds a Node object in the given readiness state.
func node(name string, ready bool) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}},
		},
	}
}

func TestWaitForNodesReady_ReturnsWhenAllNodesReady(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(node("cp", true), node("w1", true))

	err := waitForNodesReady(context.Background(), client, time.Second, time.Millisecond)
	assert.NoError(t, err)
}

// A cluster whose nodes never settle must fail with a message naming the nodes
// still not ready; a bare "timeout" leaves an operator with nowhere to look.
func TestWaitForNodesReady_TimeoutNamesNotReadyNodes(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(node("cp", true), node("w1", false), node("w2", false))

	err := waitForNodesReady(context.Background(), client, 50*time.Millisecond, time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "w1")
	assert.Contains(t, err.Error(), "w2")
	assert.NotContains(t, err.Error(), "cp", "ready nodes must not be reported as pending")
}

// The API server is not reachable for the first moments after cluster creation,
// so list errors must be treated as "not ready yet", not as fatal.
func TestWaitForNodesReady_KeepsPollingThroughListErrors(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		if calls.Add(1) < 3 {
			return true, nil, assert.AnError
		}
		return true, &corev1.NodeList{Items: []corev1.Node{*node("cp", true)}}, nil
	})

	err := waitForNodesReady(context.Background(), client, time.Second, time.Millisecond)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, calls.Load(), int32(3), "must have retried past the failures")
}

// An empty node list means the control plane has not registered anything yet.
func TestWaitForNodesReady_KeepsPollingWhileNodeListEmpty(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	err := waitForNodesReady(context.Background(), client, 30*time.Millisecond, time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForNodesReady_HonoursContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := fake.NewSimpleClientset(node("cp", false))

	err := waitForNodesReady(ctx, client, time.Minute, time.Millisecond)

	assert.ErrorIs(t, err, context.Canceled)
}

// A node reporting no Ready condition at all is pending, not ready.
func TestWaitForNodesReady_NodeWithoutReadyConditionIsPending(t *testing.T) {
	t.Parallel()

	bare := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "bare"}}
	client := fake.NewSimpleClientset(bare)

	err := waitForNodesReady(context.Background(), client, 30*time.Millisecond, time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bare")
}

// waitForAllNodesReady is the production entry point; it must reject a
// kubeconfig it cannot parse rather than hanging until the timeout.
func TestWaitForAllNodesReady_RejectsUnusableKubeconfig(t *testing.T) {
	t.Parallel()

	err := waitForAllNodesReady(context.Background(), "not a kubeconfig", 50*time.Millisecond)
	assert.Error(t, err)
}

var _ kubernetes.Interface = fake.NewSimpleClientset()

// A kubeconfig can be structurally valid yet still fail to yield a client: here
// certificate-authority-data is valid base64 but not a PEM certificate, so TLS
// setup fails. The wait must surface that immediately rather than polling.
func TestWaitForAllNodesReady_RejectsKubeconfigWithUnusableCA(t *testing.T) {
	t.Parallel()

	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: aGVsbG8=
users:
- name: u
  user: {}
contexts:
- name: x
  context:
    cluster: c
    user: u
current-context: x
`

	start := time.Now()
	err := waitForAllNodesReady(context.Background(), kubeconfig, time.Minute)

	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second, "must fail fast, not wait for the timeout")
}
