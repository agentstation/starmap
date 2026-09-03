// Package deploymentdocs verifies the deployment manifests that the maintained
// documentation publishes. It holds tests only. The tests parse the Kubernetes
// example in docs/DOCKER.md with the module YAML dependency, so a structural
// break in that example fails a repository gate.
package deploymentdocs
