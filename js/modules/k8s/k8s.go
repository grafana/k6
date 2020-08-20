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
	"github.com/dop251/goja"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

type K8s struct{
	Pods *Pods
}

func New() *K8s {
	configPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, _ := clientcmd.BuildConfigFromFlags("", configPath)
	clientset, _ := kubernetes.NewForConfig(config)

	return &K8s{
		Pods: NewPods(clientset),
	}
}

func (*K8s) Fail(msg string) (goja.Value, error) {
	return goja.Undefined(), errors.New(msg)
}

func (k8s *K8s) List(ctx context.Context, namespace string) ([]string, error) {
	return k8s.Pods.List(namespace)
}

func (k8s *K8s) Kill(ctx context.Context, namespace string, podName string) error {
	return k8s.Pods.Kill(namespace, podName)
}

func (k8s *K8s) Status(ctx context.Context, namespace string, podName string) (string, error) {
	status, err := k8s.Pods.Status(namespace, podName)
	return status.String(), err
}
