/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package k8s

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dop251/goja"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// K8s client for controlling k8s clusters from k6
type K8s struct {
	Pods *Pods
}

// New creates a new instance of th K8s struct
func New() *K8s {
	configPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	info, err := os.Stat(configPath)
	if os.IsNotExist(err) || info.IsDir() {
		return nil
	}

	config, _ := clientcmd.BuildConfigFromFlags("", configPath)
	client, _ := kubernetes.NewForConfig(config)

	return &K8s{
		Pods: NewPods(client),
	}
}

// Fail allows us to test that the module is actually loaded
func (*K8s) Fail(msg string) (goja.Value, error) {
	return goja.Undefined(), errors.New(msg)
}

// List pods in a specific namespace
func (k8s *K8s) List(ctx context.Context, namespace string) ([]string, error) {
	return k8s.Pods.List(namespace)
}

// Kill a specific pod in a specific namespace
func (k8s *K8s) Kill(ctx context.Context, namespace string, podName string) error {
	return k8s.Pods.Kill(namespace, podName)
}

// Status of a pod in a specific namespace
func (k8s *K8s) Status(ctx context.Context, namespace string, podName string) (string, error) {
	status, err := k8s.Pods.Status(namespace, podName)

	return status.String(), err
}
