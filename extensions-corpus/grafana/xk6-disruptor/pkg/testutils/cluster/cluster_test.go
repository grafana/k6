package cluster

import (
	"testing"
)

const defaultConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane`

const configWithNodePort = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 32080
    hostPort: 8080
    listenAddress: "0.0.0.0"
    protocol: tcp`

const configWithWorkers = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker`

func Test_CreateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title          string
		options        Options
		expectError    bool
		expectedConfig string
	}{
		{
			title:          "Default config",
			options:        Options{},
			expectError:    false,
			expectedConfig: defaultConfig,
		},
		{
			title: "Node Ports",
			options: Options{
				NodePorts: []NodePort{
					{
						NodePort: 32080,
						HostPort: 8080,
					},
				},
			},
			expectError:    false,
			expectedConfig: configWithNodePort,
		},
		{
			title: "Invalid Node Ports",
			options: Options{
				NodePorts: []NodePort{
					{
						NodePort: 0,
						HostPort: 8080,
					},
				},
			},
			expectError:    true,
			expectedConfig: "",
		},
		{
			title: "Worker nodes",
			options: Options{
				Workers: 2,
			},
			expectError:    false,
			expectedConfig: configWithWorkers,
		},
		{
			title: "Custom config",
			options: Options{
				Config: defaultConfig,
			},
			expectError:    false,
			expectedConfig: defaultConfig,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			config, err := NewConfig("test-cluster", tc.options)

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && config == nil {
				t.Errorf("a config was expected but none was returned")
				return
			}

			kindConfig, _ := config.Render()
			if kindConfig != tc.expectedConfig {
				t.Errorf("Actual config is not the expected\n"+
					"Actual:\n%s\nExpected:\n%s\n",
					kindConfig,
					tc.expectedConfig,
				)
			}
		})
	}
}
