package parser

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// ConvertNodesToClashMeta converts given parsed nodes into standard Clash.Meta / Mihomo YAML format
func ConvertNodesToClashMeta(nodes []ParsedNode) (string, error) {
	if len(nodes) == 0 {
		return "", fmt.Errorf("no proxy nodes to convert")
	}

	var sb strings.Builder
	sb.WriteString("port: 7890\n")
	sb.WriteString("socks-port: 7891\n")
	sb.WriteString("allow-lan: true\n")
	sb.WriteString("mode: rule\n")
	sb.WriteString("log-level: info\n\n")
	sb.WriteString("proxies:\n")

	for _, n := range nodes {
		server := n.Server
		if server == "" {
			continue
		}
		port := n.Port
		if port <= 0 {
			port = 443
		}

		name := strings.ReplaceAll(n.Name, ":", "-")
		if name == "" {
			name = fmt.Sprintf("Proxy-%s", server)
		}

		raw := n.RawURI
		if strings.HasPrefix(raw, "vless://") {
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
			sb.WriteString("    type: vless\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    port: %d\n", port))
			sb.WriteString(fmt.Sprintf("    uuid: %s\n", n.Username))
			sb.WriteString("    udp: true\n")
			if strings.Contains(raw, "security=reality") {
				sb.WriteString("    tls: true\n")
			} else if strings.Contains(raw, "security=tls") {
				sb.WriteString("    tls: true\n")
			}
		} else if strings.HasPrefix(raw, "vmess://") {
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
			sb.WriteString("    type: vmess\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    port: %d\n", port))
			sb.WriteString(fmt.Sprintf("    uuid: %s\n", n.Username))
			sb.WriteString("    alterId: 0\n")
			sb.WriteString("    cipher: auto\n")
			sb.WriteString("    udp: true\n")
		} else if strings.HasPrefix(raw, "trojan://") {
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
			sb.WriteString("    type: trojan\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    port: %d\n", port))
			sb.WriteString(fmt.Sprintf("    password: \"%s\"\n", n.Password))
			sb.WriteString("    udp: true\n")
		} else if strings.HasPrefix(raw, "ss://") {
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
			sb.WriteString("    type: ss\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    port: %d\n", port))
			sb.WriteString("    cipher: chacha20-ietf-poly1305\n")
			sb.WriteString(fmt.Sprintf("    password: \"%s\"\n", n.Password))
			sb.WriteString("    udp: true\n")
		} else if strings.HasPrefix(raw, "hysteria2://") || strings.HasPrefix(raw, "hy2://") {
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
			sb.WriteString("    type: hysteria2\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    port: %d\n", port))
			sb.WriteString(fmt.Sprintf("    password: \"%s\"\n", n.Password))
		}
	}

	return sb.String(), nil
}

// ConvertNodesToSingBoxJSON converts nodes into Sing-Box JSON format
func ConvertNodesToSingBoxJSON(nodes []ParsedNode) (string, error) {
	if len(nodes) == 0 {
		return "", fmt.Errorf("no nodes to convert")
	}

	var outbounds []map[string]interface{}
	for _, n := range nodes {
		if n.Outbound != nil && len(n.Outbound) > 0 {
			outbounds = append(outbounds, n.Outbound)
		}
	}

	if len(outbounds) == 0 {
		return "", fmt.Errorf("no Sing-box outbounds could be generated")
	}

	doc := map[string]interface{}{
		"outbounds": outbounds,
	}

	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ConvertRawInputToFormat parses raw text links and outputs into target format
func ConvertRawInputToFormat(rawInput, targetFormat string) (string, error) {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return "", fmt.Errorf("input cannot be empty")
	}

	// If Base64 encoded payload, decode first
	if decoded, err := base64.StdEncoding.DecodeString(rawInput); err == nil && len(decoded) > 0 {
		rawInput = string(decoded)
	}

	var nodes []ParsedNode
	scanner := bufio.NewScanner(strings.NewReader(rawInput))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if node, err := ParseURI(line); err == nil && node != nil {
			nodes = append(nodes, *node)
		}
	}

	if len(nodes) == 0 {
		return "", fmt.Errorf("no valid proxy URIs found in input")
	}

	switch strings.ToLower(targetFormat) {
	case "clash", "clash_meta", "yaml":
		return ConvertNodesToClashMeta(nodes)
	case "singbox", "singbox_json", "json":
		return ConvertNodesToSingBoxJSON(nodes)
	case "base64", "sub", "subscription":
		var lines []string
		for _, n := range nodes {
			if n.RawURI != "" {
				lines = append(lines, strings.TrimSpace(n.RawURI))
			}
		}
		raw := strings.Join(lines, "\n")
		return base64.StdEncoding.EncodeToString([]byte(raw)), nil
	case "links", "v2ray":
		var links []string
		for _, n := range nodes {
			if n.RawURI != "" {
				links = append(links, n.RawURI)
			}
		}
		return strings.Join(links, "\n"), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", targetFormat)
	}
}
