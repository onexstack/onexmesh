// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package spec

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	restv1 "github.com/onexstack/onexmesh/pkg/proto/onexmesh/rest/v1"
	onexmeshv1 "github.com/onexstack/onexmesh/pkg/proto/onexmesh/v1"
)

// defaultVerbs is the verb set generated when a Resource does not override it.
var defaultVerbs = []string{
	"get", "list", "watch", "create", "update", "patch", "delete",
}

// Load scans gen.Files and builds a Spec from the rest.v1 options declared on
// the anchor clientset file and the resource/list messages.
func Load(gen *protogen.Plugin) (*Spec, error) {
	spec := &Spec{ClientsetAPIPath: "/apis"}

	// key: group+"\x00"+version -> kind -> resource
	resources := map[string]map[string]*ResourceSpec{}
	// key: group+"\x00"+version -> API types package import path / name.
	goPackage := map[string]string{}
	goPackageName := map[string]string{}
	// key: group+"\x00"+version -> directory of the resource .proto file.
	protoDir := map[string]string{}

	for _, file := range gen.Files {
		opts := file.Proto.GetOptions()
		if opts == nil {
			continue
		}

		// File-level defaults for resources declared in this file.
		var fileRest *restv1.FileRest
		if proto.HasExtension(opts, restv1.E_File) {
			fileRest = proto.GetExtension(opts, restv1.E_File).(*restv1.FileRest)
		}

		// Anchor clientset declaration.
		if proto.HasExtension(opts, restv1.E_Clientset) {
			cs := proto.GetExtension(opts, restv1.E_Clientset).(*restv1.Clientset)
			if spec.ClientsetName != "" {
				return nil, fmt.Errorf("multiple clientset anchors: %q and %q", spec.ClientsetName, cs.GetName())
			}
			spec.ClientsetName = cs.GetName()
			spec.ClientsetGoName = UpperFirst(cs.GetName())
			spec.ClientsetGoPackage = string(file.GoImportPath)
			spec.ClientsetPackageName = packageNameOf(string(file.GoImportPath), string(file.GoPackageName))
			spec.ClientsetProtoDir = path.Dir(file.GeneratedFilenamePrefix)
			spec.PreferProtobuf = cs.GetPreferProtobuf()
			if cs.GetApiPath() != "" {
				spec.ClientsetAPIPath = cs.GetApiPath()
			}
		}

		for _, msg := range file.Messages {
			msgOpts, ok := msg.Desc.Options().(*descriptorpb.MessageOptions)
			if !ok || msgOpts == nil {
				continue
			}
			if !proto.HasExtension(msgOpts, restv1.E_Resource) {
				continue
			}
			rs, group, version, err := buildResource(msg, proto.GetExtension(msgOpts, restv1.E_Resource).(*restv1.Resource), fileRest)
			if err != nil {
				return nil, fmt.Errorf("message %s: %w", msg.GoIdent.GoName, err)
			}
			key := groupVersionKey(group, version)
			if resources[key] == nil {
				resources[key] = map[string]*ResourceSpec{}
			}
			resources[key][rs.Kind] = rs
			goPackage[key] = string(file.GoImportPath)
			goPackageName[key] = string(file.GoPackageName)
			protoDir[key] = path.Dir(file.GeneratedFilenamePrefix)
		}
	}

	// Pair list messages with their item resources. A list lives in the same
	// group/version as its item (both declared in the same file), so the item
	// kind is resolved within that group/version only.
	for _, file := range gen.Files {
		fileGroup, fileVersion := fileGroupVersion(file)

		for _, msg := range file.Messages {
			msgOpts, ok := msg.Desc.Options().(*descriptorpb.MessageOptions)
			if !ok || msgOpts == nil {
				continue
			}
			if !proto.HasExtension(msgOpts, restv1.E_List) {
				continue
			}
			itemKind := listItemKind(msg)
			if itemKind == "" {
				return nil, fmt.Errorf("list message %s: cannot infer item kind from items field", msg.GoIdent.GoName)
			}
			if err := pairList(msg, itemKind, fileGroup, fileVersion, resources); err != nil {
				return nil, err
			}
		}
	}

	if spec.ClientsetName == "" {
		return nil, fmt.Errorf("no clientset anchor found; add option (onexmesh.rest.v1.clientset) to an anchor .proto file")
	}

	// Scan for onexmesh service-discovery declarations. This is independent of
	// the REST clientset: files without the option are silently skipped.
	for _, file := range gen.Files {
		opts := file.Proto.GetOptions()
		if opts == nil || !proto.HasExtension(opts, onexmeshv1.E_MeshService) {
			continue
		}
		ms := proto.GetExtension(opts, onexmeshv1.E_MeshService).(*onexmeshv1.MeshService)
		if !ms.GetEnableServiceDiscovery() {
			continue
		}

		serviceName := ms.GetServiceName()
		if serviceName == "" {
			serviceName = inferServiceName(file)
		}
		registry := ms.GetRegistry()
		if registry == "" {
			registry = "polaris"
		}
		protocol := ms.GetProtocol()
		if protocol == "" {
			protocol = "grpc"
		}

		for _, svc := range file.Services {
			spec.Meshes = append(spec.Meshes, &FileMesh{
				GoImportPath: string(file.GoImportPath),
				PackageName:  string(file.GoPackageName),
				ServiceName:  svc.GoName,
				ClientName:   svc.GoName + "Client",
				NewClientFn:  "New" + svc.GoName + "Client",
				ProtoDir:     path.Dir(file.GeneratedFilenamePrefix),
				Mesh: MeshSpec{
					Enabled:     true,
					ServiceName: serviceName,
					Registry:    registry,
					Protocol:    protocol,
				},
			})
		}
	}

	spec.Groups = assembleGroups(resources, goPackage, goPackageName, protoDir)
	return spec, nil
}

// inferServiceName derives a default logical service name from the proto
// package, e.g. "edu.course.student" for package "edu.course.student".
func inferServiceName(file *protogen.File) string {
	return string(file.Proto.GetPackage())
}

// buildResource builds a ResourceSpec from a resource message, its Resource
// option and the file-level defaults. It returns the resolved group/version.
func buildResource(msg *protogen.Message, r *restv1.Resource, fileRest *restv1.FileRest) (*ResourceSpec, string, string, error) {
	rs := &ResourceSpec{
		Kind:       msg.GoIdent.GoName,
		Singular:   strings.ToLower(msg.GoIdent.GoName),
		Namespaced: true,
	}

	group, version := "", ""
	if fileRest != nil {
		group, version = fileRest.GetGroup(), fileRest.GetVersion()
	}
	if r.GetGroup() != "" {
		group = r.GetGroup()
	}
	if r.GetVersion() != "" {
		version = r.GetVersion()
	}
	if version == "" {
		return nil, "", "", fmt.Errorf("missing API version (set file-level or message-level rest.v1 option)")
	}

	if r.GetPlural() != "" {
		rs.Plural = r.GetPlural()
	} else {
		rs.Plural = Pluralize(rs.Singular)
	}
	if r.GetSingular() != "" {
		rs.Singular = r.GetSingular()
	}
	if r.Namespaced != nil {
		rs.Namespaced = *r.Namespaced
	}
	rs.NoStatus = r.GetNoStatus()

	if len(r.GetVerbs()) > 0 {
		rs.Verbs = append([]string(nil), r.GetVerbs()...)
	} else {
		rs.Verbs = append([]string(nil), defaultVerbs...)
	}
	if hasStatusField(msg) && !rs.NoStatus {
		rs.HasStatus = true
		rs.Verbs = append(rs.Verbs, "updateStatus")
	}

	// Record the Go field names for the conventional top-level fields.
	rs.APIVersionField = fieldGoName(msg, "apiVersion", "ApiVersion")
	rs.KindField = fieldGoName(msg, "kind", "Kind")
	rs.MetadataField = fieldGoName(msg, "metadata", "Metadata")

	// Record business fields (excluding the conventional metadata fields) for
	// the applyconfiguration generator.
	rs.Fields = businessFields(msg, rs.APIVersionField, rs.KindField, rs.MetadataField)

	for _, sr := range r.GetSubresources() {
		sub := SubresourceSpec{
			Name:  sr.GetName(),
			Path:  sr.GetPath(),
			Verbs: sr.GetVerbs(),
			Kind:  sr.GetKind(),
		}
		if sub.Path == "" {
			sub.Path = sub.Name
		}
		if len(sub.Verbs) == 0 {
			sub.Verbs = []string{"get", "update"}
		}
		rs.Subresources = append(rs.Subresources, sub)
	}

	return rs, group, version, nil
}

// hasStatusField reports whether the message has a field named "status".
func hasStatusField(msg *protogen.Message) bool {
	for _, f := range msg.Fields {
		if f.GoName == "Status" {
			return true
		}
	}
	return false
}

// fieldGoName returns the Go field name for the conventional field whose proto
// name is protoName, falling back to defaultName when absent.
func fieldGoName(msg *protogen.Message, protoName, defaultName string) string {
	for _, f := range msg.Fields {
		if string(f.Desc.Name()) == protoName {
			return f.GoName
		}
	}
	return defaultName
}

// businessFields returns the FieldSpecs for all fields of msg except the three
// conventional metadata fields (apiVersion/kind/metadata).
func businessFields(msg *protogen.Message, apiVersionField, kindField, metadataField string) []FieldSpec {
	var out []FieldSpec
	for _, f := range msg.Fields {
		if f.GoName == apiVersionField || f.GoName == kindField || f.GoName == metadataField {
			continue
		}
		fs := FieldSpec{
			GoName:    f.GoName,
			ProtoName: string(f.Desc.Name()),
			GoType:    goTypeOf(f),
			IsMessage: f.Message != nil,
		}
		out = append(out, fs)
	}
	return out
}

// goTypeOf returns the Go type used to declare an apply-configuration field for
// the given message field. Message fields reference the (pointer) proto type
// directly; scalars use their Go type.
func goTypeOf(f *protogen.Field) string {
	if f.Message != nil {
		return "*" + f.Message.GoIdent.GoName
	}
	if f.Enum != nil {
		return f.Enum.GoIdent.GoName
	}
	if f.Desc.IsMap() {
		// Map fields: use the Go map type. We only need a pointer-worthy
		// declaration; a nil map already distinguishes set/unset.
		return mapTypeOf(f)
	}
	if f.Desc.IsList() {
		return "[]" + scalarGoType(f.Desc.Kind())
	}
	return scalarGoType(f.Desc.Kind())
}

// scalarGoType maps a protoreflect scalar kind to its Go type.
func scalarGoType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return "string"
	}
}

// mapTypeOf returns the Go map type for a map field, e.g.
// "map[string]string" or "map[string]int32".
func mapTypeOf(f *protogen.Field) string {
	key := scalarGoType(f.Desc.MapKey().Kind())
	val := scalarGoType(f.Desc.MapValue().Kind())
	if f.Desc.MapValue().Kind() == protoreflect.MessageKind || f.Desc.MapValue().Kind() == protoreflect.GroupKind {
		val = "interface{}"
	}
	return "map[" + key + "]" + val
}

// listItemKind returns the Go type name of the item message from the first
// repeated message field, or "" if not found.
func listItemKind(msg *protogen.Message) string {
	for _, f := range msg.Fields {
		if f.Desc.IsList() && f.Message != nil {
			return f.Message.GoIdent.GoName
		}
	}
	return ""
}

// pairList links a list message to its item resource within the given
// group/version, setting ListKind on the ResourceSpec.
func pairList(listMsg *protogen.Message, itemKind, group, version string, resources map[string]map[string]*ResourceSpec) error {
	byKind := resources[groupVersionKey(group, version)]
	rs, ok := byKind[itemKind]
	if !ok {
		return fmt.Errorf("list message %s: item kind %q has no matching resource in group/version %q/%q", listMsg.GoIdent.GoName, itemKind, group, version)
	}
	rs.ListKind = listMsg.GoIdent.GoName
	return nil
}

// fileGroupVersion returns the (group, version) declared by a file's
// file-level rest.v1 option, or empty strings if unset.
func fileGroupVersion(file *protogen.File) (string, string) {
	opts := file.Proto.GetOptions()
	if opts == nil {
		return "", ""
	}
	x := proto.GetExtension(opts, restv1.E_File)
	if x == nil {
		return "", ""
	}
	fr := x.(*restv1.FileRest)
	return fr.GetGroup(), fr.GetVersion()
}

// assembleGroups groups resources by (group, version), returning groups sorted
// deterministically (group name asc, version asc).
func assembleGroups(resources map[string]map[string]*ResourceSpec, goPackage, goPackageName, protoDir map[string]string) []*GroupSpec {
	type vk struct{ group, version string }
	versions := map[vk]*VersionSpec{}

	for key, byKind := range resources {
		g, v := splitGroupVersionKey(key)
		k := vk{g, v}
		vs := versions[k]
		if vs == nil {
			vs = &VersionSpec{
				Group:         g,
				Version:       v,
				GoPackage:     goPackage[key],
				GoPackageName: packageNameOf(goPackage[key], goPackageName[key]),
				ProtoDir:      protoDir[key],
			}
			versions[k] = vs
		}
		for _, rs := range byKind {
			vs.Resources = append(vs.Resources, rs)
		}
		// Deterministic resource order within a version.
		sort.Slice(vs.Resources, func(i, j int) bool { return vs.Resources[i].Kind < vs.Resources[j].Kind })
	}

	groups := map[string]*GroupSpec{}
	for k, vs := range versions {
		gs := groups[k.group]
		if gs == nil {
			gs = &GroupSpec{Group: k.group, GroupGoName: ToGroupGoName(k.group)}
			groups[k.group] = gs
		}
		gs.Versions = append(gs.Versions, vs)
	}

	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)

	out := make([]*GroupSpec, 0, len(names))
	for _, g := range names {
		gs := groups[g]
		sort.Slice(gs.Versions, func(i, j int) bool { return gs.Versions[i].Version < gs.Versions[j].Version })
		out = append(out, gs)
	}
	return out
}

func groupVersionKey(group, version string) string {
	return group + "\x00" + version
}

func splitGroupVersionKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}

func packageNameOf(importPath, declaredName string) string {
	if declaredName != "" {
		return declaredName
	}
	for i := len(importPath) - 1; i >= 0; i-- {
		if importPath[i] == '/' {
			return importPath[i+1:]
		}
	}
	return importPath
}
