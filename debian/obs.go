package debian

import "go.opentelemetry.io/otel/attribute"

var (
	attrArchitecture = attribute.Key("architecture")

	attrDist         = attribute.Key("dist")
	attrPackageCount = attribute.Key("package_count")
)
