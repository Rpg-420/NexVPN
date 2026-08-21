#!/bin/bash
# NexVPN Linux Setup Script
# This script applies necessary network capabilities and optimizations for Linux.
# Run this script with sudo or as root.

if [ "$EUID" -ne 0 ]; then
  echo "Please run this script as root (use sudo ./install_linux.sh)"
  exit 1
fi

echo "=========================================="
echo "    NexVPN Pro - Linux Optimization       "
echo "=========================================="

# 1. Apply Network Capabilities to avoid password prompts
echo "[*] Applying Network Capabilities (CAP_NET_ADMIN)..."

POSSIBLE_DIRS=("./bin" "./build/bin/bin" "./build/bin" "../bin" "../../bin")

apply_caps() {
    bin_name=$1
    found=false
    for dir in "${POSSIBLE_DIRS[@]}"; do
        if [ -f "$dir/$bin_name" ]; then
            setcap cap_net_admin,cap_net_bind_service=+ep "$dir/$bin_name"
            echo "  -> $bin_name (in $dir): capabilities applied."
            found=true
        fi
    done
    if [ "$found" = false ]; then
        echo "  -> $bin_name not found in any standard bin directory. Skipping."
    fi
}

apply_caps "sing-box"
apply_caps "openvpn"


# 2. Enable BBR TCP Congestion Control
echo "[*] Enabling TCP BBR Congestion Control..."
if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf; then
    echo "  -> BBR is already configured."
else
    echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
    sysctl -p > /dev/null 2>&1
    echo "  -> BBR enabled and applied."
fi

# 3. DNS settings check
echo "[*] Recommended: Ensure systemd-resolved is running for leak-proof DNS."
systemctl is-active --quiet systemd-resolved
if [ $? -eq 0 ]; then
    echo "  -> systemd-resolved is running."
else
    echo "  -> systemd-resolved is NOT running. TUN mode will use fallback DNS."
fi

# 4. Generate Polkit Policy for passwordless core execution
echo "[*] Generating Polkit Policy for passwordless execution..."
POLICY_FILE="/usr/share/polkit-1/actions/dev.nexvpn.policy"

REAL_SINGBOX=""
REAL_OPENVPN=""
REAL_AETHER=""

for dir in "${POSSIBLE_DIRS[@]}"; do
    if [ -f "$dir/sing-box" ]; then
        REAL_SINGBOX=$(realpath "$dir/sing-box")
    fi
done

if [ -n "$REAL_SINGBOX" ]; then
    REAL_AETHER=$(dirname "$REAL_SINGBOX")/aether
fi

REAL_OPENVPN=$(which openvpn 2>/dev/null)
REAL_IPTABLES=$(which iptables 2>/dev/null)

cat <<EOF > "$POLICY_FILE"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE policyconfig PUBLIC
 "-//freedesktop//DTD PolicyKit Policy Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/PolicyKit/1/policyconfig.dtd">
<policyconfig>
  <vendor>NexVPN</vendor>
  <vendor_url>https://nexvpn.dev/</vendor_url>
EOF

if [ -n "$REAL_SINGBOX" ]; then
  cat <<EOF >> "$POLICY_FILE"
  <action id="dev.nexvpn.run-singbox">
    <description>Run NexVPN Sing-box core</description>
    <message>Authentication is required to run the NexVPN core</message>
    <defaults>
      <allow_any>yes</allow_any>
      <allow_inactive>yes</allow_inactive>
      <allow_active>yes</allow_active>
    </defaults>
    <annotate key="org.freedesktop.policykit.exec.path">$REAL_SINGBOX</annotate>
    <annotate key="org.freedesktop.policykit.exec.allow_root_user">true</annotate>
  </action>
EOF
fi

if [ -n "$REAL_AETHER" ]; then
  cat <<EOF >> "$POLICY_FILE"
  <action id="dev.nexvpn.run-aether">
    <description>Run NexVPN Aether core</description>
    <message>Authentication is required to run the NexVPN Aether core</message>
    <defaults>
      <allow_any>yes</allow_any>
      <allow_inactive>yes</allow_inactive>
      <allow_active>yes</allow_active>
    </defaults>
    <annotate key="org.freedesktop.policykit.exec.path">$REAL_AETHER</annotate>
    <annotate key="org.freedesktop.policykit.exec.allow_root_user">true</annotate>
  </action>
EOF
fi

if [ -n "$REAL_OPENVPN" ]; then
  cat <<EOF >> "$POLICY_FILE"
  <action id="dev.nexvpn.run-openvpn">
    <description>Run NexVPN OpenVPN core</description>
    <message>Authentication is required to run the NexVPN OpenVPN core</message>
    <defaults>
      <allow_any>yes</allow_any>
      <allow_inactive>yes</allow_inactive>
      <allow_active>yes</allow_active>
    </defaults>
    <annotate key="org.freedesktop.policykit.exec.path">$REAL_OPENVPN</annotate>
    <annotate key="org.freedesktop.policykit.exec.allow_root_user">true</annotate>
  </action>
EOF
fi

if [ -n "$REAL_IPTABLES" ]; then
  cat <<EOF >> "$POLICY_FILE"
  <action id="dev.nexvpn.run-iptables">
    <description>Run NexVPN iptables helper</description>
    <message>Authentication is required to run iptables for VPN Kill Switch</message>
    <defaults>
      <allow_any>yes</allow_any>
      <allow_inactive>yes</allow_inactive>
      <allow_active>yes</allow_active>
    </defaults>
    <annotate key="org.freedesktop.policykit.exec.path">$REAL_IPTABLES</annotate>
    <annotate key="org.freedesktop.policykit.exec.allow_root_user">true</annotate>
  </action>
EOF
fi

echo "</policyconfig>" >> "$POLICY_FILE"
chmod 644 "$POLICY_FILE"
echo "  -> Polkit policy installed successfully at $POLICY_FILE."

echo "=========================================="
echo "Setup Complete! You can now run NexVPN."
echo "You will no longer be asked for a root password."
echo "=========================================="
