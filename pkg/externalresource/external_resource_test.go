package externalresource

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func reset(name string, er *ExternalResource) {
	sanitizedExternalName := sanitizeForEnv(name)
	for _, endpoint := range er.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)
		os.Unsetenv(endpointUrlEnv)
	}
}

func TestExternalResource_LoadMarkdownContent(t *testing.T) {
	manifestPath := "/test/okteto.yml"
	markdownContent := "## Markdown content"
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/test/external/readme.md", []byte(markdownContent), 0600)

	tests := []struct {
		name                string
		externalResourceFSM ERFilesystemManager
		expectErr           bool
		expectedResult      ExternalResource
	}{
		{
			name: "markdown not found",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: &Notes{
						Path: "/readme.md",
					},
					Endpoints: []*ExternalEndpoint{},
				},
				Fs: fs,
			},
			expectErr: true,
		},
		{
			name: "valid markdown",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: &Notes{
						Path: "/external/readme.md",
					},
					Endpoints: []*ExternalEndpoint{},
				},
				Fs: fs,
			},
			expectErr: false,
			expectedResult: ExternalResource{
				Icon: "myIcon",
				Notes: &Notes{
					Path:     "/external/readme.md",
					Markdown: "IyMgTWFya2Rvd24gY29udGVudA==",
				},
				Endpoints: []*ExternalEndpoint{},
			},
		},
		{
			name: "notes info not present in external. No markdown loaded",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Endpoints: []*ExternalEndpoint{
						{
							Name: "name1",
							Url:  "/some/url",
						},
					},
				},
				Fs: fs,
			},
			expectErr: false,
			expectedResult: ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "name1",
						Url:  "/some/url",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectErr {
				assert.Error(t, tt.externalResourceFSM.LoadMarkdownContent(manifestPath))
			} else {
				assert.NoError(t, tt.externalResourceFSM.LoadMarkdownContent(manifestPath))
				assert.Equal(t, tt.externalResourceFSM.ExternalResource, tt.expectedResult)
			}
		})
	}
}

func TestExternalResource_SetURLUsingEnvironFile(t *testing.T) {
	externalResourceName := "test"
	newURLvalue := "/new/url/value"
	tests := []struct {
		name                     string
		externalResource         *ExternalResource
		expectedExternalResource *ExternalResource
		envsToSet                []string
		expectedErr              error
	}{
		{
			name: "error - empty url value",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
					},
				},
			},
			expectedErr: assert.AnError,
		},
		{
			name: "url value to add",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  newURLvalue,
					},
				},
			},
			envsToSet: []string{
				"OKTETO_EXTERNAL_TEST_ENDPOINTS_ENDPOINT1_URL",
			},
		},
		{
			name: "no url value to overwrite",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
				},
			},
		},
		{
			name: "url value to overwrite",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
					{
						Name: "endpoint2",
						Url:  "/url/for/endpoint/2",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  newURLvalue,
					},
					{
						Name: "endpoint2",
						Url:  "/url/for/endpoint/2",
					},
				},
			},
			envsToSet: []string{
				"OKTETO_EXTERNAL_TEST_ENDPOINTS_ENDPOINT1_URL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, env := range tt.envsToSet {
				os.Setenv(env, newURLvalue)
			}

			defer reset(externalResourceName, tt.externalResource)
			err := tt.externalResource.SetURLUsingEnvironFile(externalResourceName)
			if tt.expectedErr != nil {
				assert.Error(t, err)
			}

			if err == nil {
				assert.Equal(t, tt.expectedExternalResource, tt.externalResource)
			}
		})
	}
}