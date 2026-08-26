package utils

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// RuntimeToUnstructured converts a runtime object in a unstructured object
func RuntimeToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	// transform runtime into a generic object
	generic, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	// create an unstructured object form the generic object
	uObj := &unstructured.Unstructured{
		Object: generic,
	}

	return uObj, nil
}

// UnstructuredToRuntime converts an unstructured object in a runtime object
func UnstructuredToRuntime(uObj *unstructured.Unstructured, obj interface{}) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(uObj.UnstructuredContent(), obj)
}

// GenericToRuntime converts a generic object to a Runtime object
func GenericToRuntime(obj map[string]interface{}, rtObj interface{}) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(obj, rtObj)
}

// RuntimeToGeneric converts a runtime object in a unstructured object
func RuntimeToGeneric(obj interface{}) (map[string]interface{}, error) {
	return runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
}
