package render

// This file contains the structures stolen from metallb config
// package to ease the rendering

type configFile struct {
	Peers          []peer            `yaml:"peers,omitempty"`
	BGPCommunities map[string]string `yaml:"bgp-communities,omitempty"` // TODO this is missing from crds
	Pools          []addressPool     `yaml:"address-pools,omitempty"`
	BFDProfiles    []bfdProfile      `yaml:"bfd-profiles,omitempty"`
}

type peer struct {
	MyASN         uint32         `yaml:"my-asn"`
	ASN           uint32         `yaml:"peer-asn"`
	Addr          string         `yaml:"peer-address"`
	SrcAddr       string         `yaml:"source-address,omitempty"`
	Port          uint16         `yaml:"peer-port,omitempty"`
	HoldTime      string         `yaml:"hold-time,omitempty"`
	RouterID      string         `yaml:"router-id,omitempty"`
	NodeSelectors []nodeSelector `yaml:"node-selectors,omitempty"`
	Password      string         `yaml:"password,omitempty"`
	BFDProfile    string         `yaml:"bfd-profile,omitempty"`
}

type nodeSelector struct {
	MatchLabels      map[string]string      `yaml:"match-labels,omitempty"`
	MatchExpressions []selectorRequirements `yaml:"match-expressions,omitempty"`
}

type selectorRequirements struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values"`
}

type addressPool struct {
	Protocol          Proto
	Name              string
	Addresses         []string
	AvoidBuggyIPs     bool               `yaml:"avoid-buggy-ips,omitempty"`
	AutoAssign        *bool              `yaml:"auto-assign,omitempty"`
	BGPAdvertisements []bgpAdvertisement `yaml:"bgp-advertisements,omitempty"`
}

type bgpAdvertisement struct {
	AggregationLength   *int32 `yaml:"aggregation-length"`
	AggregationLengthV6 *int32 `yaml:"aggregation-length-v6"`
	LocalPref           *uint32
	Communities         []string
}

type bfdProfile struct {
	Name                 string  `yaml:"name"`
	ReceiveInterval      *uint32 `yaml:"receive-interval,omitempty"`
	TransmitInterval     *uint32 `yaml:"transmit-interval,omitempty"`
	DetectMultiplier     *uint32 `yaml:"detect-multiplier,omitempty"`
	EchoReceiveInterval  *string `yaml:"echo-receive-interval,omitempty"`
	EchoTransmitInterval *uint32 `yaml:"echo-transmit-interval,omitempty"`
	EchoMode             *bool   `yaml:"echo-mode,omitempty"`
	PassiveMode          *bool   `yaml:"passive-mode,omitempty"`
	MinimumTTL           *uint32 `yaml:"minimum-ttl,omitempty"`
}
