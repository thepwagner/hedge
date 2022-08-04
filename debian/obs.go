package debian

import "go.opentelemetry.io/otel/attribute"

var (
	attrArchitectures = attribute.Key("architectures")
	attrComponents    = attribute.Key("components")

	attrArchitecture = attribute.Key("architecture")
	attrComponent    = attribute.Key("component")

	attrDist         = attribute.Key("dist")
	attrPackageCount = attribute.Key("package_count")
)
