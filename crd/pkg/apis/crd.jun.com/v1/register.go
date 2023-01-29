// +groupName=crd.jun.com
package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	Schema = runtime.NewScheme()
	GroupVersion = schema.GroupVersion{"cr.jun.com", "v1"}
	Codec = serializer.NewCodecFactory(Schema)

)