package debian

import "go.opentelemetry.io/otel/attribute"

var (
	attrPackageCount = attribute.Key("package_count")
)

func attrDist(dist string) attribute.KeyValue {
	return attribute.String("debian_dist", dist)
}

func attrArchitecture(arch Architecture) attribute.KeyValue {
	return attribute.String("debian_arch", string(arch))
}

func attrComponent(component string) attribute.KeyValue {
	return attribute.String("debian_component", component)
}

func attrComponents(components []string) attribute.KeyValue {
	return attribute.StringSlice("debian_components", components)
}
