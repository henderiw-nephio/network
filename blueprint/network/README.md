# network

## Description
network controller

## Usage

### Fetch the package
`kpt pkg get REPO_URI[.git]/PKG_PATH[@VERSION] network`
Details: https://kpt.dev/reference/cli/pkg/get/

### View package content
`kpt pkg tree network`
Details: https://kpt.dev/reference/cli/pkg/tree/

### Apply the package
```
kpt live init network
kpt live apply network --reconcile-timeout=2m --output=table
```
Details: https://kpt.dev/reference/cli/live/
