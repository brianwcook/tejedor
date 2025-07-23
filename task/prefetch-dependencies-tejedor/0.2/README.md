# prefetch-dependencies-tejedor task

Task that uses Hermeto to prefetch build dependencies with Tejedor as a PyPI proxy sidecar for Python dependencies.
See docs at https://github.com/containerbuildsystem/cachi2#basic-usage.

## Tejedor Integration

When Python pip dependencies are detected in the input, this task will:
1. Start Tejedor as a sidecar container serving as a PyPI proxy
2. Configure Hermeto to use Tejedor as the PyPI index
3. Enable wheel downloads in Hermeto configuration
4. Use the specified private PyPI URL for Tejedor

The Tejedor sidecar will be automatically terminated when the task completes.

## Configuration

Config file must be passed as a YAML string. For all available config options please check
[available configuration parameters] page.

Example of setting timeouts:

```yaml
params:
  - name: config-file-content
    value: |
      ---
      requests_timeout: 300
      subprocess_timeout: 3600
```

[available configuration parameters]: https://github.com/containerbuildsystem/cachi2?tab=readme-ov-file#available-configuration-parameters

## Parameters
|name|description|default value|required|
|---|---|---|---|
|input|Configures project packages that will have their dependencies prefetched.||true|
|dev-package-managers|Enable in-development package managers. WARNING: the behavior may change at any time without notice. Use at your own risk. |false|false|
|log-level|Set cachi2 log level (debug, info, warning, error)|info|false|
|config-file-content|Pass configuration to cachi2. Note this needs to be passed as a YAML-formatted config dump, not as a file path! |""|false|
|sbom-type|Select the SBOM format to generate. Valid values: spdx, cyclonedx.|spdx|false|
|caTrustConfigMapName|The name of the ConfigMap to read CA bundle data from.|trusted-ca|false|
|caTrustConfigMapKey|The name of the key in the ConfigMap that contains the CA bundle data.|ca-bundle.crt|false|
|ACTIVATION_KEY|Name of secret which contains subscription activation key|activation-key|false|
|private-pypi-url|URL of the private PyPI server to use with Tejedor|""|false|
|proxy-server|Optional proxy server URL for Tejedor to use for external requests|""|false|

## Workspaces
|name|description|optional|
|---|---|---|
|source|Workspace with the source code, cachi2 artifacts will be stored on the workspace as well|false|
|git-basic-auth|A Workspace containing a .gitconfig and .git-credentials file or username and password. These will be copied to the user's home before any cachi2 commands are run. Any other files in this Workspace are ignored. It is strongly recommended to bind a Secret to this Workspace over other volume types. |true|
|netrc|Workspace containing a .netrc file. Cachi2 will use the credentials in this file when performing http(s) requests. |true|

## Usage Examples

### Basic Python pip dependency prefetching

```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: prefetch-python-deps
spec:
  params:
    - name: private-pypi-url
      value: "https://private-pypi.company.com/simple/"
  workspaces:
    - name: source
  tasks:
    - name: prefetch-deps
      taskRef:
        name: prefetch-dependencies-tejedor
      params:
        - name: input
          value: "pip"
        - name: private-pypi-url
          value: $(params.private-pypi-url)
      workspaces:
        - name: source
          workspace: source
```

### Mixed dependencies with Python and Go

```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: prefetch-mixed-deps
spec:
  params:
    - name: private-pypi-url
      value: "https://private-pypi.company.com/simple/"
  workspaces:
    - name: source
  tasks:
    - name: prefetch-deps
      taskRef:
        name: prefetch-dependencies-tejedor
      params:
        - name: input
          value: |
            [
              {"type": "pip"},
              {"type": "gomod"}
            ]
        - name: private-pypi-url
          value: $(params.private-pypi-url)
      workspaces:
        - name: source
          workspace: source
```

### With proxy server

```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: prefetch-with-proxy
spec:
  params:
    - name: private-pypi-url
      value: "https://private-pypi.company.com/simple/"
    - name: proxy-server
      value: "http://proxy.company.com:8080"
  workspaces:
    - name: source
  tasks:
    - name: prefetch-deps
      taskRef:
        name: prefetch-dependencies-tejedor
      params:
        - name: input
          value: "pip"
        - name: private-pypi-url
          value: $(params.private-pypi-url)
        - name: proxy-server
          value: $(params.proxy-server)
      workspaces:
        - name: source
          workspace: source
``` 