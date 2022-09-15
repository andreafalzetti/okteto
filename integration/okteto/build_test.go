package okteto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const (
	manifestContentWithSecrets = `
build:
  test:
    context: .
    args:
    - RABBITMQ_PASS=${RABBITMQ_PASS}

deploy:
- echo "fake deploy"
`
	dockerfileUsingSecrets = `
FROM nginx
ARG RABBITMQ_PASS
RUN if [ -z "$RABBITMQ_PASS" ]; then exit 1; else echo $RABBITMQ_PASS; fi
`
)

func TestBuildReplaceSecretsInManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createManifestV2(dir, manifestContentWithSecrets))
	require.NoError(t, createDockerfile(dir))
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	testNamespace := integration.GetTestNamespace("TestBuildWithSecrets", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	options := &commands.BuildOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
}

func createDockerfile(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfileContent := []byte(dockerfileUsingSecrets)
	return os.WriteFile(dockerfilePath, dockerfileContent, 0600)
}

func createManifestV2(dir, content string) error {
	manifestPath := filepath.Join(dir, "okteto.yml")
	manifestBytes := []byte(content)
	return os.WriteFile(manifestPath, manifestBytes, 0600)
}
