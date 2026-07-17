package discovery

// AllConnectors is the single, real, shared source of truth for every registered
// connector -- used by both the search command and the status command, so adding an 8th
// connector means updating this one list, not silently drifting between two separate,
// hand-maintained copies.
func AllConnectors() []Connector {
	return []Connector{
		ForceDreamConnector{},
		MCPRegistryConnector{},
		GitHubConnector{},
		NpmConnector{},
		SmitheryConnector{},
		WebConnector{},
		DockerHubConnector{},
		CratesIOConnector{},
		MavenCentralConnector{},
		NuGetConnector{},
		PackagistConnector{},
		RubyGemsConnector{},
		HexPMConnector{},
	}
}
