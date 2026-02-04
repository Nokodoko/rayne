#!/usr/bin/env bash
# cloudflare-dns-setup.sh
#
# Creates CNAME DNS records in Cloudflare that point to the rayne-webhooks
# tunnel (UUID: 2d837cb9-22e8-44c7-a8a4-2316157ec9c9).
#
# Each `cloudflared tunnel route dns` command creates a CNAME record
# for the given hostname pointing to <tunnel-uuid>.cfargotunnel.com.
# If the record already exists, cloudflared will report it and move on.
#
# Prerequisites:
#   - cloudflared is installed and authenticated
#   - The tunnel "rayne-webhooks" already exists
#   - n0kos.com nameservers are pointed at Cloudflare
#
# Usage:
#   ./scripts/cloudflare-dns-setup.sh

set -euo pipefail

TUNNEL_NAME="rayne-webhooks"

echo "==> Creating DNS routes for tunnel: ${TUNNEL_NAME}"

# Route the apex domain (n0kos.com) to the tunnel.
# This creates a CNAME record: n0kos.com -> <tunnel-uuid>.cfargotunnel.com
echo "  -> n0kos.com"
cloudflared tunnel route dns "${TUNNEL_NAME}" n0kos.com

# Route the www subdomain to the same tunnel.
# This creates a CNAME record: www.n0kos.com -> <tunnel-uuid>.cfargotunnel.com
echo "  -> www.n0kos.com"
cloudflared tunnel route dns "${TUNNEL_NAME}" www.n0kos.com

# The webhooks subdomain should already have a CNAME from initial setup,
# but run the command anyway to ensure it exists.
echo "  -> webhooks.n0kos.com"
cloudflared tunnel route dns "${TUNNEL_NAME}" webhooks.n0kos.com

# Route the gateway subdomain to the tunnel.
# Traffic reaches the gateway running on base (192.168.50.179:8001) via
# the ingress rule in k8s/cloudflare-tunnel.yaml.
echo "  -> gateway.n0kos.com"
cloudflared tunnel route dns "${TUNNEL_NAME}" gateway.n0kos.com

echo ""
echo "Done. Verify with: cloudflared tunnel route list"
