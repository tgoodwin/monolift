# install prometheus / grafana
helm install kube-prometheus-stack --create-namespace --namespace monitoring prometheus-community/kube-prometheus-stack

# install k9s
curl -sS https://webinstall.dev/k9s | bash
source ~/.config/envman/PATH.env


