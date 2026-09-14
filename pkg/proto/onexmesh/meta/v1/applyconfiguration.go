// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package meshmeta

// This file contains the hand-written apply configuration types for the
// runtime metadata. They mirror the pointer-fielded shape used by
// k8s.io/client-go/applyconfigurations/meta/v1 so that generated resource
// apply configurations can embed them and satisfy gentype's namedObject
// constraint (comparable + GetName() *string).

// TypeMetaApplyConfiguration is the declarative configuration of TypeMeta.
type TypeMetaApplyConfiguration struct {
	// Kind of the resource.
	Kind *string `json:"kind,omitempty"`
	// APIVersion of the resource.
	APIVersion *string `json:"apiVersion,omitempty"`
}

// WithKind sets the Kind field in the declarative configuration to the given
// value and returns the receiver, so that objects can be built by chaining
// "With" function invocations.
func (b *TypeMetaApplyConfiguration) WithKind(value string) *TypeMetaApplyConfiguration {
	b.Kind = &value
	return b
}

// WithAPIVersion sets the APIVersion field in the declarative configuration to
// the given value and returns the receiver.
func (b *TypeMetaApplyConfiguration) WithAPIVersion(value string) *TypeMetaApplyConfiguration {
	b.APIVersion = &value
	return b
}

// ObjectMetaApplyConfiguration is the declarative configuration of the
// configurable subset of ObjectMeta (name, namespace, labels, annotations and
// finalizers). System-managed fields (uid, resourceVersion, generation,
// timestamps, ...) are intentionally excluded: they cannot be applied.
type ObjectMetaApplyConfiguration struct {
	// Name must be unique within a namespace.
	Name *string `json:"name,omitempty"`
	// GenerateName is an optional prefix used by the server to generate a
	// unique name ONLY IF the Name field has not been provided.
	GenerateName *string `json:"generateName,omitempty"`
	// Namespace defines the space within which each name must be unique.
	Namespace *string `json:"namespace,omitempty"`
	// Labels are key/value pairs attached to the object.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is an unstructured key/value map stored with the object.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Finalizers are an ordered list that must be empty before the object is
	// deleted from the registry.
	Finalizers []string `json:"finalizers,omitempty"`
}

// GetName returns the Name field value, or nil if unset. It satisfies the
// gentype.namedObject constraint.
func (b *ObjectMetaApplyConfiguration) GetName() *string {
	if b == nil {
		return nil
	}
	return b.Name
}

// WithName sets the Name field in the declarative configuration to the given
// value and returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithName(value string) *ObjectMetaApplyConfiguration {
	b.Name = &value
	return b
}

// WithGenerateName sets the GenerateName field in the declarative
// configuration to the given value and returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithGenerateName(value string) *ObjectMetaApplyConfiguration {
	b.GenerateName = &value
	return b
}

// WithNamespace sets the Namespace field in the declarative configuration to
// the given value and returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithNamespace(value string) *ObjectMetaApplyConfiguration {
	b.Namespace = &value
	return b
}

// WithLabels puts the entries into the Labels field in the declarative
// configuration, overwriting existing map entries with the same key, and
// returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithLabels(entries map[string]string) *ObjectMetaApplyConfiguration {
	if b.Labels == nil && len(entries) > 0 {
		b.Labels = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.Labels[k] = v
	}
	return b
}

// WithAnnotations puts the entries into the Annotations field in the
// declarative configuration, overwriting existing map entries with the same
// key, and returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithAnnotations(entries map[string]string) *ObjectMetaApplyConfiguration {
	if b.Annotations == nil && len(entries) > 0 {
		b.Annotations = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.Annotations[k] = v
	}
	return b
}

// WithFinalizers adds the given value to the Finalizers field in the
// declarative configuration and returns the receiver.
func (b *ObjectMetaApplyConfiguration) WithFinalizers(values ...string) *ObjectMetaApplyConfiguration {
	for i := range values {
		if values[i] == "" {
			panic("nil value passed to WithFinalizers")
		}
		b.Finalizers = append(b.Finalizers, values[i])
	}
	return b
}
