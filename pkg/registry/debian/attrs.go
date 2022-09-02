package debian

import "go.opentelemetry.io/otel/attribute"

func attrDist(dist string) attribute.KeyValue {
	return attribute.String("debian,dist", dist)
}

func attrArchitecture(arch Architecture) attribute.KeyValue {
	return attribute.String("debian.arch", string(arch))
}

func attrComponent(component string) attribute.KeyValue {
	return attribute.String("debian.component", component)
}

func attrComponents(components []string) attribute.KeyValue {
	return attribute.StringSlice("debian.components", components)
}

func attrPackageCount(count int) attribute.KeyValue {
	return attribute.Int("debian.package.count", count)
}
