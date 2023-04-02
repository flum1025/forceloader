package forceloader

func SetIgnoreResolverStructs(val string) {
	ignoreResolverStructs = &val
}

func SetResolverStruct(val string) {
	resolverStruct = &val
}

func SetRestrictedPackages(val string) {
	restrictedPackages = &val
}
