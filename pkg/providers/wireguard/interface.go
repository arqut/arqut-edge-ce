package wireguard

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

// createTUNInterface is a placeholder function in the common file.
// The actual implementation is in the platform-specific files.
func createTUNInterface(name string, addr string) (tun.Device, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	log.Printf("WG Manager: create TUN device %s on address %s", name, addr)

	dev, err := tun.CreateTUN(name, 1420)
	if err != nil {
		return nil, err
	}

	ifaceName, err := dev.Name()
	if err != nil {
		return nil, fmt.Errorf("failed to get TUN device name: %w", err)
	}

	// Wait a moment for the interface to be fully available in the OS
	time.Sleep(200 * time.Millisecond)

	switch runtime.GOOS {
	case "linux":
		if err := configureLinux(ifaceName, addr); err != nil {
			dev.Close()
			return nil, err
		}
	case "windows":
		if err := configureWindows(ifaceName, addr); err != nil {
			dev.Close()
			return nil, err
		}
	}

	return dev, nil
}

// runCommand executes a command and returns an error with stdout/stderr if it fails.
func runCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command '%s %s' failed: %w\nOutput: %s", name, strings.Join(arg, " "), err, string(output))
	}
	return nil
}

// --- Platform-Specific Configurations ---

func configureLinux(name string, addr string) error {
	if err := runCommand("ip", "address", "add", addr+"/24", "dev", name); err != nil {
		return fmt.Errorf("failed to add IP address: %w", err)
	}
	if err := runCommand("ip", "link", "set", "dev", name, "up"); err != nil {
		return fmt.Errorf("failed to set link up: %w", err)
	}
	return nil
}

func configureWindows(name string, addr string) error {
	// The wireguard-go library for windows handles this via its LUID based configuration.
	// We just need to assign the address.
	// ip, _, err := net.ParseCIDR(addr)
	// if err != nil {
	// 	return fmt.Errorf("invalid CIDR address: %w", err)
	// }

	// Example using netsh. This requires admin privileges, which we want to avoid.
	// The userspace implementation should handle this, but if not, this is the fallback.
	// For non-admin, the expectation is that the WinTUN driver setup handles this.
	// The library's internal `WintunSetAdapterAddresses` should be called. Since we can't
	// call it directly, we rely on the library's behavior. If IP configuration fails,
	// it indicates a potential permissions or setup issue with the WinTUN driver.

	// We will rely on wireguard-go's internal setup. If direct configuration is needed
	// and we must remain non-admin, a different approach (like a helper service with
	// permissions) would be required. This implementation assumes the library handles it.

	// Let's try to configure it with netsh, but log a warning about admin rights.
	log.Print("On Windows, automatic IP configuration might require administrator privileges if the WinTUN driver is not properly configured.")
	err := runCommand("netsh", "interface", "ip", "set", "address", fmt.Sprintf("name=\"%s\"", name), "source=static", fmt.Sprintf("addr=%s", addr), "mask=255.255.255.0")
	if err != nil {
		log.Print("Failed to configure IP with netsh. Please configure it manually or run as admin.", "error", err)
		return fmt.Errorf("netsh configuration failed, manual setup may be required: %w", err)
	}

	return nil
}
