package wasm

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// moduleListNode is a node in a doubly linked list of names.
type moduleListNode struct {
	name       string
	module     *ModuleInstance
	next, prev *moduleListNode
}

// setModule makes the module visible for import.
func (s *Store) setModule(m *ModuleInstance) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	node, ok := s.nameToNode[m.Name]
	if !ok {
		return fmt.Errorf("module[%s] name has not been required", m.Name)
	}

	node.module = m
	return nil
}

// deleteModule makes the moduleName available for instantiation again.
func (s *Store) deleteModule(moduleName string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	node, ok := s.nameToNode[moduleName]
	if !ok {
		return nil
	}

	// remove this module name
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		s.moduleList = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	delete(s.nameToNode, moduleName)
	return nil
}

// module returns the module of the given name or error if not in this store
func (s *Store) module(moduleName string) (*ModuleInstance, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	node, ok := s.nameToNode[moduleName]
	if !ok {
		return nil, fmt.Errorf("module[%s] not in store", moduleName)
	}

	if node.module == nil {
		return nil, fmt.Errorf("module[%s] not set in store", moduleName)
	}

	return node.module, nil
}

// requireModules returns all instantiated modules whose names equal the keys in the input, or errs if any are missing.
func (s *Store) requireModules(moduleNames map[string]struct{}) (map[string]*ModuleInstance, error) {
	ret := make(map[string]*ModuleInstance, len(moduleNames))

	s.mux.RLock()
	defer s.mux.RUnlock()

	for n := range moduleNames {
		node, ok := s.nameToNode[n]
		if !ok {
			return nil, fmt.Errorf("module[%s] not instantiated", n)
		}
		ret[n] = node.module
	}
	return ret, nil
}

// requireModuleName is a pre-flight check to reserve a module.
// This must be reverted on error with deleteModule if initialization fails.
func (s *Store) requireModuleName(moduleName string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if _, ok := s.nameToNode[moduleName]; ok {
		return fmt.Errorf("module[%s] has already been instantiated", moduleName)
	}

	// add the newest node to the moduleNamesList as the head.
	node := &moduleListNode{
		name: moduleName,
		next: s.moduleList,
	}
	if node.next != nil {
		node.next.prev = node
	}
	s.moduleList = node
	s.nameToNode[moduleName] = node
	return nil
}

// AliasModule aliases the instantiated module named `src` as `dst`.
//
// Note: This is only used for spectests.
func (s *Store) AliasModule(src, dst string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.nameToNode[dst] = s.nameToNode[src]
	return nil
}

// Module implements wazero.Runtime Module
func (s *Store) Module(moduleName string) api.Module {
	m, err := s.module(moduleName)
	if err != nil {
		return nil
	}
	return m.CallCtx
}
