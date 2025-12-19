package wireguard

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pion/webrtc/v4"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func generateTurnCredentials(turnCreds *TurnCredentials) webrtc.ICEServer {
	return webrtc.ICEServer{
		Username:       turnCreds.Username,
		Credential:     turnCreds.Password,
		CredentialType: webrtc.ICECredentialTypePassword,
		URLs:           turnCreds.URLs,
	}
}

func generateKeyPair() (wgtypes.Key, wgtypes.Key, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, wgtypes.Key{}, err
	}
	return privateKey, privateKey.PublicKey(), nil
}

func createWebrtcPeerConnection(turnCreds *TurnCredentials) (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			generateTurnCredentials(turnCreds),
		},
	}
	// utils.Dump("WebRTC config:", config)
	return webrtc.NewPeerConnection(config)
}

func createTunNameFromPeerID(peerID string) string {
	// Create a SHA256 hash of the peer ID to ensure uniqueness
	// Take first 8 hex characters for a compact but collision-resistant name
	hash := sha256.Sum256([]byte(peerID))
	hashStr := fmt.Sprintf("%x", hash[:4]) // First 4 bytes = 8 hex chars
	return fmt.Sprintf("arqut-%s", hashStr)
}

func forceCleanupTUNInterface(name string) error {
	// Attempt to remove interface using ip command
	// This handles cases where the interface exists but is not properly released
	if runtime.GOOS == "linux" {
		cmd := exec.Command("ip", "link", "delete", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Ignore "cannot find device" errors as interface may already be gone
			if !strings.Contains(string(output), "Cannot find device") {
				return fmt.Errorf("failed to cleanup interface %s: %w\nOutput: %s", name, err, string(output))
			}
		}
		log.Printf("WG Manager: Force cleaned up stale interface %s", name)
	}
	return nil
}

func cleanupStaleWireGuardInterfaces() {
	if runtime.GOOS != "linux" {
		return
	}

	// List all network interfaces and find WireGuard ones
	cmd := exec.Command("ip", "link", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("WG Manager: Failed to list interfaces for cleanup: %v", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Look for lines like "3: arqut-8ad791cb: <POINTOPOINT,NOARP> mtu 1420 qdisc noop state DOWN mode DEFAULT group default"
		if strings.Contains(line, ": arqut-") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				ifaceName := strings.TrimSpace(parts[1])
				if strings.HasPrefix(ifaceName, "arqut-") {
					log.Printf("WG Manager: Cleaning up stale interface %s", ifaceName)
					forceCleanupTUNInterface(ifaceName)
				}
			}
		}
	}
}

func copyStruct(src, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
