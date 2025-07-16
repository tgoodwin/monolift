# install prometheus / grafana
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts

helm upgrade --install prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  -f - <<EOF
prometheus:
  service:
    type: LoadBalancer
    # Optional: For MetalLB, you might specify a separate IP or pool for Prometheus
    # loadBalancerIP: 192.168.1.200 # Replace with an IP from your MetalLB pool
    # annotations:
    #   metallb.io/address-pool: "your-metallb-pool-name"

grafana:
  service:
    type: LoadBalancer
    # Optional: For MetalLB, you might specify a separate IP or pool for Grafana
    # loadBalancerIP: 192.168.1.201
    # annotations:
    #   metallb.io/address-pool: "your-other-metallb-pool"
EOF

# install k9s
curl -sS https://webinstall.dev/k9s | bash
source ~/.config/envman/PATH.env

# install dapr
echo "Installing Dapr CLI..."
wget -q https://raw.githubusercontent.com/dapr/cli/master/install/install.sh -O - | /bin/bash

echo "Initializing Dapr..."
sudo dapr init -k

echo "Dapr installed and initialized."
sudo dapr status -k


# reminders
echo "reminder to login to docker! docker login -u tlg2132"

