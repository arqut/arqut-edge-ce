package haaddon

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/arqut/arqut-edge-ce/pkg/utils"
	"github.com/gofiber/fiber/v2/log"
	"gopkg.in/yaml.v3"
)

const configPath = "/haconfig/configuration.yaml"

// GetNetworkSubnets returns the network subnets that will be added as trusted proxies
func GetNetworkSubnets() ([]string, error) {
	ips, err := utils.GetLocalIPs(false)
	if err != nil {
		return nil, fmt.Errorf("could not detect local IPs: %w", err)
	}

	var subnets []string

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}

		// -- IPv4 → /24
		if ip4 := ip.To4(); ip4 != nil {
			subnets = append(subnets, to24(ip4))
			continue
		}

		// -- IPv6 --
		//  a) link-local? skip
		if ip.IsLinkLocalUnicast() {
			continue
		}
		//  b) otherwise treat as /64
		subnets = append(subnets, to64(ip))
	}

	return subnets, nil
}

// UpdateHAConfig updates the Home Assistant configuration to trust the edge proxy
func UpdateHAConfig() {
	// 1) Detect all container IPs
	ips, err := utils.GetLocalIPs(false)
	if err != nil {
		log.Errorf("Could not detect local IPs: %v", err)
		return
	}

	var subnets []string

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			log.Warnf("Skipping unparsable IP: %s", ipStr)
			continue
		}

		// -- IPv4 → /24
		if ip4 := ip.To4(); ip4 != nil {
			subnets = append(subnets, to24(ip4))
			continue
		}

		// -- IPv6 --
		//  a) link-local? skip
		if ip.IsLinkLocalUnicast() {
			continue
		}
		//  b) otherwise treat as /64
		subnets = append(subnets, to64(ip))
	}

	if len(subnets) == 0 {
		log.Error("No valid local subnets found")
		return
	}

	// 2) Update HA config for each subnet
	for _, subnet := range subnets {
		log.Infof("Adding trusted proxy subnet: %s", subnet)
		if err := ensureTrustedProxySubnet(configPath, subnet); err != nil {
			log.Errorf("Failed to update %s with %s: %v", configPath, subnet, err)
		}
	}
}

func to64(ip net.IP) string {
	// Use the 16-byte representation to extract the first 64 bits
	ip16 := ip.To16()
	if ip16 == nil {
		return ip.String() + "/64"
	}
	// Zero out the last 8 bytes (host portion of /64)
	masked := make(net.IP, 16)
	copy(masked, ip16[:8])
	// Format as CIDR - this handles :: compression correctly
	ipNet := net.IPNet{IP: masked, Mask: net.CIDRMask(64, 128)}
	return ipNet.String()
}

// to24 zero-out the last octet and add /24
func to24(ip net.IP) string {
	ip = ip.Mask(net.CIDRMask(24, 32))
	return fmt.Sprintf("%s/24", ip.String())
}

func ensureTrustedProxySubnet(path, subnet string) error {
	orig, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	updated, err := patchHTTPSection(orig, subnet)
	if err != nil {
		return err
	}

	beautified := addBlankAroundSection(updated, "http")

	if bytes.Equal(beautified, orig) {
		return nil // idempotent – nothing to do
	}
	return os.WriteFile(path, beautified, 0o644)
}

// patchHTTPSection manipulates the YAML via yaml.Node (tags preserved).
func patchHTTPSection(data []byte, subnet string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("unexpected YAML document structure")
	}
	root := doc.Content[0] // root mapping node

	// --- 1. find or create the http: mapping ---------------------------------
	var httpKey, httpVal *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == "http" {
			httpKey = k
			httpVal = root.Content[i+1]
			break
		}
	}
	if httpKey == nil {
		httpKey = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "http"}
		httpVal = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, httpKey, httpVal)
	}
	if httpVal.Kind != yaml.MappingNode {
		// overwrite anything unexpected with a fresh mapping
		httpVal = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		replaceValueNode(root, httpKey, httpVal)
	}

	// --- 2. ensure use_x_forwarded_for: true ---------------------------------
	setScalarInMapping(httpVal, "use_x_forwarded_for", "!!bool", "true")

	// --- 3. ensure subnet in trusted_proxies ---------------------------------
	proxiesNode := ensureSequenceInMapping(httpVal, "trusted_proxies")
	if !sequenceContains(proxiesNode, subnet) {
		proxiesNode.Content = append(proxiesNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: subnet})
	}

	// --- 4. encode back to bytes ---------------------------------------------
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // keeps file neat; tags preserved automatically
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	enc.Close()
	return buf.Bytes(), nil
}

// -----------------------------------------------------------------------------
// Helper functions for yaml.Node manipulation
// -----------------------------------------------------------------------------

// replaceValueNode substitutes the mapping value for the given key node.
func replaceValueNode(mapping *yaml.Node, key, newVal *yaml.Node) {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i] == key {
			mapping.Content[i+1] = newVal
			return
		}
	}
}

// setScalarInMapping sets or creates a scalar k/v inside a mapping node.
func setScalarInMapping(mapping *yaml.Node, key, tag, value string) {
	for i := 0; i < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			v := mapping.Content[i+1]
			v.Kind, v.Tag, v.Value = yaml.ScalarNode, tag, value
			return
		}
	}
	// not found – append
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value},
	)
}

// ensureSequenceInMapping returns the sequence node at key, creating it if absent.
func ensureSequenceInMapping(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			val := mapping.Content[i+1]
			if val.Kind == yaml.SequenceNode {
				return val
			}
			break // key exists but wrong type – fall through to recreate
		}
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		seq,
	)
	return seq
}

// sequenceContains reports whether the sequence node already has the string v.
func sequenceContains(seq *yaml.Node, v string) bool {
	for _, n := range seq.Content {
		if n.Kind == yaml.ScalarNode && n.Value == v {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Cosmetic – add a blank line before and after a root‑level section
// -----------------------------------------------------------------------------

// addBlankAroundSection works on the encoded YAML bytes (tags preserved).
func addBlankAroundSection(yamlContent []byte, section string) []byte {
	lines := strings.Split(strings.TrimRight(string(yamlContent), "\n"), "\n")

	// locate root‑level "section:" line
	secIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == section+":" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			secIdx = i
			break
		}
	}
	if secIdx == -1 {
		return yamlContent // section not found – should not happen
	}

	// ensure blank line BEFORE
	if secIdx == 0 || strings.TrimSpace(lines[secIdx-1]) != "" {
		lines = append(lines[:secIdx], append([]string{""}, lines[secIdx:]...)...)
		secIdx++
	}

	// find end of section block (next root‑level key or EOF)
	endIdx := len(lines) - 1
	for i := secIdx + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t") || strings.HasPrefix(trim, "#") {
			continue
		}
		endIdx = i - 1
		break
	}

	// ensure blank line AFTER
	if endIdx == len(lines)-1 || strings.TrimSpace(lines[endIdx+1]) != "" {
		lines = append(lines[:endIdx+1], append([]string{""}, lines[endIdx+1:]...)...)
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}
