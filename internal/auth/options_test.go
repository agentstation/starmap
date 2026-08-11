package auth

import "github.com/agentstation/starmap/pkg/catalogs"

func withCredentialSource(source credentialSource) ResolverOption {
	return func(resolver *Resolver) {
		if source != nil {
			resolver.sources[source.Backend()] = source
		}
	}
}

func withCloudChain(
	primitive catalogs.ProviderAuthenticationPrimitive,
	chain cloudChain,
) ResolverOption {
	return func(resolver *Resolver) {
		if chain != nil {
			resolver.cloudChains[primitive] = chain
		}
	}
}
