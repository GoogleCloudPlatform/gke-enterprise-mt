package gce

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGCEConf(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		want *Config
	}{
		{
			desc: "GCE",
			in: `[global]
project-id = k8s-lab
network-project-id = alextc-k8s-lab
network-name = default
subnetwork-name = default
node-instance-prefix = kubernetes-minion
node-tags = kubernetes-minion`,
			want: &Config{
				Global{
					ProjectID:        "k8s-lab",
					NetworkProjectID: "alextc-k8s-lab",
					NetworkName:      "default",
				},
			},
		},
		{
			desc: "GKE",
			in: `[global]
token-url = https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens
token-body = "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}"
project-id = gke-master
network-name = default
subnetwork-name = default
node-instance-prefix = gke-sandbox
node-tags = gke-sandbox-ac7f9ea0-node
regional=true`,
			want: &Config{
				Global{
					TokenURL:         "https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens",
					TokenBodyRaw:     `{"projectNumber":722785932522,"clusterId":"sandbox"}`,
					TokenBody:        &TokenBody{ProjectNumber: 722785932522, ClusterID: "sandbox"},
					Regional:         "true",
					ProjectID:        "gke-master",
					NetworkProjectID: "gke-master",
					NetworkName:      "default",
				},
			},
		},
		{
			desc: "NetworkURL set",
			in: `[global]
token-url = https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens
token-body = "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}"
project-id = gke-master
network-name = foobar/projects/project
subnetwork-name = default
node-instance-prefix = gke-sandbox
node-tags = gke-sandbox-ac7f9ea0-node
regional=true`,
			want: &Config{
				Global{
					TokenURL:         "https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens",
					TokenBodyRaw:     `{"projectNumber":722785932522,"clusterId":"sandbox"}`,
					TokenBody:        &TokenBody{ProjectNumber: 722785932522, ClusterID: "sandbox"},
					Regional:         "true",
					ProjectID:        "gke-master",
					NetworkProjectID: "gke-master",
					NetworkURL:       "foobar/projects/project",
					NetworkName:      "foobar/projects/project",
				},
			},
		},
		{
			desc: "inline comments, CRLF line endings, Global casing, uppercase keys, and values with equals",
			in: "[Global]\r\n" +
				"PROJECT-ID = k8s-lab ; inline semicolon comment\r\n" +
				"NETWORK-PROJECT-ID = alextc-k8s-lab # inline hash comment\r\n" +
				"TOKEN-URL = https://my-project.googleapis.com/v1/tokens?foo=bar&baz=1 # comment after URL\r\n" +
				"TOKEN-BODY = \"{\\\"projectNumber\\\":722785932522,\\\"clusterId\\\":\\\"sand#box;1=2\\\"}\" ; comment after quoted JSON\r\n" +
				"NETWORK-NAME = default\r\n" +
				"REGIONAL = true\r\n",
			want: &Config{
				Global{
					TokenURL:         "https://my-project.googleapis.com/v1/tokens?foo=bar&baz=1",
					TokenBodyRaw:     `{"projectNumber":722785932522,"clusterId":"sand#box;1=2"}`,
					TokenBody:        &TokenBody{ProjectNumber: 722785932522, ClusterID: "sand#box;1=2"},
					Regional:         "true",
					ProjectID:        "k8s-lab",
					NetworkProjectID: "alextc-k8s-lab",
					NetworkName:      "default",
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			r := strings.NewReader(tt.in)
			got, err := read(r)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("read(%q) mismatch (-want +got):\n%s", tt.desc, diff)
			}
		})
	}
}

func TestConfig_DeepCopy(t *testing.T) {
	tests := []struct {
		name string
		c    *Config
	}{
		{
			name: "nil config",
			c:    nil,
		},
		{
			name: "empty config",
			c:    &Config{},
		},
		{
			name: "config with global fields",
			c: &Config{
				Global: Global{
					ProjectID:   "test-project",
					NetworkName: "test-network",
				},
			},
		},
		{
			name: "config with token body",
			c: &Config{
				Global: Global{
					ProjectID: "test-project",
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "test-cluster",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.DeepCopy()

			if diff := cmp.Diff(tt.c, got); diff != "" {
				t.Errorf("DeepCopy() mismatch (-want +got):\n%s", diff)
			}

			if tt.c != nil {
				got.Global.ProjectID = "modified-project-id"
				if tt.c.Global.ProjectID == "modified-project-id" {
					t.Errorf("DeepCopy() did not create a deep copy for Global.ProjectID: mutating copy changed original")
				}

				if tt.c.Global.TokenBody != nil {
					got.Global.TokenBody.ClusterID = "modified-cluster-id"
					if tt.c.Global.TokenBody.ClusterID == "modified-cluster-id" {
						t.Errorf("DeepCopy() did not create a deep copy for Global.TokenBody.ClusterID: mutating copy changed original")
					}
				}
			}
		})
	}
}

func TestRead_FileAndErrors(t *testing.T) {
	t.Run("reads valid file and ignores comments and other sections", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "gce.conf")
		content := `# comment line
; semicolon comment
[other]
project-id = ignored-project
[global]
api-endpoint = https://compute.googleapis.com/compute/v1/
project-id = my-project
`
		if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Read(tmpFile)
		if err != nil {
			t.Fatalf("Read(%q) unexpected error: %v", tmpFile, err)
		}
		want := &Config{
			Global: Global{
				APIEndpoint:      "https://compute.googleapis.com/compute/v1/",
				ProjectID:        "my-project",
				NetworkProjectID: "my-project",
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Read(%q) diff (-want +got):\n%s", tmpFile, diff)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		if _, err := Read(filepath.Join(t.TempDir(), "nonexistent.conf")); err == nil {
			t.Fatal("Read() succeeded on nonexistent file, want error")
		}
	})

	t.Run("malformed line without equals returns error", func(t *testing.T) {
		if _, err := read(strings.NewReader("[global]\nmalformed_line\n")); err == nil {
			t.Fatal("read() succeeded on malformed line, want error")
		}
	})

	t.Run("malformed token-body JSON returns error", func(t *testing.T) {
		if _, err := read(strings.NewReader("[global]\ntoken-body = {not-json}\n")); err == nil {
			t.Fatal("read() succeeded on malformed token-body JSON, want error")
		}
	})
}
