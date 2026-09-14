// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package spec defines the intermediate representation consumed by all
// generators. It is produced once (from a protogen.Plugin) and then shared by
// the typed/scheme/register/meta/fake/informer/lister generators.
package spec

// SubresourceSpec describes a subresource such as "status" or "scale".
type SubresourceSpec struct {
	// Name is the subresource name, e.g. "status".
	Name string
	// Path is the URL path segment; defaults to Name.
	Path string
	// Verbs generated for this subresource; defaults to get,update.
	Verbs []string
	// Kind is the result kind; empty means the parent kind.
	Kind string
}

// FieldSpec describes a single business field of a resource message (i.e. any
// top-level field other than apiVersion/kind/metadata). It is used by the
// applyconfiguration generator.
type FieldSpec struct {
	// GoName is the Go field name, e.g. "Spec".
	GoName string
	// GoType is the unqualified Go type used in the apply configuration field
	// declaration, e.g. "string" for a scalar or "*DeploymentSpec" for a
	// message.
	GoType string
	// ProtoName is the proto field name, used for the JSON tag.
	ProtoName string
	// IsMessage is true when the field is a message (or map/slice whose value
	// is a message), requiring a package-qualified type reference.
	IsMessage bool
}

// ResourceSpec describes a single REST resource and its client shape.
type ResourceSpec struct {
	// Kind is the Go type name, e.g. "Deployment".
	Kind string
	// Plural is the REST plural name, e.g. "deployments".
	Plural string
	// Singular is the REST singular name, e.g. "deployment".
	Singular string
	// Namespaced reports whether the resource is namespace-scoped.
	Namespaced bool
	// Verbs is the verb set to generate (get/list/watch/create/update/...).
	Verbs []string
	// Subresources lists subresources such as status or scale.
	Subresources []SubresourceSpec
	// HasStatus is true when the message has a status field (drives
	// UpdateStatus/ApplyStatus).
	HasStatus bool
	// NoStatus suppresses status verbs even when HasStatus is true.
	NoStatus bool
	// ListKind is the Go type name of the list type, e.g. "DeploymentList".
	ListKind string

	// Field names (Go field names on the generated message) used by the meta
	// adapter. The resource message MUST carry these conventional top-level
	// fields.
	APIVersionField string // e.g. "ApiVersion"
	KindField       string // e.g. "Kind"
	MetadataField   string // e.g. "Metadata"

	// Fields are the business fields (excluding apiVersion/kind/metadata),
	// used by the applyconfiguration generator.
	Fields []FieldSpec
}

// VersionSpec groups resources by API version.
type VersionSpec struct {
	// Group is the short API group name, e.g. "apps". Empty means core.
	Group string
	// Version is the API version, e.g. "v1".
	Version string
	// GoPackage is the import path of the API types package (from go_package).
	GoPackage string
	// GoPackageName is the package name (the part after the last ';' or path
	// base), e.g. "v1".
	GoPackageName string
	// ProtoDir is the directory of the resource .proto file, used to place
	// generated files next to the .pb.go (for paths=source_relative). In
	// paths=import mode this equals the go_package import path directory.
	ProtoDir string
	// Resources are the resources in this group/version.
	Resources []*ResourceSpec
}

// GroupSpec groups versions by API group.
type GroupSpec struct {
	// Group is the short API group name, e.g. "apps".
	Group string
	// GroupGoName is the Go-facing group name, e.g. "Apps".
	GroupGoName string
	// Versions are the versions in this group, sorted with the preferred
	// version first.
	Versions []*VersionSpec
}

// Spec is the aggregated intermediate representation consumed by generators.
type Spec struct {
	// ClientsetName is the clientset name from the anchor option, e.g.
	// "exampleclient".
	ClientsetName string
	// ClientsetGoName is the exported Go identifier for the clientset, e.g.
	// "Exampleclient".
	ClientsetGoName string
	// ClientsetAPIPath is the API path, "/apis" (or "/api" for core).
	ClientsetAPIPath string
	// PreferProtobuf prefers protobuf encoding over JSON.
	PreferProtobuf bool
	// ClientsetGoPackage is the import path of the generated clientset package
	// (the anchor file's go_package).
	ClientsetGoPackage string
	// ClientsetPackageName is the package name of the clientset (the go_package
	// suffix, or the last path segment).
	ClientsetPackageName string
	// ClientsetProtoDir is the directory of the anchor clientset .proto file,
	// used as the output directory for the clientset and its typed/scheme
	// subtrees.
	ClientsetProtoDir string
	// Groups are the aggregated groups.
	Groups []*GroupSpec
	// Meshes are the gRPC services flagged for onexmesh service-discovery
	// code generation.
	Meshes []*FileMesh
}

// MeshSpec captures the onexmesh service-discovery option for a file.
type MeshSpec struct {
	// Enabled reports whether service discovery is on (enable_service_discovery).
	Enabled bool
	// ServiceName is the logical service name, e.g. "edu.course.student-api".
	ServiceName string
	// Registry is the backend type: "polaris", "etcd" or "kubernetes".
	Registry string
	// Protocol is "grpc" or "http".
	Protocol string
}

// FileMesh describes the mesh client to generate for one gRPC service.
type FileMesh struct {
	// GoImportPath is the API types package import path (from go_package).
	GoImportPath string
	// PackageName is the package name of the API types package.
	PackageName string
	// ServiceName is the gRPC service Go name, e.g. "DeploymentService".
	ServiceName string
	// ClientName is the generated gRPC client interface name.
	ClientName string
	// NewClientFn is the protoc-gen-go-grpc constructor, e.g.
	// "NewDeploymentServiceClient".
	NewClientFn string
	// ProtoDir is the directory of the .proto file, used to place generated
	// files next to the .pb.go.
	ProtoDir string
	// Mesh holds the service-discovery options.
	Mesh MeshSpec
}
