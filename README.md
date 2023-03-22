# rename-pvc

`rename-pvc` can rename PersistentVolumeClaims (PVCs) inside Kubernetes.

:warning: Be sure to create a backup of your data in the PVC before using `rename-pvc`!

## Installation

### From krew plugin manager

See [krew install guide.](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)

Update krew packages and install `rename-pvc`:

```shell
kubectl krew update
kubectl krew install rename-pvc
```

Now you can use `rename-pvc` with `kubectl rename-pvc`.

### From source

If you have Go 1.16+, you can directly install by running:

```bash
go install github.com/stackitcloud/rename-pvc/cmd/rename-pvc@latest
```
> Based on your go configuration the `rename-pvc` binary can be found in `$GOPATH/bin` or `$HOME/go/bin` in case `$GOPATH` is not set.
> Make sure to add the respective directory to your `$PATH`.
> [For more information see go docs for further information](https://golang.org/ref/mod#go-install). Run `go env` to view your current configuration.

### From the released binaries

Download the desired version for your operating system and processor architecture from the [`rename-pvc` releases page](https://github.com/stackitcloud/rename-pvc/releases).
Make the file executable and place it in a directory available in your `$PATH`.

## Usage

To rename a PVC from `pvc-name` to `new-pvc-name` run the command:

```shell
rename-pvc pvc-name new-pvc-name
```

Example Output:

```shell
Rename PVC from 'pvc-name' in namespace 'default' to 'new-pvc-name' in namespace 'default'? (yes or no) y
New PVC with name 'new-pvc-name' created
ClaimRef of PV 'pvc-2dc982d6-72a0-4e80-b1a6-126b108d2adf' is updated to new PVC 'new-pvc-name'
New PVC 'new-pvc-name' is bound to PV 'pvc-2dc982d6-72a0-4e80-b1a6-126b108d2adf'
Old PVC 'pvc-name' is deleted
```

With the flag `--target-namespace` it is possible to change the namespace of the newly created PVC. `rename-pvc -n test1 --target-namespace test2 pvc-name pvc-name` will create the new PVC in Namespace `test2`.

To select the Namespace and Kubernetes cluster you can use the default `kubectl` flags and environment variables (like `--namespace`, `--kubeconfig` or the `KUBECONFIG` environment variable).
For all options run `--help`.

```shell
Flags:
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "/home/m/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
  -h, --help                           help for /tmp/go-build4237287669/b001/exe/main
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string               If present, the namespace scope for this CLI request
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
  -N, --target-namespace string        Defines in which namespace the new PVC should be created. By default the source PVC's namespace is used.
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
  -y, --yes                            Skips confirmation if flag is set
```

## How does it work?

`rename-pvc` runs the following steps to rename an PVC in your Kubernetes cluster:

1. Creates the new PVC with the `.spec.volumeName` set to the existing PV
   - This new PVC is now in status `Lost`
2. Updates the `spec.claimRef` in the `PersistentVolume` to the new PVC
3. Waits until the new PVC's status is updated to `Bound`
4. Deletes the old PVC

## Maintainers

| Name                                                 | Email                           |
|:-----------------------------------------------------|:--------------------------------|
| [@dergeberl](https://github.com/dergeberl)           | maximilian.geberl@stackit.de    |
| [@einfachnuralex](https://github.com/einfachnuralex) | alexander.predeschly@stackit.de |

## Contribution

If you want to contribute to `rename-pvc` please have a look at our [contribution guidelines](CONTRIBUTING.md).
